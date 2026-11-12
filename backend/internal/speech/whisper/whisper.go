// Package whisper implements speech.Engine with whisper.cpp's own HTTP
// server (`whisper-server`). It runs as a separate local process, so the
// model (1.6 GB) is loaded once and not for every recording. The backend
// only talks to it on 127.0.0.1; the phone never does.
//
// Start it with backend/scripts/whisper-server.sh (or .ps1): German,
// voice activity detection, no temperature fallback, the prompt kept for
// every window (backend/README.md, speech). Every request carries the
// Vocabulary as prompt.
package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

// DefaultURL: our backend uses 8080, whisper-server gets 8178.
const DefaultURL = "http://127.0.0.1:8178"

// DefaultModel is the model the start script loads. Only for Name.
const DefaultModel = "large-v3-turbo"

// pingTimeout limits how long GET /status waits for the server.
const pingTimeout = 2 * time.Second

// maxResponseBytes: a 5-minute recording gives a few kilobytes of JSON.
const maxResponseBytes = 4 << 20

// Whisper implements speech.Engine.
type Whisper struct {
	// Prompt is sent with every recording; New sets Vocabulary, "" sends
	// none (cmd/speechcheck -vocabulary=false).
	Prompt string

	url    string
	model  string
	client *http.Client
}

// New talks to the whisper-server at baseURL (DefaultURL if empty). The
// time limit comes from the caller's context.
func New(baseURL string) *Whisper {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	return &Whisper{Prompt: Vocabulary, url: strings.TrimRight(baseURL, "/"), model: DefaultModel, client: &http.Client{}}
}

// Name is e.g. "whisper.cpp large-v3-turbo".
func (w *Whisper) Name() string { return "whisper.cpp " + w.model }

// Recognize sends the WAV file to POST /inference. The file is always
// called "audio.wav": whisper-server writes the file name to its log, and
// the name from the phone could tell something about the content.
func (w *Whisper) Recognize(ctx context.Context, wav []byte) ([]core.Segment, error) {
	body, contentType, err := form(wav, w.Prompt)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url+"/inference", body)
	if err != nil {
		return nil, fmt.Errorf("whisper request: %v: %w", err, speech.ErrUnavailable)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := w.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("whisper-server not reachable: %v: %w", err, speech.ErrUnavailable)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("whisper response: %v: %w", err, speech.ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		// The body is not logged: it could repeat parts of the request.
		return nil, fmt.Errorf("whisper-server answered %d: %w", resp.StatusCode, speech.ErrUnavailable)
	}
	return parseResponse(data)
}

// form builds the multipart request: language fixed to German,
// temperature 0 without fallback, segments with times, and the prompt
// unless it is empty.
//
// no_language_probabilities: verbose_json otherwise runs a language
// detection on every request, although the language is fixed. On a freshly
// started whisper-server 1.9.5 it crashes the server when the voice
// activity detection finds no speech (whisper.cpp issue #4156, fixed in
// #4158 after 1.9.5). Without it the server answers {"text":"",
// "segments":[]} and is faster.
func form(wav []byte, prompt string) (io.Reader, string, error) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err == nil {
		_, err = fw.Write(wav)
	}
	fields := [][2]string{
		{"language", core.TranscriptLanguage},
		{"temperature", "0.0"},
		{"temperature_inc", "0.0"},
		{"response_format", "verbose_json"},
		{"no_language_probabilities", "true"},
	}
	if prompt != "" {
		fields = append(fields, [2]string{"prompt", prompt})
	}
	for _, f := range fields {
		if err == nil {
			err = mw.WriteField(f[0], f[1])
		}
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		return nil, "", fmt.Errorf("whisper form: %w", err)
	}
	return &b, mw.FormDataContentType(), nil
}

// response is the part of verbose_json we use.
type response struct {
	Text     *string `json:"text"`
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
	Error string `json:"error"`
}

func parseResponse(data []byte) ([]core.Segment, error) {
	var r response
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("whisper response is no JSON: %v: %w", err, speech.ErrUnavailable)
	}
	if r.Error != "" || r.Text == nil {
		return nil, fmt.Errorf("whisper-server reported an error: %w", speech.ErrUnavailable)
	}
	segments := make([]core.Segment, 0, len(r.Segments))
	for _, s := range r.Segments {
		segments = append(segments, core.Segment{Start: s.Start, End: s.End, Text: s.Text})
	}
	// Without segments (e.g. another response format) the whole text is
	// one segment.
	if len(segments) == 0 && strings.TrimSpace(*r.Text) != "" {
		segments = append(segments, core.Segment{Text: *r.Text})
	}
	return segments, nil
}

// Ping implements speech.Engine: GET / answers 200 when the server is up.
func (w *Whisper) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url+"/", nil)
	if err != nil {
		return fmt.Errorf("%v: %w", err, speech.ErrUnavailable)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("whisper-server not reachable: %v: %w", err, speech.ErrUnavailable)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("whisper-server answered %d: %w", resp.StatusCode, speech.ErrUnavailable)
	}
	return nil
}
