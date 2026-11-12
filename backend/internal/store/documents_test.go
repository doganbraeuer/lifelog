package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

func ptr(s string) *string { return &s }

// sampleEntry is synthetic (see docs/API.md).
func sampleEntry() core.Entry {
	return core.Entry{
		ID:           "6727a1f3c2b4e5d6f7a8b9c0",
		Module:       "health",
		Type:         "lab_report",
		DocumentDate: ptr("2026-10-14"),
		ImportedAt:   time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer:       ptr("Labor Muster GmbH, Salzburg"),
		FullText:     "Hämoglobin 14,2 g/dl",
		Facts: []core.Fact{
			{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")},
			{ID: "f2", Name: "Befundart", Value: "Blutbild klein"},
		},
		File: core.File{
			ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund.pdf",
			MimeType: "application/pdf", Size: 184320, Sha256: strings.Repeat("ab", 32),
		},
		Corrections: []core.Correction{{
			Field: "facts.f1.value", Recognized: ptr("14,1"), Corrected: ptr("14,2"),
			CorrectedAt: time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC),
		}},
		Verification: sampleVerification(),
	}
}

func sampleVerification() core.Verification {
	at := time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC)
	ninety := 90
	return core.Verification{
		Status: core.VerificationChecked, Source: core.SourceMixed, Engine: "pdftotext 26.11.0, tesseract 5.5.4 deu",
		CheckedAt: &at, Confirmed: 1, Corrected: 1, Total: 2,
		Fields: []core.FieldCheck{
			{Field: "facts.f1", Status: core.FieldCorrectedConfirmed, Source: core.SourceOCR, Confidence: &ninety,
				Locations: []core.Location{{Page: 2, Box: core.Box{X: 0.1, Y: 0.2, W: 0.3, H: 0.04}}}},
			{Field: "facts.f2", Status: core.FieldUnconfirmed, Locations: []core.Location{}},
		},
	}
}

// An entry without verification (seed data, or stored before 2026-11-11)
// is left without the field and reads back as unchecked/not_run.
func TestEntryDoc_WithoutVerification(t *testing.T) {
	e := sampleEntry()
	e.Verification = core.Verification{}
	doc, err := toEntryDoc(e)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bson.Raw(raw).LookupErr("verification"); err == nil {
		t.Error("verification stored although empty")
	}
	var read entryDoc
	if err := bson.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}
	if got := read.toCore().Verification; !reflect.DeepEqual(got, core.Unchecked(core.ReasonNotRun)) {
		t.Errorf("got %+v, want unchecked/not_run", got)
	}
}

// Text import: a location is a range, stored with page 0.
func TestVerificationDoc_TextRange(t *testing.T) {
	v := core.Verification{Status: core.VerificationChecked, Source: core.SourceText, Total: 1, Confirmed: 1,
		Fields: []core.FieldCheck{{Field: "issuer", Status: core.FieldConfirmed, Source: core.SourceText,
			Locations: []core.Location{{Start: 3, End: 12}}}}}
	raw, err := bson.Marshal(toVerificationDoc(v))
	if err != nil {
		t.Fatal(err)
	}
	var read verificationDoc
	if err := bson.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}
	if got := read.toCore(); !reflect.DeepEqual(got, v) {
		t.Errorf("got %+v, want %+v", got, v)
	}
}

func TestOCRDoc_RoundTrip(t *testing.T) {
	r := core.OCRResult{
		Engine: "pdftotext 26.11.0, tesseract 5.5.4 deu",
		Pages: []core.OCRPage{
			{Number: 1, Source: core.SourcePDFText, Width: 595.2756, Height: 841.8898},
			{Number: 2, Source: core.SourceOCR, Width: 2480, Height: 3508},
		},
		Words: []core.OCRWord{
			{Text: "Hämoglobin", Source: core.SourcePDFText, Page: 1, Line: 0, Box: core.Box{X: 0.1, Y: 0.2, W: 0.15, H: 0.02}},
			{Text: "14,2", Source: core.SourceOCR, Confidence: 91.5, Page: 2, Line: 7, Box: core.Box{X: 0.3, Y: 0.2, W: 0.05, H: 0.02}},
		},
	}
	doc, err := toOCRDoc("6727a1f3c2b4e5d6f7a8b9c0", r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var read ocrDoc
	if err := bson.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}
	if read.EntryID.Hex() != "6727a1f3c2b4e5d6f7a8b9c0" {
		t.Errorf("_id %s", read.EntryID.Hex())
	}
	if got := read.toCore(); !reflect.DeepEqual(got, r) {
		t.Errorf("after BSON:\n%+v\nwant:\n%+v", got, r)
	}
	if _, err := toOCRDoc("nope", r); err == nil {
		t.Error("no error for an invalid entry ID")
	}
}

// There and back through real BSON, without database: nothing may be lost.
func TestEntryDoc_RoundTrip(t *testing.T) {
	original := sampleEntry()

	doc, err := toEntryDoc(original)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var read entryDoc
	if err := bson.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}

	if got := read.toCore(); !reflect.DeepEqual(got, original) {
		t.Errorf("after BSON:\n%+v\nwant:\n%+v", got, original)
	}
}

func TestEntryDoc_NullAndEmptyLists(t *testing.T) {
	e := sampleEntry()
	e.DocumentDate = nil
	e.Issuer = nil
	e.Facts = nil
	e.Corrections = nil

	doc, err := toEntryDoc(e)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := bson.Marshal(doc)
	var m bson.M
	if err := bson.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}

	// Date and issuer are stored as null in the database (field present).
	for _, field := range []string{"documentDate", "issuer"} {
		if v, ok := m[field]; !ok || v != nil {
			t.Errorf("%s = %v (present: %v), want null", field, v, ok)
		}
	}
	// Lists are empty arrays, not null.
	for _, field := range []string{"facts", "corrections"} {
		if _, ok := m[field].(bson.A); !ok {
			t.Errorf("%s is %T, want an array", field, m[field])
		}
	}

	// An old document with null lists becomes empty lists when read.
	doc.Facts, doc.Corrections = nil, nil
	c := doc.toCore()
	if c.Facts == nil || c.Corrections == nil {
		t.Error("toCore must return empty lists instead of nil")
	}
}

func TestEntryDoc_UnitMissingWhenNil(t *testing.T) {
	doc, _ := toEntryDoc(sampleEntry())
	raw, _ := bson.Marshal(doc.Facts[1])
	var m bson.M
	_ = bson.Unmarshal(raw, &m)
	if _, ok := m["unit"]; ok {
		t.Errorf("unit should be missing when nil: %v", m)
	}
}

func TestEntryDoc_InvalidIDs(t *testing.T) {
	e := sampleEntry()
	e.ID = "not-an-objectid"
	if _, err := toEntryDoc(e); err == nil {
		t.Error("invalid entry ID must give an error")
	}
	e = sampleEntry()
	e.File.ID = ""
	if _, err := toEntryDoc(e); err == nil {
		t.Error("invalid file ID must give an error")
	}
}

func TestChunkDoc_RoundTrip(t *testing.T) {
	original := core.Chunk{
		EntryID:  "6727a1f3c2b4e5d6f7a8b9c0",
		Module:   "health",
		Position: 2,
		Text:     "Hämoglobin 14,2 g/dl",
		// Values that are not exactly representable in decimal as float32.
		Embedding: []float32{0.1, -0.33333334, 1e-7, 0.99999994, -1},
	}
	doc, err := toChunkDoc(original)
	if err != nil {
		t.Fatal(err)
	}
	if doc.ID.IsZero() {
		t.Error("a chunk needs its own _id")
	}

	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var m bson.M
	_ = bson.Unmarshal(raw, &m)
	if _, ok := m["entryId"].(bson.ObjectID); !ok {
		t.Errorf("entryId is %T, want ObjectID (otherwise DeleteChunks finds nothing)", m["entryId"])
	}
	if _, ok := m["embedding"].(bson.A); !ok {
		t.Errorf("embedding is %T, want array (Atlas Vector Search)", m["embedding"])
	}

	var read chunkDoc
	if err := bson.Unmarshal(raw, &read); err != nil {
		t.Fatal(err)
	}
	if got := read.toCore(); !reflect.DeepEqual(got, original) {
		t.Errorf("after BSON:\n%+v\nwant:\n%+v", got, original)
	}
}

func TestChunkDoc_Invalid(t *testing.T) {
	if _, err := toChunkDoc(core.Chunk{EntryID: "x", Embedding: []float32{1}}); err == nil {
		t.Error("invalid entry ID must give an error")
	}
	if _, err := toChunkDoc(core.Chunk{EntryID: "6727a1f3c2b4e5d6f7a8b9c0"}); err == nil {
		t.Error("chunk without embedding must give an error")
	}
}

func TestNewID(t *testing.T) {
	a, b := NewID(), NewID()
	if a == b {
		t.Error("two IDs are equal")
	}
	if _, err := bson.ObjectIDFromHex(a); err != nil || len(a) != 24 {
		t.Errorf("%q is not an ObjectId: %v", a, err)
	}
}

func TestImportedAtBecomesUTC(t *testing.T) {
	e := sampleEntry()
	vienna := time.FixedZone("CET", 3600)
	e.ImportedAt = time.Date(2026, 11, 3, 19, 42, 7, 0, vienna) // = 18:42:07 UTC
	doc, _ := toEntryDoc(e)
	if doc.ImportedAt.Location() != time.UTC || doc.ImportedAt.Hour() != 18 {
		t.Errorf("ImportedAt = %v, want 18:42:07 UTC", doc.ImportedAt)
	}
}
