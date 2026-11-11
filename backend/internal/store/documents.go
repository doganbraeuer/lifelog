package store

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

// The *Doc types describe how an entry looks in MongoDB. They are separate
// from the core types, so the domain logic knows nothing about BSON.
//
// Stored are:
//   - _id as ObjectId
//   - dokumentdatum as string "YYYY-MM-DD" or null. A string instead of a
//     BSON date, so no time zone can shift the date.
//   - importdatum as BSON date (UTC)
//   - lists always as array, never as null
//
// The BSON field names are still German; they change when the database is
// refilled (plan step B9).

type entryDoc struct {
	ID           bson.ObjectID   `bson:"_id"`
	Module       string          `bson:"modul"`
	Type         string          `bson:"typ"`
	DocumentDate *string         `bson:"dokumentdatum"`
	ImportedAt   time.Time       `bson:"importdatum"`
	Issuer       *string         `bson:"quelle"`
	FullText     string          `bson:"volltext"`
	Facts        []factDoc       `bson:"infos"`
	File         fileDoc         `bson:"datei"`
	Corrections  []correctionDoc `bson:"korrekturen"`
}

// factDoc and correctionDoc have exactly the field names of core.Fact and
// core.Correction, so they can be converted directly (factDoc(f)).
type factDoc struct {
	ID    string  `bson:"id"`
	Name  string  `bson:"name"`
	Value string  `bson:"wert"`
	Unit  *string `bson:"einheit,omitempty"`
}

type fileDoc struct {
	ID       bson.ObjectID `bson:"id"`
	FileName string        `bson:"dateiname"`
	MimeType string        `bson:"mimeTyp"`
	Size     int64         `bson:"groesse"`
	Sha256   string        `bson:"sha256"`
}

type correctionDoc struct {
	Field       string    `bson:"feld"`
	Recognized  *string   `bson:"erkannt"`
	Corrected   *string   `bson:"korrigiert"`
	CorrectedAt time.Time `bson:"zeitpunkt"`
}

// entrySummaryDoc only reads the fields the timeline needs.
type entrySummaryDoc struct {
	ID           bson.ObjectID `bson:"_id"`
	Module       string        `bson:"modul"`
	Type         string        `bson:"typ"`
	DocumentDate *string       `bson:"dokumentdatum"`
	ImportedAt   time.Time     `bson:"importdatum"`
	Issuer       *string       `bson:"quelle"`
}

// summaryProjection tells MongoDB which fields to send for the list. Full
// text, facts etc. stay in the database, the response stays small.
var summaryProjection = bson.D{
	{Key: "modul", Value: 1},
	{Key: "typ", Value: 1},
	{Key: "dokumentdatum", Value: 1},
	{Key: "importdatum", Value: 1},
	{Key: "quelle", Value: 1},
}

func toEntryDoc(e core.Entry) (entryDoc, error) {
	id, err := bson.ObjectIDFromHex(e.ID)
	if err != nil {
		return entryDoc{}, fmt.Errorf("invalid entry ID: %w", err)
	}
	fileID, err := bson.ObjectIDFromHex(e.File.ID)
	if err != nil {
		return entryDoc{}, fmt.Errorf("invalid file ID: %w", err)
	}

	facts := make([]factDoc, 0, len(e.Facts))
	for _, f := range e.Facts {
		facts = append(facts, factDoc(f))
	}
	corrections := make([]correctionDoc, 0, len(e.Corrections))
	for _, c := range e.Corrections {
		c.CorrectedAt = c.CorrectedAt.UTC()
		corrections = append(corrections, correctionDoc(c))
	}

	return entryDoc{
		ID:           id,
		Module:       e.Module,
		Type:         e.Type,
		DocumentDate: e.DocumentDate,
		ImportedAt:   e.ImportedAt.UTC(),
		Issuer:       e.Issuer,
		FullText:     e.FullText,
		Facts:        facts,
		File: fileDoc{
			ID:       fileID,
			FileName: e.File.FileName,
			MimeType: e.File.MimeType,
			Size:     e.File.Size,
			Sha256:   e.File.Sha256,
		},
		Corrections: corrections,
	}, nil
}

func (d entryDoc) toCore() core.Entry {
	facts := make([]core.Fact, 0, len(d.Facts))
	for _, f := range d.Facts {
		facts = append(facts, core.Fact(f))
	}
	corrections := make([]core.Correction, 0, len(d.Corrections))
	for _, c := range d.Corrections {
		corrections = append(corrections, core.Correction(c))
	}
	return core.Entry{
		ID:           d.ID.Hex(),
		Module:       d.Module,
		Type:         d.Type,
		DocumentDate: d.DocumentDate,
		ImportedAt:   d.ImportedAt.UTC(),
		Issuer:       d.Issuer,
		FullText:     d.FullText,
		Facts:        facts,
		File: core.File{
			ID:       d.File.ID.Hex(),
			FileName: d.File.FileName,
			MimeType: d.File.MimeType,
			Size:     d.File.Size,
			Sha256:   d.File.Sha256,
		},
		Corrections: corrections,
	}
}

func (d entrySummaryDoc) toCore() core.EntrySummary {
	return core.EntrySummary{
		ID:           d.ID.Hex(),
		Module:       d.Module,
		Type:         d.Type,
		DocumentDate: d.DocumentDate,
		ImportedAt:   d.ImportedAt.UTC(),
		Issuer:       d.Issuer,
	}
}

// chunkDoc is a chunk in the collection "abschnitte".
//
// The embedding is stored as a plain array of numbers (768 numbers). Atlas
// Vector Search reads exactly this field (path "embedding"), and "modul" is
// registered there as a filter field.
type chunkDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	EntryID   bson.ObjectID `bson:"eintragId"`
	Module    string        `bson:"modul"`
	Position  int           `bson:"nr"`
	Text      string        `bson:"text"`
	Embedding []float32     `bson:"embedding"`
}

func toChunkDoc(c core.Chunk) (chunkDoc, error) {
	entryID, err := bson.ObjectIDFromHex(c.EntryID)
	if err != nil {
		return chunkDoc{}, fmt.Errorf("invalid entry ID in chunk: %w", err)
	}
	if len(c.Embedding) == 0 {
		return chunkDoc{}, fmt.Errorf("chunk %d has no embedding", c.Position)
	}
	return chunkDoc{
		ID:        bson.NewObjectID(),
		EntryID:   entryID,
		Module:    c.Module,
		Position:  c.Position,
		Text:      c.Text,
		Embedding: c.Embedding,
	}, nil
}

func (d chunkDoc) toCore() core.Chunk {
	return core.Chunk{
		EntryID:   d.EntryID.Hex(),
		Module:    d.Module,
		Position:  d.Position,
		Text:      d.Text,
		Embedding: d.Embedding,
	}
}
