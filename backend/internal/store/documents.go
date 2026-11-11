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
//   - documentDate as string "YYYY-MM-DD" or null. A string instead of a
//     BSON date, so no time zone can shift the date.
//   - importedAt as BSON date (UTC)
//   - lists always as array, never as null
//
// The BSON field names are the same as the JSON names of the API.

type entryDoc struct {
	ID           bson.ObjectID   `bson:"_id"`
	Module       string          `bson:"module"`
	Type         string          `bson:"type"`
	DocumentDate *string         `bson:"documentDate"`
	ImportedAt   time.Time       `bson:"importedAt"`
	Issuer       *string         `bson:"issuer"`
	FullText     string          `bson:"fullText"`
	Facts        []factDoc       `bson:"facts"`
	File         fileDoc         `bson:"file"`
	Corrections  []correctionDoc `bson:"corrections"`
}

// factDoc and correctionDoc have exactly the field names of core.Fact and
// core.Correction, so they can be converted directly (factDoc(f)).
type factDoc struct {
	ID    string  `bson:"id"`
	Name  string  `bson:"name"`
	Value string  `bson:"value"`
	Unit  *string `bson:"unit,omitempty"`
}

type fileDoc struct {
	ID       bson.ObjectID `bson:"id"`
	FileName string        `bson:"fileName"`
	MimeType string        `bson:"mimeType"`
	Size     int64         `bson:"size"`
	Sha256   string        `bson:"sha256"`
}

type correctionDoc struct {
	Field       string    `bson:"field"`
	Recognized  *string   `bson:"recognized"`
	Corrected   *string   `bson:"corrected"`
	CorrectedAt time.Time `bson:"correctedAt"`
}

// entrySummaryDoc only reads the fields the timeline needs.
type entrySummaryDoc struct {
	ID           bson.ObjectID `bson:"_id"`
	Module       string        `bson:"module"`
	Type         string        `bson:"type"`
	DocumentDate *string       `bson:"documentDate"`
	ImportedAt   time.Time     `bson:"importedAt"`
	Issuer       *string       `bson:"issuer"`
}

// summaryProjection tells MongoDB which fields to send for the list. Full
// text, facts etc. stay in the database, the response stays small.
var summaryProjection = bson.D{
	{Key: "module", Value: 1},
	{Key: "type", Value: 1},
	{Key: "documentDate", Value: 1},
	{Key: "importedAt", Value: 1},
	{Key: "issuer", Value: 1},
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

// chunkDoc is a chunk in the collection "chunks".
//
// The embedding is stored as a plain array of numbers (768 numbers). Atlas
// Vector Search reads exactly this field (path "embedding"), and "module" is
// registered there as a filter field.
type chunkDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	EntryID   bson.ObjectID `bson:"entryId"`
	Module    string        `bson:"module"`
	Position  int           `bson:"position"`
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
