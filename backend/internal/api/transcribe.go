package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

// Error codes of the transcription (API.md 3.9) besides the shared ones.
const (
	CodeAudioTooLong = "AUDIO_TOO_LONG"
	CodeNoSpeech     = "NO_SPEECH"
)

// TranscribeTimeout limits a whole transcription (API.md 3.9: up to about
// 120 seconds; the app waits at least 130).
const TranscribeTimeout = 120 * time.Second

// Transcriber turns a recording into text. Implemented by speech.Pipeline;
// nil in Dependencies means: switched off.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mimeType string) (core.Transcript, error)
	Ping(ctx context.Context) error
}

type transcriptJSON struct {
	Text            string  `json:"text"`
	DurationSeconds float64 `json:"durationSeconds"`
	Language        string  `json:"language"`
	Engine          string  `json:"engine"`
}

const messageSpeechUnavailable = "Die Spracherkennung ist gerade nicht erreichbar. Bitte später noch einmal versuchen."

// transcribeHandler answers POST /api/v1/transcriptions (API.md 3.9).
// Nothing is stored: the recording stays in memory (the form is parsed
// with maxMemory as large as the request, so Go writes no temporary
// file), and only the text goes back. The log gets size, length and
// result, never the text.
func transcribeHandler(t Transcriber) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, core.MaxFileBytes+formOverhead)
		if err := r.ParseMultipartForm(core.MaxFileBytes + formOverhead); err != nil {
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "Die Aufnahme ist größer als 10 MB.", nil)
			case errors.Is(err, http.ErrNotMultipart):
				writeError(w, http.StatusBadRequest, CodeInvalidRequest,
					"Bitte die Aufnahme als multipart/form-data im Feld audio schicken.", nil)
			default:
				writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Das Formular ist nicht lesbar.", nil)
			}
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		files := r.MultipartForm.File["audio"]
		switch {
		case len(files) == 0:
			writeError(w, http.StatusBadRequest, CodeFileMissing, "Bitte eine Aufnahme mitschicken.", nil)
			return
		case len(files) > 1:
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Bitte genau eine Aufnahme mitschicken.",
				map[string]any{"field": "audio"})
			return
		}
		audio, err := readFile(files[0])
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		mime, err := core.CheckAudio(audio)
		if err != nil {
			writeTranscribeError(w, r, err)
			return
		}
		if t == nil {
			slog.Info("transcription", "result", "off")
			writeError(w, http.StatusServiceUnavailable, CodeModelUnavailable, messageSpeechUnavailable, nil)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), TranscribeTimeout)
		defer cancel()
		start := time.Now()
		tr, err := t.Transcribe(ctx, audio, mime)
		if err != nil {
			slog.Info("transcription", "bytes", len(audio), "type", mime, "took", time.Since(start).Round(time.Millisecond), "error", err)
			writeTranscribeError(w, r, err)
			return
		}
		slog.Info("transcription", "bytes", len(audio), "type", mime, "seconds", tr.DurationSeconds,
			"chars", len([]rune(tr.Text)), "took", time.Since(start).Round(time.Millisecond))
		writeJSON(w, http.StatusOK, transcriptJSON{
			Text:            tr.Text,
			DurationSeconds: tr.DurationSeconds,
			Language:        tr.Language,
			Engine:          tr.Engine,
		})
	}
}

// writeTranscribeError maps the errors to the codes of API.md 3.9.
func writeTranscribeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, core.ErrFileEmpty):
		writeError(w, http.StatusBadRequest, CodeFileMissing, "Bitte eine Aufnahme mitschicken.", nil)
	case errors.Is(err, core.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "Die Aufnahme ist größer als 10 MB.", nil)
	case errors.Is(err, core.ErrUnsupportedAudioType):
		writeError(w, http.StatusUnsupportedMediaType, CodeUnsupportedFileType,
			"Dieses Audioformat wird nicht unterstützt. Erlaubt sind M4A, 3GP, AMR, WAV, OGG, WebM, MP3 und FLAC.", nil)
	case errors.Is(err, speech.ErrTooLong):
		writeError(w, http.StatusRequestEntityTooLarge, CodeAudioTooLong,
			"Die Aufnahme ist länger als 5 Minuten. Bitte kürzer sprechen oder aufteilen.",
			map[string]any{"maxSeconds": core.MaxAudioSeconds})
	case errors.Is(err, speech.ErrUnreadable):
		writeError(w, http.StatusUnprocessableEntity, CodeUnreadable, "Die Aufnahme lässt sich nicht abspielen.", nil)
	case errors.Is(err, speech.ErrNoSpeech):
		writeError(w, http.StatusUnprocessableEntity, CodeNoSpeech, "In der Aufnahme ist keine Sprache zu hören.", nil)
	case errors.Is(err, speech.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusServiceUnavailable, CodeModelUnavailable, messageSpeechUnavailable, nil)
	case errors.Is(err, context.Canceled):
		// The app dropped the connection; nobody reads the response anymore.
	default:
		writeInternalError(w, r, err)
	}
}
