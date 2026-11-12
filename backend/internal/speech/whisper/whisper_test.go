package whisper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

// verboseJSON is a shortened real answer of whisper-server 1.9.5.
const verboseJSON = `{"task":"transcribe","language":"german","duration":9.7,
"text":" Arztbrief vom 12. November 2026.\n Ibuprofen 400 mg dreimal täglich.\n",
"segments":[
 {"id":0,"text":" Arztbrief vom 12. November 2026.","start":0.0,"end":3.56,"temperature":0.0,"avg_logprob":-0.04,"no_speech_prob":0.00002,"tokens":[1,2],"words":[]},
 {"id":1,"text":" Ibuprofen 400 mg dreimal täglich.","start":6.62,"end":9.59,"temperature":0.0,"avg_logprob":-0.03,"no_speech_prob":0.00002,"tokens":[3],"words":[]}]}`

func TestRecognize(t *testing.T) {
	var fields map[string]string
	var fileName string
	var fileSize int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/inference" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		fields = map[string]string{}
		for k, v := range r.MultipartForm.Value {
			fields[k] = v[0]
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(f)
		fileName, fileSize = h.Filename, len(b)
		io.WriteString(w, verboseJSON)
	}))
	defer srv.Close()

	wav := core.WAV(make([]int16, 1600))
	segs, err := New(srv.URL+"/").Recognize(context.Background(), wav)
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Segment{
		{Start: 0, End: 3.56, Text: " Arztbrief vom 12. November 2026."},
		{Start: 6.62, End: 9.59, Text: " Ibuprofen 400 mg dreimal täglich."},
	}
	if len(segs) != 2 || segs[0] != want[0] || segs[1] != want[1] {
		t.Errorf("segments %+v", segs)
	}
	wantFields := map[string]string{"language": "de", "temperature": "0.0", "temperature_inc": "0.0", "response_format": "verbose_json"}
	for k, v := range wantFields {
		if fields[k] != v {
			t.Errorf("field %s = %q, want %q", k, fields[k], v)
		}
	}
	if fileName != "audio.wav" || fileSize != len(wav) {
		t.Errorf("file %q (%d bytes), want audio.wav (%d)", fileName, fileSize, len(wav))
	}
}

func TestParseResponse(t *testing.T) {
	segs, err := parseResponse([]byte(`{"text":""}`))
	if err != nil || len(segs) != 0 {
		t.Errorf("empty text: %v, %v", segs, err)
	}
	segs, err = parseResponse([]byte(`{"text":" Puls 64."}`))
	if err != nil || len(segs) != 1 || segs[0].Text != " Puls 64." {
		t.Errorf("text without segments: %v, %v", segs, err)
	}
	for name, body := range map[string]string{
		"no JSON":        "<html>",
		"error field":    `{"error":"failed to read audio"}`,
		"text missing":   `{"segments":[]}`,
		"truncated JSON": `{"text":" Puls`,
	} {
		if _, err := parseResponse([]byte(body)); !errors.Is(err, speech.ErrUnavailable) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRecognize_ServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"busy"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Recognize(context.Background(), []byte("RIFF")); !errors.Is(err, speech.ErrUnavailable) {
		t.Errorf("500: %v", err)
	}

	// Nothing listens on the closed server's address.
	closed := httptest.NewServer(http.NotFoundHandler())
	url := closed.URL
	closed.Close()
	w := New(url)
	if _, err := w.Recognize(context.Background(), []byte("RIFF")); !errors.Is(err, speech.ErrUnavailable) {
		t.Errorf("refused: %v", err)
	}
	if err := w.Ping(context.Background()); !errors.Is(err, speech.ErrUnavailable) {
		t.Errorf("ping refused: %v", err)
	}
}

func TestRecognize_TimeLimit(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := New(srv.URL).Recognize(ctx, []byte("RIFF")); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error %v, want deadline exceeded", err)
	}
}

func TestPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	if err := New(srv.URL).Ping(context.Background()); err != nil {
		t.Errorf("ping: %v", err)
	}
	if New("").url != DefaultURL || New("").Name() != "whisper.cpp large-v3-turbo" {
		t.Error("defaults")
	}
}

// Against the real whisper-server (backend/scripts/whisper-server.sh):
//
//	LIFELOG_LIVE=1 go test ./internal/speech/whisper -run Live -v
func TestLive_Silence(t *testing.T) {
	if os.Getenv("LIFELOG_LIVE") != "1" {
		t.Skip("live test skipped (set LIFELOG_LIVE=1)")
	}
	w := New(os.Getenv("WHISPER_URL"))
	if err := w.Ping(context.Background()); err != nil {
		t.Skipf("whisper-server not running: %v", err)
	}
	// 5 s of silence must give no text at all (voice activity detection).
	segs, err := w.Recognize(context.Background(), core.WAV(make([]int16, 5*core.SpeechSampleRate)))
	if err != nil {
		t.Fatal(err)
	}
	if text := core.CleanTranscript(segs); strings.TrimSpace(text) != "" {
		t.Errorf("silence gave %q", text)
	}
}
