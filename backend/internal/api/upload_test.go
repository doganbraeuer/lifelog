package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/service"
)

// fakeUploader remembers the upload and returns entry or err.
type fakeUploader struct {
	entry      core.Entry
	err        error
	upload     *service.Upload
	textUpload *service.TextUpload
}

func (u *fakeUploader) Upload(_ context.Context, up service.Upload) (core.Entry, error) {
	u.upload = &up
	return u.entry, u.err
}

func (u *fakeUploader) UploadText(_ context.Context, up service.TextUpload) (core.Entry, error) {
	u.textUpload = &up
	return u.entry, u.err
}

// form builds a multipart request. files: file name → content.
func form(t *testing.T, files map[string][]byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, content := range files {
		fw, err := mw.CreateFormFile("datei", name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(content)
	}
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/eintraege", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func upload(t *testing.T, u *fakeUploader, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Uploader: u})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func fullEntry() core.Entry {
	p := func(s string) *string { return &s }
	return core.Entry{
		ID: "6727a1f3c2b4e5d6f7a8b9c0", Module: "gesundheit", Type: "laborbefund",
		DocumentDate: p("2026-10-14"),
		ImportedAt:   time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer:       p("Labor Muster GmbH, Salzburg"),
		FullText:     "Hämoglobin 14,2 g/dl",
		Facts: []core.Fact{
			{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: p("g/dl")},
			{ID: "i2", Name: "Befundart", Value: "Blutbild klein"},
		},
		File: core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund.pdf",
			MimeType: "application/pdf", Size: 184320, Sha256: strings.Repeat("ab", 32)},
		Corrections: nil, // must still come out as []
	}
}

func TestUpload_Success(t *testing.T) {
	u := &fakeUploader{entry: fullEntry()}
	rec := upload(t, u, form(t,
		map[string][]byte{"befund.pdf": []byte("%PDF-1.7")},
		map[string]string{"modul": "gesundheit"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0" {
		t.Errorf("Location = %q", loc)
	}
	if u.upload == nil || string(u.upload.Data) != "%PDF-1.7" || u.upload.FileName != "befund.pdf" || u.upload.Module != "gesundheit" {
		t.Errorf("upload to the service = %+v", u.upload)
	}

	body := rec.Body.String()
	for _, must := range []string{
		`"korrekturen":[]`,
		`"importdatum":"2026-11-03T18:42:07Z"`,
		`"mimeTyp":"application/pdf"`,
		`{"id":"i2","name":"Befundart","wert":"Blutbild klein"}`, // without einheit
		`"volltext":"Hämoglobin 14,2 g/dl"`,
	} {
		if !strings.Contains(body, must) {
			t.Errorf("response does not contain %s:\n%s", must, body)
		}
	}
}

func TestUpload_WithoutModuleIsEmpty(t *testing.T) {
	u := &fakeUploader{entry: fullEntry()}
	upload(t, u, form(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, nil))
	if u.upload == nil || u.upload.Module != "" {
		t.Errorf("module = %+v (empty means default, the service decides)", u.upload)
	}
}

func TestUpload_FormErrors(t *testing.T) {
	pdf := []byte("%PDF-")
	cases := []struct {
		name    string
		request func(t *testing.T) *http.Request
		status  int
		code    string
	}{
		{"not multipart", func(t *testing.T) *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/eintraege", strings.NewReader(`{"datei":"x"}`))
			r.Header.Set("Content-Type", "application/json")
			return r
		}, 400, CodeInvalidRequest},
		{"without file", func(t *testing.T) *http.Request {
			return form(t, nil, map[string]string{"modul": "gesundheit"})
		}, 400, CodeFileMissing},
		{"file and text", func(t *testing.T) *http.Request {
			return form(t, map[string][]byte{"a.pdf": pdf}, map[string]string{"text": "Befund"})
		}, 400, CodeInvalidRequest},
		{"two files", func(t *testing.T) *http.Request {
			return form(t, map[string][]byte{"a.pdf": pdf, "b.pdf": pdf}, nil)
		}, 400, CodeInvalidRequest},
		{"request much too large", func(t *testing.T) *http.Request {
			return form(t, map[string][]byte{"large.pdf": make([]byte, core.MaxFileBytes+2*formOverhead)}, nil)
		}, 413, CodeFileTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := &fakeUploader{}
			rec := upload(t, u, c.request(t))
			checkError(t, rec, c.status, c.code)
			if u.upload != nil || u.textUpload != nil {
				t.Error("the service must not be called on a form error")
			}
		})
	}
}

func TestUpload_EmptyTextDoesNotCount(t *testing.T) {
	// "Empty or only whitespace counts as missing" (API.md 3.1): file + empty text is ok.
	u := &fakeUploader{entry: fullEntry()}
	rec := upload(t, u, form(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, map[string]string{"text": "  "}))
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, body %s", rec.Code, rec.Body)
	}
	if u.upload == nil || u.textUpload != nil {
		t.Error("must run as a file upload")
	}
}

func TestUpload_ServiceErrors(t *testing.T) {
	cases := []struct {
		err     error
		status  int
		code    string
		details map[string]any
	}{
		{core.ErrUnknownModule, 400, CodeInvalidRequest, map[string]any{"feld": "modul", "wert": "sport"}},
		{core.ErrFileEmpty, 400, CodeFileMissing, nil},
		{core.ErrFileTooLarge, 413, CodeFileTooLarge, nil},
		{core.ErrUnsupportedFileType, 415, CodeUnsupportedFileType, nil},
		{&service.DuplicateError{EntryID: "6727a1f3c2b4e5d6f7a8b9c0"}, 409, CodeDuplicate,
			map[string]any{"eintragId": "6727a1f3c2b4e5d6f7a8b9c0"}},
		{core.ErrUnreadable, 422, CodeUnreadable, nil},
		{&core.ExtractionError{Field: "typ", Reason: "x"}, 502, CodeExtractionFailed, nil},
		{fmt.Errorf("broken JSON: %w", llm.ErrInvalidResponse), 502, CodeExtractionFailed, nil},
		{fmt.Errorf("Gemini HTTP 503: %w", llm.ErrUnavailable), 503, CodeModelUnavailable, nil},
		{errors.New("database gone"), 500, CodeInternalError, nil},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			u := &fakeUploader{err: c.err}
			rec := upload(t, u, form(t,
				map[string][]byte{"a.pdf": []byte("%PDF-")}, map[string]string{"modul": "sport"}))

			a := checkError(t, rec, c.status, c.code)
			if c.details != nil && fmt.Sprint(a.Error.Details) != fmt.Sprint(c.details) {
				t.Errorf("details = %v, want %v", a.Error.Details, c.details)
			}
			// Internal causes (e.g. Gemini messages) do not belong in the response.
			for _, internal := range []string{"database gone", "Gemini", "broken JSON"} {
				if strings.Contains(rec.Body.String(), internal) {
					t.Errorf("internal cause %q in the response: %s", internal, rec.Body)
				}
			}
		})
	}
}

func TestUpload_Canceled(t *testing.T) {
	u := &fakeUploader{err: context.Canceled}
	rec := upload(t, u, form(t, map[string][]byte{"a.pdf": []byte("%PDF-")}, nil))
	// The app is gone; nothing useful is written anymore.
	if rec.Body.Len() != 0 {
		t.Errorf("body after cancel: %s", rec.Body)
	}
}

func checkError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) errorResponse {
	t.Helper()
	if rec.Code != status {
		t.Errorf("status = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	var a errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("no error JSON: %v (%s)", err, rec.Body)
	}
	if a.Error.Code != code || a.Error.Message == "" {
		t.Errorf("error = %+v, want code %s", a.Error, code)
	}
	return a
}
