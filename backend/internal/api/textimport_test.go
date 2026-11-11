package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/service"
)

// textForm builds a form from fields in a fixed order; a name may appear
// several times. A field "file" is attached as a file.
func textForm(t *testing.T, fields ...[2]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range fields {
		if f[0] == "file" {
			fw, err := mw.CreateFormFile("file", "a.pdf")
			if err != nil {
				t.Fatal(err)
			}
			fw.Write([]byte(f[1]))
			continue
		}
		mw.WriteField(f[0], f[1])
	}
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/entries", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func textEntry() core.Entry {
	e := fullEntry()
	e.FullText = "  Hausarztpraxis Beispiel\r\nÜberweisung\n"
	e.File = core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "text.txt",
		MimeType: "text/plain; charset=utf-8", Size: 42, Sha256: strings.Repeat("cd", 32)}
	return e
}

func TestTextImport_Success(t *testing.T) {
	text := "  Hausarztpraxis Beispiel\r\nÜberweisung\n"
	u := &fakeUploader{entry: textEntry()}
	rec := upload(t, u, textForm(t, [2]string{"text", text}, [2]string{"module", "gesundheit"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Location") != "/api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0" {
		t.Errorf("Location = %q", rec.Header().Get("Location"))
	}
	// Unchanged to the service, including whitespace and \r\n.
	if u.textUpload == nil || u.textUpload.Text != text || u.textUpload.Module != "gesundheit" || u.upload != nil {
		t.Errorf("to the service: text %+v, file %+v", u.textUpload, u.upload)
	}
	for _, must := range []string{`"mimeType":"text/plain; charset=utf-8"`, `"fileName":"text.txt"`, `"corrections":[]`} {
		if !strings.Contains(rec.Body.String(), must) {
			t.Errorf("response without %s: %s", must, rec.Body)
		}
	}
}

// Errors the form already shows; the service is not called.
func TestTextImport_FormErrors(t *testing.T) {
	for _, c := range []struct {
		name    string
		fields  [][2]string
		status  int
		code    string
		message string
	}{
		{"neither file nor text", [][2]string{{"module", "gesundheit"}}, 400, CodeFileMissing, "Datei"},
		{"text empty", [][2]string{{"text", ""}}, 400, CodeFileMissing, "Text"},
		{"text only whitespace", [][2]string{{"text", " \r\n\t "}}, 400, CodeFileMissing, "Text"},
		{"file and text", [][2]string{{"file", "%PDF-"}, {"text", "Befund"}}, 400, CodeInvalidRequest, "nicht beides"},
		{"two texts", [][2]string{{"text", "a"}, {"text", "b"}}, 400, CodeInvalidRequest, "genau einen Text"},
		{"two texts, one empty", [][2]string{{"text", ""}, {"text", "b"}}, 400, CodeInvalidRequest, "genau einen Text"},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := &fakeUploader{}
			rec := upload(t, u, textForm(t, c.fields...))
			a := checkError(t, rec, c.status, c.code)
			if !strings.Contains(a.Error.Message, c.message) {
				t.Errorf("message %q does not contain %q", a.Error.Message, c.message)
			}
			if u.upload != nil || u.textUpload != nil {
				t.Error("service called")
			}
		})
	}
}

// The checks from core.CheckText, through the real service: this also shows
// that characters count, not bytes.
func TestTextImport_CheckThroughTheService(t *testing.T) {
	for _, c := range []struct {
		name    string
		fields  [][2]string
		status  int
		code    string
		details string
	}{
		{"100,001 characters", [][2]string{{"text", strings.Repeat("a", core.MaxTextChars+1)}},
			413, CodeFileTooLarge, "map[maxChars:100000]"},
		{"100,001 umlauts", [][2]string{{"text", strings.Repeat("ü", core.MaxTextChars+1)}},
			413, CodeFileTooLarge, "map[maxChars:100000]"},
		{"invalid UTF-8", [][2]string{{"text", "Befund \xff\xfe"}},
			400, CodeInvalidRequest, "map[field:text]"},
		{"unknown module", [][2]string{{"text", "Befund"}, {"module", "sport"}},
			400, CodeInvalidRequest, "map[field:module value:sport]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Service without store and models: the check comes before anything else.
			u := service.NewUploader(nil, nil, nil, nil)
			router := NewRouter(Dependencies{DB: fakePinger{}, Uploader: u})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, textForm(t, c.fields...))

			a := checkError(t, rec, c.status, c.code)
			if fmt.Sprint(a.Error.Details) != c.details {
				t.Errorf("details = %v, want %s", a.Error.Details, c.details)
			}
		})
	}
}

// 100,000 umlauts are 200,000 bytes, but allowed: the handler lets them
// through.
func TestTextImport_LimitAllowed(t *testing.T) {
	u := &fakeUploader{entry: textEntry()}
	rec := upload(t, u, textForm(t, [2]string{"text", strings.Repeat("ü", core.MaxTextChars)}))
	if rec.Code != http.StatusCreated || u.textUpload == nil {
		t.Errorf("status %d", rec.Code)
	}
}

func TestTextImport_ServiceErrors(t *testing.T) {
	for _, c := range []struct {
		err     error
		status  int
		code    string
		message string
		details string
	}{
		{core.ErrFileEmpty, 400, CodeFileMissing, "Bitte einen Text eingeben.", "map[]"},
		{&service.DuplicateError{EntryID: "6727a1f3c2b4e5d6f7a8b9c0"}, 409, CodeDuplicate,
			"Diesen Text hast du schon importiert.", "map[entryId:6727a1f3c2b4e5d6f7a8b9c0]"},
		{&core.ExtractionError{Field: "type", Reason: "x"}, 502, CodeExtractionFailed, "", "map[]"},
		{fmt.Errorf("broken JSON: %w", llm.ErrInvalidResponse), 502, CodeExtractionFailed, "", "map[]"},
		{fmt.Errorf("Gemini 503: %w", llm.ErrUnavailable), 503, CodeModelUnavailable, "", "map[]"},
		{fmt.Errorf("database gone"), 500, CodeInternalError, "", "map[]"},
	} {
		t.Run(c.code, func(t *testing.T) {
			u := &fakeUploader{err: c.err}
			rec := upload(t, u, textForm(t, [2]string{"text", "Befund"}))
			a := checkError(t, rec, c.status, c.code)
			if c.message != "" && a.Error.Message != c.message {
				t.Errorf("message %q", a.Error.Message)
			}
			if fmt.Sprint(a.Error.Details) != c.details && !(c.details == "map[]" && a.Error.Details == nil) {
				t.Errorf("details = %v, want %s", a.Error.Details, c.details)
			}
			for _, internal := range []string{"database gone", "Gemini", "broken JSON", "Befund"} {
				if strings.Contains(rec.Body.String(), internal) {
					t.Errorf("%q in the response: %s", internal, rec.Body)
				}
			}
		})
	}
}
