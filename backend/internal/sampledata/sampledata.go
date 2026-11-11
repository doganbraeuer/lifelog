// Package sampledata contains the invented test entries. They are shared by
// cmd/seed (insert, remove) and cmd/calibrate (questions whose answer is in
// a certain entry). All data is invented.
//
// The content (full texts, facts, issuers) stays German: it imitates real
// documents, and the calibration compares its scores across changes, so it
// must stay byte for byte the same.
package sampledata

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Fixed IDs of the test entries. Fixed, so cmd/seed can delete exactly these
// again and cmd/calibrate knows which entry contains the answer.
const (
	LabReport    = "6727a1f3c2b4e5d6f7a8b9c0"
	DoctorLetter = "6727a20bc2b4e5d6f7a8b9d4"
	Note         = "6727a2a9c2b4e5d6f7a8b9e7"
	Prescription = "6727a2f0c2b4e5d6f7a8b9f2"
)

// Entries covers the cases of the timeline: different types, an entry
// without date and without issuer, two entries with the same date.
//
// The entries look like text imports: the original file is the full text as
// text.txt.
func Entries() []core.Entry {
	return []core.Entry{
		textEntry(
			LabReport, "6727a1f3c2b4e5d6f7a8b9c1", "laborbefund",
			ptr("2026-10-14"), ptr("Labor Muster GmbH, Salzburg"), "2026-11-03T18:42:07Z",
			"Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\n"+
				"Patient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\n"+
				"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
			[]core.Fact{
				{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
				{ID: "f2", Name: "Leukozyten", Value: "6,8", Unit: ptr("G/l")},
				{ID: "f3", Name: "CRP", Value: "<0,5", Unit: ptr("mg/dl")},
				{ID: "f4", Name: "Befundart", Value: "Blutbild klein"},
			},
		),
		textEntry(
			DoctorLetter, "6727a20bc2b4e5d6f7a8b9d5", "arztbrief",
			ptr("2026-09-02"), ptr("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T18:50:12Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\nBeispielgasse 7, 5020 Salzburg\n\n"+
				"Arztbrief vom 02.09.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\n"+
				"Vorstellung zur Routinekontrolle. Blutdruck 128/82 mmHg.\n"+
				"Kontrolle in 6 Monaten vereinbart.",
			[]core.Fact{
				{ID: "f1", Name: "Blutdruck", Value: "128/82", Unit: ptr("mmHg")},
				{ID: "f2", Name: "Nächste Kontrolle", Value: "in 6 Monaten"},
			},
		),
		textEntry(
			Note, "6727a2a9c2b4e5d6f7a8b9e8", "sonstiges",
			nil, nil, "2026-11-03T19:01:44Z",
			"Notiz ohne Datum und ohne Absender.\nImpfpass liegt in der Schublade.",
			[]core.Fact{},
		),
		textEntry(
			Prescription, "6727a2f0c2b4e5d6f7a8b9f3", "rezept",
			ptr("2026-10-14"), ptr("Dr. Erika Beispiel, Allgemeinmedizin"), "2026-11-03T19:10:00Z",
			"Dr. Erika Beispiel, Allgemeinmedizin\n\nRezept vom 14.10.2026\n"+
				"Patient: Max Mustermann\n\nBeispielin 100 mg, 1-0-0, 30 Stück",
			[]core.Fact{
				{ID: "f1", Name: "Präparat", Value: "Beispielin 100 mg"},
				{ID: "f2", Name: "Dosierung", Value: "1-0-0"},
				{ID: "f3", Name: "Menge", Value: "30", Unit: ptr("Stück")},
			},
		),
	}
}

// Chunks splits the full text of an entry as the upload does
// (service.Uploader): same limit, Position from 1, module of the entry. The
// embeddings are still missing; the caller sets them.
func Chunks(e core.Entry) []core.Chunk {
	texts := core.SplitIntoChunks(e.FullText, core.ChunkMaxChars)
	chunks := make([]core.Chunk, len(texts))
	for i, t := range texts {
		chunks[i] = core.Chunk{EntryID: e.ID, Module: e.Module, Position: i + 1, Text: t}
	}
	return chunks
}

func ptr(s string) *string { return &s }

func utc(text string) time.Time {
	t, err := time.Parse(time.RFC3339, text)
	if err != nil {
		panic(err)
	}
	return t
}

// textEntry builds an entry as if the full text had come in by text import.
// Hash and size match the text.
func textEntry(id, fileID, typ string, date, issuer *string, importedAt, fullText string, facts []core.Fact) core.Entry {
	hash := sha256.Sum256([]byte(fullText))
	return core.Entry{
		ID:           id,
		Module:       "gesundheit",
		Type:         typ,
		DocumentDate: date,
		ImportedAt:   utc(importedAt),
		Issuer:       issuer,
		FullText:     fullText,
		Facts:        facts,
		File: core.File{
			ID:       fileID,
			FileName: "text.txt",
			MimeType: "text/plain; charset=utf-8",
			Size:     int64(len(fullText)),
			Sha256:   hex.EncodeToString(hash[:]),
		},
		Corrections: []core.Correction{},
	}
}

// Throwaway builds an invented entry for trying out DELETE
// (cmd/seed -throwaway). The caller assigns new IDs, so it never hits a
// fixed test entry or an uploaded document. The time is in the text, so
// every throwaway entry has a different hash and is no duplicate.
//
// Instead of a real embedding, the chunks have a fixed unit vector
// (1, 0, 0, …): no Gemini call. Its score for real questions is around 0.5,
// far below core.MinScore, so search does not show it.
//
// Text, issuer and file name stay German because the app shows them.
func Throwaway(id, fileID string, now time.Time) (core.Entry, []core.Chunk) {
	now = now.UTC().Truncate(time.Millisecond)
	fullText := "Wegwerf-Eintrag zum Testen des Löschens.\n\nAngelegt am " +
		now.Format(time.RFC3339Nano) + ". Enthält keine Gesundheitsdaten."
	hash := sha256.Sum256([]byte(fullText))
	e := core.Entry{
		ID:         id,
		Module:     "gesundheit",
		Type:       "sonstiges",
		ImportedAt: now,
		Issuer:     ptr("Wegwerf-Test"),
		FullText:   fullText,
		Facts:      []core.Fact{},
		File: core.File{
			ID:       fileID,
			FileName: "wegwerf.txt",
			MimeType: "text/plain; charset=utf-8",
			Size:     int64(len(fullText)),
			Sha256:   hex.EncodeToString(hash[:]),
		},
		Corrections: []core.Correction{},
	}
	chunks := Chunks(e)
	for i := range chunks {
		v := make([]float32, llm.EmbeddingDimensions)
		v[0] = 1
		chunks[i].Embedding = v
	}
	return e, chunks
}
