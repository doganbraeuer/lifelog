// Package core contains the domain logic without database, network or HTTP:
// types, module catalog, sorting and validation rules. Everything here is
// deterministic and covered by unit tests.
package core

import (
	"errors"
	"time"
)

// ErrDuplicate: there already is an entry with exactly this file (same
// SHA-256 hash). The store reports it when the unique index fires.
var ErrDuplicate = errors.New("duplicate: file already exists")

// ErrEntryNotFound: there is no entry with this ID
// (→ 404 ENTRY_NOT_FOUND).
var ErrEntryNotFound = errors.New("entry not found")

// ErrInvalidID: the ID is not an ObjectId (24 hex characters).
var ErrInvalidID = errors.New("invalid ID")

// ErrOriginalMissing: the entry exists, but its original file does not. Must
// not happen (the file is stored before the entry and deleted after it); if
// it does, it is an internal error.
var ErrOriginalMissing = errors.New("original file missing")

// CheckID checks whether id looks like an ObjectId: exactly 24 hex
// characters. This way the API answers "/entries/abc" with 400 instead of
// 404, and the database is not even asked.
func CheckID(id string) error {
	if len(id) != 24 {
		return ErrInvalidID
	}
	for _, r := range id {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'f' || 'A' <= r && r <= 'F') {
			return ErrInvalidID
		}
	}
	return nil
}

// Where the document date comes from (docs/API.md, section 2,
// documentDateSource).
const (
	// DateSourceDocument: the date is from the document or the text, or
	// there is none. Also for entries stored before the field existed.
	DateSourceDocument = "document"
	// DateSourceImport: a text import named no date, so the backend set the
	// day of the import (Europe/Vienna). This date is not in the text and
	// is therefore never checked by the verification.
	DateSourceImport = "import"
)

// Entry is a full entry as in docs/API.md, section 2.
type Entry struct {
	ID string // ObjectId as hex string (24 characters)
	// UserID is the owner (docs/API.md, per-user data). The store fills it
	// when reading; it is never part of the API response.
	UserID       string
	Module       string  // e.g. "health"
	Type         string  // e.g. "lab_report", valid within the module
	DocumentDate *string // "YYYY-MM-DD" or nil if not recognized
	// DocumentDateSource is DateSourceDocument or DateSourceImport.
	DocumentDateSource string
	ImportedAt         time.Time // UTC
	Issuer             *string   // issuing body or nil
	FullText           string
	Facts              []Fact
	File               File
	Corrections        []Correction
	Verification       Verification // check by a second reader; zero value until the upload sets it
}

// Fact is an extracted fact. Value always stays a string, exactly as on the
// document ("14,2", "<0,5"), so nothing is lost by conversion.
type Fact struct {
	ID    string // "f1", "f2", …
	Name  string
	Value string
	Unit  *string // nil if the document names no unit
}

// File refers to the stored original file.
type File struct {
	ID       string
	FileName string
	MimeType string
	Size     int64
	Sha256   string
}

// Correction records the originally recognized and the corrected value.
type Correction struct {
	Field       string
	Recognized  *string
	Corrected   *string
	CorrectedAt time.Time
}

// EntrySummary is the short form for the timeline (without full text, facts
// etc.).
type EntrySummary struct {
	ID                 string
	Module             string
	Type               string
	DocumentDate       *string
	DocumentDateSource string // DateSourceDocument or DateSourceImport
	ImportedAt         time.Time
	Issuer             *string
}

// DateSourceOrDocument: an entry stored before 2026-11-23 has no source; its
// date always came from the document (or is null).
func DateSourceOrDocument(source string) string {
	if source == "" {
		return DateSourceDocument
	}
	return source
}

// ImportDate gives a text import without a document date the day of its
// import in Europe/Vienna and marks it (DateSourceImport). Entries with a
// date are not changed. The caller only calls it for text imports; files
// keep a missing date (API.md 3.1).
func ImportDate(e *Entry) {
	if e.DocumentDate != nil {
		return
	}
	day := LocalDay(e.ImportedAt)
	e.DocumentDate = &day
	e.DocumentDateSource = DateSourceImport
}

// Chunk is a piece of the full text with its embedding, the basis of search
// (POST /questions). The module is stored in the chunk itself so the vector
// search can filter by it directly.
type Chunk struct {
	EntryID   string
	Module    string
	Position  int // order within the full text, starting at 1
	Text      string
	Embedding []float32
}
