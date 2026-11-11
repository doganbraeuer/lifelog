// ocrcheck checks entries against their originals with the second reader
// (PDF text layer or OCR, docs/API.md, verification). It never calls a
// model. Start from backend/:
//
//	go run ./cmd/ocrcheck                  # show what it would fill in, and measure
//	go run ./cmd/ocrcheck -write           # fill in the verification of old entries
//	go run ./cmd/ocrcheck -values -out ../docs/ocr/2026-11-11-tesseract.md
//
// It does three things:
//
//  1. Fill in: entries stored before 2026-11-11 have no verification
//     (unchecked/not_run). ocrcheck reads their originals from the database
//     and checks them as the upload does. Without -write it only prints
//     the result it would store; with -write it stores the words
//     (collection ocr) and the verification.
//  2. Words without an entry: lists stored words whose entry no longer
//     exists (deleting should remove them). Only reports, deletes nothing.
//  3. Measure: reads every file in demo/ (PDFs once via the text layer and
//     once with forced OCR, photos with OCR) and checks it against the entry
//     uploaded from the same document. Shows per document and per variant
//     how much is confirmed, so we can decide whether Tesseract is good
//     enough or PaddleOCR is needed.
//
// Needs Poppler and Tesseract (backend/README.md, verification).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/ocr"
	"lifelog/backend/internal/ocr/poppler"
	"lifelog/backend/internal/ocr/tesseract"
	"lifelog/backend/internal/service"
	"lifelog/backend/internal/store"
)

func main() {
	write := flag.Bool("write", false, "store the verification of entries that were never checked (default: only show it)")
	demo := flag.String("demo", "../demo", "folder with the demo documents")
	out := flag.String("out", "", "also write the measurement as Markdown to this file, e.g. ../docs/ocr/2026-11-11-tesseract.md")
	values := flag.Bool("values", false, "list the fields that were not confirmed, with expected value and OCR row")
	flag.Parse()

	if err := run(*write, *demo, *out, *values); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(write bool, demoDir, out string, values bool) error {
	if err := config.LoadDotenv(); err != nil {
		return err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	pdf, err := poppler.New()
	if err != nil {
		return fmt.Errorf("%w (see backend/README.md, verification)", err)
	}
	engine, err := tesseract.New(c.TesseractLang)
	if err != nil {
		return fmt.Errorf("%w (see backend/README.md, verification)", err)
	}
	pipeline := &ocr.Pipeline{Engine: engine, PDF: pdf}
	programs := pdf.Name() + ", " + engine.Name()

	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		return err
	}
	defer db.Disconnect(context.Background())

	entries, err := allEntries(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("Database %q: %d entries. Programs: %s. No model is called.\n\n", c.MongoDB, len(entries), programs)

	if err := fillIn(ctx, service.NewVerifier(db, pipeline), entries, write); err != nil {
		return err
	}
	if err := orphans(ctx, db, entries); err != nil {
		return err
	}

	rep, err := measure(ctx, pipeline, entries, demoDir)
	if err != nil {
		return err
	}
	rep.Programs, rep.Database, rep.AllValues = programs, c.MongoDB, values
	rep.Date = time.Now().Format("2006-01-02")
	md := rep.Markdown()
	fmt.Println(md)
	if out != "" {
		if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
			return err
		}
		fmt.Println("Measurement written to", out)
	}
	return nil
}

// allEntries reads every entry of every module, in full.
func allEntries(ctx context.Context, db *store.Store) ([]core.Entry, error) {
	var out []core.Entry
	for _, module := range core.Modules() {
		list, err := db.ListSummaries(ctx, module)
		if err != nil {
			return nil, err
		}
		for _, s := range list {
			e, found, err := db.GetEntry(ctx, s.ID)
			if err != nil {
				return nil, err
			}
			if found {
				out = append(out, e)
			}
		}
	}
	slices.SortFunc(out, func(a, b core.Entry) int { return a.ImportedAt.Compare(b.ImportedAt) })
	return out, nil
}

// fillIn checks the entries that were never checked and, with write,
// stores the result.
func fillIn(ctx context.Context, v *service.Verifier, entries []core.Entry, write bool) error {
	var todo []core.Entry
	for _, e := range entries {
		if service.NeedsCheck(e) {
			todo = append(todo, e)
		}
	}
	fmt.Printf("Fill in: %d of %d entries were never checked (not_run).\n", len(todo), len(entries))
	for _, e := range todo {
		ver, words, err := v.Check(ctx, e)
		if err != nil {
			return fmt.Errorf("entry %s: %w", e.ID, err)
		}
		result := ver.Status + "/" + ver.Reason
		if ver.Status == core.VerificationChecked {
			result = fmt.Sprintf("checked via %s, %d of %d confirmed", ver.Source, ver.Confirmed, ver.Total)
			if words != nil {
				result += fmt.Sprintf(", %d words", len(words.Words))
			}
		}
		fmt.Printf("  %s  %-13s %-10s %-45s → %s\n", e.ID, e.Type, deref(e.DocumentDate), e.File.FileName, result)
		if write {
			if err := v.Save(ctx, e.ID, ver, words); err != nil {
				return fmt.Errorf("entry %s: %w", e.ID, err)
			}
		}
	}
	switch {
	case len(todo) == 0:
	case write:
		fmt.Printf("Stored for %d entries.\n", len(todo))
	default:
		fmt.Println("Dry run: nothing was written. Run with -write to store these results.")
	}
	fmt.Println()
	return nil
}

// orphans reports stored words whose entry no longer exists.
func orphans(ctx context.Context, db *store.Store, entries []core.Entry) error {
	ids, err := db.OCRIDs(ctx)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, e := range entries {
		known[e.ID] = true
	}
	var left []string
	for _, id := range ids {
		if !known[id] {
			left = append(left, id)
		}
	}
	if len(left) == 0 {
		fmt.Printf("Stored words: %d, none without an entry.\n\n", len(ids))
		return nil
	}
	fmt.Printf("Stored words: %d, WITHOUT an entry: %s (not deleted)\n\n", len(ids), strings.Join(left, ", "))
	return nil
}

// measure reads every demo file and checks it against the entry uploaded
// from the same document.
func measure(ctx context.Context, p *ocr.Pipeline, entries []core.Entry, demoDir string) (report, error) {
	files, err := demoFiles(demoDir)
	if err != nil {
		return report{}, fmt.Errorf("demo folder: %w", err)
	}
	forced := &ocr.Pipeline{Engine: p.Engine, PDF: p.PDF, ForceOCR: true}

	refs := map[string]core.Entry{} // document → reference entry
	var rep report
	for _, f := range files {
		ref, ok := refs[f.Doc]
		if !ok {
			ref, ok, err = reference(f, files, entries, demoDir)
			if err != nil {
				return report{}, err
			}
			if !ok {
				if !slices.Contains(rep.Missing, f.Doc) {
					rep.Missing = append(rep.Missing, f.Doc)
				}
				continue
			}
			refs[f.Doc] = ref
		}

		data, err := os.ReadFile(filepath.Join(demoDir, f.Path))
		if err != nil {
			return report{}, err
		}
		switch f.variantName() {
		case "text":
			start := time.Now()
			v := core.VerifyText(ref, time.Now())
			vo := core.VerifyText(withoutUnits(ref), time.Now())
			rep.Rows = append(rep.Rows, measureRow(f.Doc, "text", kindText, ref, v, vo,
				core.OCRResult{Words: core.WordsFromText(ref.FullText)}, time.Since(start)))
		case "pdf":
			rep.Rows = append(rep.Rows, measureFile(ctx, p, f, data, core.MimePDF, "pdf", kindPDFText, ref))
			rep.Rows = append(rep.Rows, measureFile(ctx, forced, f, data, core.MimePDF, "pdf, forced OCR", kindPDFOCR, ref))
		default:
			mime, err := core.DetectFileType(data)
			if err != nil {
				return report{}, fmt.Errorf("%s: %w", f.Path, err)
			}
			variant := f.variantName()
			if variant == "photo" && strings.HasPrefix(f.Path, "live") {
				variant = "photo (live)" // a second photo of the same document, see demo/README.md
			}
			rep.Rows = append(rep.Rows, measureFile(ctx, p, f, data, mime, variant, kindPhoto, ref))
		}
	}
	return rep, nil
}

func measureFile(ctx context.Context, p *ocr.Pipeline, f demoFile, data []byte, mime, variant, kind string, ref core.Entry) row {
	ctx, cancel := context.WithTimeout(ctx, service.OCRTimeout)
	defer cancel()
	start := time.Now()
	r, err := p.Recognize(ctx, data, mime)
	d := time.Since(start)
	if err != nil {
		return row{Doc: f.Doc, Variant: variant, Kind: kind, Reason: ocr.Reason(err), Date: "–", Issuer: "–", Duration: d}
	}
	now := time.Now()
	return measureRow(f.Doc, variant, kind, ref, core.Verify(ref, r, now), core.Verify(withoutUnits(ref), r, now), r, d)
}

// reference finds the entry uploaded from the document of f: by file name
// for files, by content for text imports (they are all called text.txt).
func reference(f demoFile, files []demoFile, entries []core.Entry, demoDir string) (core.Entry, bool, error) {
	var names []string
	var texts []string
	for _, other := range files {
		if other.Doc != f.Doc {
			continue
		}
		if other.variantName() == "text" {
			data, err := os.ReadFile(filepath.Join(demoDir, other.Path))
			if err != nil {
				return core.Entry{}, false, err
			}
			texts = append(texts, string(data))
		} else {
			names = append(names, other.Name)
		}
	}
	var hits []core.Entry
	for _, e := range entries {
		if e.File.MimeType == core.MimeText {
			if slices.ContainsFunc(texts, func(t string) bool { return strings.TrimSpace(t) == strings.TrimSpace(e.FullText) }) {
				hits = append(hits, e)
			}
		} else if slices.Contains(names, e.File.FileName) {
			hits = append(hits, e)
		}
	}
	switch len(hits) {
	case 0:
		return core.Entry{}, false, nil
	case 1:
		return hits[0], true, nil
	}
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return core.Entry{}, false, errors.New("several entries for " + f.Doc + ": " + strings.Join(ids, ", "))
}
