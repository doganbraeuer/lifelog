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

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
	speechfake "lifelog/backend/internal/speech/fake"
)

// m4a is the start of an M4A file (enough for core.CheckAudio).
var m4a = append([]byte("\x00\x00\x00\x1cftypM4A \x00\x00\x00\x00isommp42"), make([]byte, 100)...)

// audioForm builds a multipart request with one "audio" part per file.
func audioForm(t *testing.T, field string, files ...[]byte) *http.Request {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for i, f := range files {
		fw, err := mw.CreateFormFile(field, fmt.Sprintf("notiz%d.m4a", i))
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(f)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transcriptions", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func serve(t Transcriber, req *http.Request) *httptest.ResponseRecorder {
	deps := Dependencies{DB: fakePinger{}}
	if t != nil {
		deps.Transcriber = t
	}
	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, req)
	return rec
}

func TestTranscribe_OK(t *testing.T) {
	f := &speechfake.Transcriber{Result: core.Transcript{
		Text:            "Arztbrief vom 12. November 2026. Ibuprofen 400 mg dreimal täglich.",
		DurationSeconds: 21.4, Language: "de", Engine: "whisper.cpp large-v3-turbo",
	}}
	rec := serve(f, audioForm(t, "audio", m4a))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	checkJSONHeader(t, rec)
	want := `{"text":"Arztbrief vom 12. November 2026. Ibuprofen 400 mg dreimal täglich.","durationSeconds":21.4,"language":"de","engine":"whisper.cpp large-v3-turbo"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body\n%s\nwant\n%s", got, want)
	}
	if mime, size := f.Last(); mime != core.MimeAudioMP4 || size != len(m4a) {
		t.Errorf("transcriber got %q, %d bytes", mime, size)
	}
}

func TestTranscribe_Errors(t *testing.T) {
	big := append(append([]byte{}, m4a...), make([]byte, core.MaxFileBytes)...)
	tests := []struct {
		name    string
		req     func(t *testing.T) *http.Request
		err     error // returned by the transcriber
		status  int
		code    string
		details string // expected JSON of details, "" = none
		called  bool   // transcriber must be called
	}{
		{"no audio field", func(t *testing.T) *http.Request { return audioForm(t, "file", m4a) }, nil, 400, CodeFileMissing, "", false},
		{"empty recording", func(t *testing.T) *http.Request { return audioForm(t, "audio", []byte{}) }, nil, 400, CodeFileMissing, "", false},
		{"two recordings", func(t *testing.T) *http.Request { return audioForm(t, "audio", m4a, m4a) }, nil, 400, CodeInvalidRequest, `{"field":"audio"}`, false},
		{"not multipart", func(t *testing.T) *http.Request {
			return httptest.NewRequest(http.MethodPost, "/api/v1/transcriptions", strings.NewReader("{}"))
		}, nil, 400, CodeInvalidRequest, "", false},
		{"over 10 MB", func(t *testing.T) *http.Request { return audioForm(t, "audio", big) }, nil, 413, CodeFileTooLarge, "", false},
		{"far over 10 MB", func(t *testing.T) *http.Request {
			return audioForm(t, "audio", append(big, make([]byte, 2<<20)...))
		}, nil, 413, CodeFileTooLarge, "", false},
		{"a photo is no recording", func(t *testing.T) *http.Request {
			return audioForm(t, "audio", []byte("\xff\xd8\xff\xe0 jpeg"))
		}, nil, 415, CodeUnsupportedFileType, "", false},
		{"too long", nil, fmt.Errorf("more than 300 s: %w", speech.ErrTooLong), 413, CodeAudioTooLong, `{"maxSeconds":300}`, true},
		{"damaged", nil, fmt.Errorf("ffmpeg: %w", speech.ErrUnreadable), 422, CodeUnreadable, "", true},
		{"silence", nil, speech.ErrNoSpeech, 422, CodeNoSpeech, "", true},
		{"whisper down", nil, fmt.Errorf("connection refused: %w", speech.ErrUnavailable), 503, CodeModelUnavailable, "", true},
		{"time limit", nil, context.DeadlineExceeded, 503, CodeModelUnavailable, "", true},
		{"unexpected", nil, errors.New("secret internal detail"), 500, CodeInternalError, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := audioForm(t, "audio", m4a)
			if tt.req != nil {
				req = tt.req(t)
			}
			f := &speechfake.Transcriber{Err: tt.err, Result: core.Transcript{Text: "x"}}
			rec := serve(f, req)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tt.status, rec.Body)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != tt.code || body.Error.Message == "" {
				t.Errorf("error %+v, want code %s", body.Error, tt.code)
			}
			details := ""
			if body.Error.Details != nil {
				b, _ := json.Marshal(body.Error.Details)
				details = string(b)
			}
			if details != tt.details {
				t.Errorf("details %s, want %s", details, tt.details)
			}
			if (f.Calls() > 0) != tt.called {
				t.Errorf("transcriber called %d times", f.Calls())
			}
			if strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "refused") {
				t.Errorf("internal cause in the response: %s", rec.Body)
			}
		})
	}
}

func TestTranscribe_Off(t *testing.T) {
	rec := serve(nil, audioForm(t, "audio", m4a))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), CodeModelUnavailable) {
		t.Errorf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestStatus_Speech(t *testing.T) {
	for _, tt := range []struct {
		name string
		t    Transcriber
		want string
	}{
		{"running", &speechfake.Transcriber{}, "ok"},
		{"whisper down", &speechfake.Transcriber{PingErr: speech.ErrUnavailable}, "unavailable"},
		{"switched off", nil, "off"},
	} {
		rec := serve(tt.t, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d (speech must not change it)", tt.name, rec.Code)
		}
		want := `{"status":"ok","database":"ok","speech":"` + tt.want + `"}`
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Errorf("%s: %s, want %s", tt.name, got, want)
		}
	}
}
