package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

// collOCR holds the words read from each original (text layer or OCR),
// one document per entry with _id = the entry's ID. PATCH checks corrected
// values against them without reading the original again (API.md 3.5).
const collOCR = "ocr"

// verificationDoc is entry.verification in MongoDB. matchPercent is not
// stored; it follows from confirmed and total.
type verificationDoc struct {
	Status    string          `bson:"status"`
	Reason    string          `bson:"reason,omitempty"`
	Source    string          `bson:"source,omitempty"`
	Engine    string          `bson:"engine,omitempty"`
	CheckedAt *time.Time      `bson:"checkedAt,omitempty"`
	Confirmed int             `bson:"confirmed"`
	Corrected int             `bson:"corrected"`
	Total     int             `bson:"total"`
	Fields    []fieldCheckDoc `bson:"fields"`
}

type fieldCheckDoc struct {
	Field      string        `bson:"field"`
	Status     string        `bson:"status"`
	Source     string        `bson:"source,omitempty"`
	Confidence *int          `bson:"confidence,omitempty"`
	Locations  []locationDoc `bson:"locations"`
}

// locationDoc: page 0 means a range in the full text (start, end).
type locationDoc struct {
	Page   int     `bson:"page"`
	X      float64 `bson:"x,omitempty"`
	Y      float64 `bson:"y,omitempty"`
	Width  float64 `bson:"width,omitempty"`
	Height float64 `bson:"height,omitempty"`
	Start  int     `bson:"start,omitempty"`
	End    int     `bson:"end,omitempty"`
}

// toVerificationDoc: nil for the zero value (an entry from cmd/seed, or
// one stored before verification existed), so the field is left out.
func toVerificationDoc(v core.Verification) *verificationDoc {
	if v.Status == "" {
		return nil
	}
	d := &verificationDoc{
		Status: v.Status, Reason: v.Reason, Source: v.Source, Engine: v.Engine,
		Confirmed: v.Confirmed, Corrected: v.Corrected, Total: v.Total,
		Fields: make([]fieldCheckDoc, 0, len(v.Fields)),
	}
	if v.CheckedAt != nil {
		t := v.CheckedAt.UTC()
		d.CheckedAt = &t
	}
	for _, f := range v.Fields {
		fd := fieldCheckDoc{Field: f.Field, Status: f.Status, Source: f.Source, Confidence: f.Confidence,
			Locations: make([]locationDoc, 0, len(f.Locations))}
		for _, l := range f.Locations {
			fd.Locations = append(fd.Locations, locationDoc{
				Page: l.Page, X: l.Box.X, Y: l.Box.Y, Width: l.Box.W, Height: l.Box.H, Start: l.Start, End: l.End,
			})
		}
		d.Fields = append(d.Fields, fd)
	}
	return d
}

// toCore: a missing verification becomes unchecked/not_run (API.md).
func (d *verificationDoc) toCore() core.Verification {
	if d == nil {
		return core.Unchecked(core.ReasonNotRun)
	}
	v := core.Verification{
		Status: d.Status, Reason: d.Reason, Source: d.Source, Engine: d.Engine,
		Confirmed: d.Confirmed, Corrected: d.Corrected, Total: d.Total,
		Fields: make([]core.FieldCheck, 0, len(d.Fields)),
	}
	if d.CheckedAt != nil {
		t := d.CheckedAt.UTC()
		v.CheckedAt = &t
	}
	for _, fd := range d.Fields {
		f := core.FieldCheck{Field: fd.Field, Status: fd.Status, Source: fd.Source, Confidence: fd.Confidence,
			Locations: make([]core.Location, 0, len(fd.Locations))}
		for _, l := range fd.Locations {
			f.Locations = append(f.Locations, core.Location{
				Page: l.Page, Box: core.Box{X: l.X, Y: l.Y, W: l.Width, H: l.Height}, Start: l.Start, End: l.End,
			})
		}
		v.Fields = append(v.Fields, f)
	}
	return v
}

// ocrDoc holds the words read from one original.
type ocrDoc struct {
	EntryID bson.ObjectID `bson:"_id"`
	Engine  string        `bson:"engine"`
	Pages   []ocrPageDoc  `bson:"pages"`
	Words   []ocrWordDoc  `bson:"words"`
}

type ocrPageDoc struct {
	Number int     `bson:"number"`
	Source string  `bson:"source"`
	Width  float64 `bson:"width"`
	Height float64 `bson:"height"`
}

type ocrWordDoc struct {
	Text       string  `bson:"text"`
	Source     string  `bson:"source"`
	Confidence float64 `bson:"confidence,omitempty"` // only OCR has one
	Page       int     `bson:"page"`
	Line       int     `bson:"line"`
	X          float64 `bson:"x"`
	Y          float64 `bson:"y"`
	Width      float64 `bson:"width"`
	Height     float64 `bson:"height"`
}

func toOCRDoc(entryID string, r core.OCRResult) (ocrDoc, error) {
	id, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return ocrDoc{}, fmt.Errorf("invalid entry ID: %w", err)
	}
	d := ocrDoc{EntryID: id, Engine: r.Engine,
		Pages: make([]ocrPageDoc, 0, len(r.Pages)), Words: make([]ocrWordDoc, 0, len(r.Words))}
	for _, p := range r.Pages {
		d.Pages = append(d.Pages, ocrPageDoc(p))
	}
	for _, w := range r.Words {
		d.Words = append(d.Words, ocrWordDoc{
			Text: w.Text, Source: w.Source, Confidence: w.Confidence, Page: w.Page, Line: w.Line,
			X: w.Box.X, Y: w.Box.Y, Width: w.Box.W, Height: w.Box.H,
		})
	}
	return d, nil
}

func (d ocrDoc) toCore() core.OCRResult {
	r := core.OCRResult{Engine: d.Engine,
		Pages: make([]core.OCRPage, 0, len(d.Pages)), Words: make([]core.OCRWord, 0, len(d.Words))}
	for _, p := range d.Pages {
		r.Pages = append(r.Pages, core.OCRPage(p))
	}
	for _, w := range d.Words {
		r.Words = append(r.Words, core.OCRWord{
			Text: w.Text, Source: w.Source, Confidence: w.Confidence, Page: w.Page, Line: w.Line,
			Box: core.Box{X: w.X, Y: w.Y, W: w.Width, H: w.Height},
		})
	}
	return r
}

// SaveOCR stores the words read from the entry's original. The upload
// calls it before SaveEntry (file → chunks → words → entry).
func (s *Store) SaveOCR(ctx context.Context, entryID string, r core.OCRResult) error {
	doc, err := toOCRDoc(entryID, r)
	if err != nil {
		return err
	}
	// Replace instead of insert: a repeated upload step must not fail
	// because of a leftover document with the same _id.
	if _, err := s.ocr.ReplaceOne(ctx, bson.D{{Key: "_id", Value: doc.EntryID}}, doc,
		options.Replace().SetUpsert(true)); err != nil {
		return fmt.Errorf("save ocr words: %w", err)
	}
	return nil
}

// GetOCR returns the stored words. found is false if there are none (text
// import, unchecked entry, or an entry from before verification existed).
func (s *Store) GetOCR(ctx context.Context, entryID string) (r core.OCRResult, found bool, err error) {
	id, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return core.OCRResult{}, false, core.ErrInvalidID
	}
	var doc ocrDoc
	err = s.ocr.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.OCRResult{}, false, nil
	}
	if err != nil {
		return core.OCRResult{}, false, fmt.Errorf("read ocr words: %w", err)
	}
	return doc.toCore(), true, nil
}

// DeleteOCR removes the stored words. If there are none, that is not an
// error (text import, or a delete that is repeated).
func (s *Store) DeleteOCR(ctx context.Context, entryID string) error {
	id, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return fmt.Errorf("invalid entry ID: %w", err)
	}
	if _, err := s.ocr.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}}); err != nil {
		return fmt.Errorf("delete ocr words: %w", err)
	}
	return nil
}

// SaveVerification sets only the verification of an entry (cmd/ocrcheck
// fills it in for entries stored before 2026-11-11). Nothing else is
// touched. found is false if the entry does not exist (anymore).
func (s *Store) SaveVerification(ctx context.Context, entryID string, v core.Verification) (found bool, err error) {
	id, err := bson.ObjectIDFromHex(entryID)
	if err != nil {
		return false, fmt.Errorf("invalid entry ID: %w", err)
	}
	res, err := s.entries.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "verification", Value: toVerificationDoc(v)}}}})
	if err != nil {
		return false, fmt.Errorf("save verification: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// OCRIDs returns the entry IDs of all stored words, so cmd/ocrcheck can
// find words whose entry no longer exists.
func (s *Store) OCRIDs(ctx context.Context) ([]string, error) {
	cur, err := s.ocr.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list ocr words: %w", err)
	}
	var docs []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("list ocr words: %w", err)
	}
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.ID.Hex())
	}
	return ids, nil
}
