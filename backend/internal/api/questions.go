package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Error codes for questions (API.md 3.6).
const (
	CodeQuestionMissing = "FRAGE_FEHLT"
	CodeQuestionTooLong = "FRAGE_ZU_LANG"
)

// maxQuestionBody: a question has at most 500 characters (≤ 2000 bytes);
// 64 KiB leave plenty of room and still limit what the server reads.
const maxQuestionBody = 64 << 10

// Asker answers questions. Implemented by service.Asker.
type Asker interface {
	Ask(ctx context.Context, question, module string) (core.Answer, error)
}

type questionRequest struct {
	Question string `json:"frage"`
	Module   string `json:"modul"`
}

type answerJSON struct {
	Answer  string       `json:"antwort"`
	Found   bool         `json:"gefunden"`
	Sources []sourceJSON `json:"quellen"`
}

type sourceJSON struct {
	Number       int     `json:"nr"`
	EntryID      string  `json:"eintragId"`
	Module       string  `json:"modul"`
	Type         string  `json:"typ"`
	Issuer       *string `json:"quelle"`
	DocumentDate *string `json:"dokumentdatum"`
	Excerpt      string  `json:"auszug"`
}

func answerToJSON(a core.Answer) answerJSON {
	sources := make([]sourceJSON, 0, len(a.Sources)) // never nil
	for _, s := range a.Sources {
		sources = append(sources, sourceJSON{
			Number: s.Number, EntryID: s.EntryID, Module: s.Module, Type: s.Type,
			Issuer: s.Issuer, DocumentDate: s.DocumentDate, Excerpt: s.Excerpt,
		})
	}
	return answerJSON{Answer: a.Text, Found: a.Found, Sources: sources}
}

// questionsHandler answers POST /api/v1/fragen (API.md 3.6).
func questionsHandler(a Asker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req questionRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxQuestionBody))
		// Unknown fields are an error (API.md 1): a typo like "frge" should
		// stand out instead of silently ending as "question missing".
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || dec.More() {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte die Frage als JSON schicken, z. B. {\"frage\": \"…\"}.", nil)
			return
		}

		answer, err := a.Ask(r.Context(), req.Question, req.Module)
		if err != nil {
			writeQuestionError(w, r, err, req.Module)
			return
		}
		writeJSON(w, http.StatusOK, answerToJSON(answer))
	}
}

func writeQuestionError(w http.ResponseWriter, r *http.Request, err error, module string) {
	switch {
	case errors.Is(err, core.ErrQuestionMissing):
		writeError(w, http.StatusBadRequest, CodeQuestionMissing, "Bitte stelle eine Frage.", nil)
	case errors.Is(err, core.ErrQuestionTooLong):
		writeError(w, http.StatusBadRequest, CodeQuestionTooLong,
			"Die Frage ist zu lang. Erlaubt sind höchstens 500 Zeichen.", nil)
	case errors.Is(err, core.ErrUnknownModule):
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"Unbekanntes Modul. Im Prototyp gibt es nur gesundheit.",
			map[string]any{"feld": "modul", "wert": module})
	case errors.Is(err, llm.ErrUnavailable), errors.Is(err, llm.ErrInvalidResponse):
		slog.Warn("question: model unavailable or answer unusable", "error", err)
		writeError(w, http.StatusServiceUnavailable, CodeModelUnavailable,
			"Die Suche ist gerade nicht erreichbar. Bitte versuche es später noch einmal.", nil)
	case errors.Is(err, context.Canceled):
		slog.Info("question canceled")
	default:
		writeInternalError(w, r, err)
	}
}
