// ocrcheck checks entries against their originals with the second reader
// (PDF text layer or OCR, docs/API.md, verification). It never calls a
// model. Start from backend/:
//
//	go run ./cmd/ocrcheck                  # show what it would fill in, and measure
//	go run ./cmd/ocrcheck -write           # fill in the verification of old entries
//	go run ./cmd/ocrcheck -recheck         # show what checking every entry again would change
//	go run ./cmd/ocrcheck -recheck -write  # check every entry again and store the result
//	go run ./cmd/ocrcheck -values -out ../docs/ocr/2026-11-11-tesseract.md
//	go run ./cmd/ocrcheck -upscale 0       # measure without scaling small images up
//	go run ./cmd/ocrcheck -downscale 3000  # measure with large images scaled down to 3000 pixels
//	go run ./cmd/ocrcheck -extra ~/Desktop/01_lab_report_2026-01-20_phone.jpg
//
// It does three things:
//
//  1. Fill in: entries stored before 2026-11-11 have no verification
//     (unchecked/not_run). ocrcheck reads their originals from the database
//     and checks them as the upload does. Without -write it only prints
//     the result it would store; with -write it stores the words
//     (collection ocr) and the verification. With -recheck it checks every
//     entry again (e.g. after a change to OCR or the rules), not only the
//     unchecked ones, and shows old → new; it stores only with -write.
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
	write := flag.Bool("write", false, "store the verification (default: only show it)")
	recheck := flag.Bool("recheck", false, "check every entry again, not only those that were never checked")
	demo := flag.String("demo", "../demo", "folder with the demo documents")
	out := flag.String("out", "", "also write the measurement as Markdown to this file, e.g. ../docs/ocr/2026-11-11-tesseract.md")
	values := flag.Bool("values", false, "list the fields that were not confirmed, with expected value and OCR row")
	upscale := flag.Int("upscale", ocr.DefaultMinLongSide, "scale images whose long side is shorter up to this many pixels before OCR; 0 = off")
	downscale := flag.Int("downscale", ocr.DefaultMaxLongSide, "scale images whose long side is longer down to this many pixels before OCR; 0 = off")
	extra := flag.String("extra", "", "comma-separated files outside demo/ to measure too, named like the document, e.g. 01_lab_report_2026-01-20_phone.jpg")
	flag.Parse()

	if err := run(options{write: *write, recheck: *recheck, demoDir: *demo, out: *out, values: *values, upscale: *upscale, downscale: *downscale, extra: *extra}); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

type options struct {
	write     bool
	recheck   bool
	demoDir   string
	out       string
	values    bool
	upscale   int
	downscale int
	extra     string
}

func run(o options) error {
	if o.upscale < 0 || o.downscale < 0 {
		return errors.New("-upscale and -downscale must be 0 or more")
	}
	if o.upscale > 0 && o.downscale > 0 && o.upscale > o.downscale {
		return errors.New("-upscale must not be larger than -downscale")
	}
	extra, err := extraFiles(strings.Split(o.extra, ","))
	if err != nil {
		return fmt.Errorf("-extra: %w", err)
	}

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
	images := ocr.DefaultImageOptions()
	images.MinLongSide, images.MaxLongSide = o.upscale, o.downscale
	pipeline := &ocr.Pipeline{Engine: engine, PDF: pdf, Image: images}
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

	if err := fillIn(ctx, service.NewVerifier(db, pipeline), entries, o.recheck, o.write); err != nil {
		return err
	}
	if err := orphans(ctx, db, entries); err != nil {
		return err
	}

	rep, err := measure(ctx, pipeline, entries, o.demoDir, extra)
	if err != nil {
		return err
	}
	rep.Programs, rep.Database, rep.AllValues, rep.Upscale = programs, c.MongoDB, o.values, imageNote(images)
	rep.Date = time.Now().Format("2006-01-02")
	md := rep.Markdown()
	fmt.Println(md)
	if o.out != "" {
		if err := os.WriteFile(o.out, []byte(md), 0o644); err != nil {
			return err
		}
		fmt.Println("Measurement written to", o.out)
	}
	return nil
}

// imageNote describes the image settings for the report.
func imageNote(o ocr.ImageOptions) string {
	up, down := "never scaled up", "never scaled down"
	if o.MinLongSide > 0 {
		up = fmt.Sprintf("scaled up to a long side of %d pixels if smaller (at most twice)", o.MinLongSide)
	}
	if o.MaxLongSide > 0 {
		down = fmt.Sprintf("scaled down to %d pixels if larger", o.MaxLongSide)
	}
	return "turned upright, " + up + ", " + down + ", as RGB PNG; Tesseract with --dpi 300"
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

// fillIn checks the entries that were never checked (with recheck: all
// entries) and, with write, stores the result.
func fillIn(ctx context.Context, v *service.Verifier, entries []core.Entry, recheck, write bool) error {
	var todo []core.Entry
	for _, e := range entries {
		if recheck || service.NeedsCheck(e) {
			todo = append(todo, e)
		}
	}
	if recheck {
		fmt.Printf("Recheck: all %d entries.\n", len(todo))
	} else {
		fmt.Printf("Fill in: %d of %d entries were never checked (not_run).\n", len(todo), len(entries))
	}
	changed := 0
	for _, e := range todo {
		ver, words, err := v.Check(ctx, e)
		if err != nil {
			return fmt.Errorf("entry %s: %w", e.ID, err)
		}
		before, after := result(e.Verification), result(ver)
		line := after
		if recheck {
			line = before + " → " + after
			if before == after {
				line = "unchanged: " + after
			}
		}
		if before != after {
			changed++
		}
		if words != nil {
			line += fmt.Sprintf(", %d words", len(words.Words))
		}
		fmt.Printf("  %s  %-13s %-10s %-45s %s\n", e.ID, e.Type, deref(e.DocumentDate), e.File.FileName, line)
		if write {
			if err := v.Save(ctx, e.ID, ver, words); err != nil {
				return fmt.Errorf("entry %s: %w", e.ID, err)
			}
		}
	}
	switch {
	case len(todo) == 0:
	case write:
		fmt.Printf("Stored for %d entries (%d changed).\n", len(todo), changed)
	default:
		fmt.Printf("Dry run: nothing was written (%d would change). Run with -write to store these results.\n", changed)
	}
	fmt.Println()
	return nil
}

// result: "checked via ocr, 5 of 6 confirmed" or "unchecked/not_run".
func result(v core.Verification) string {
	if v.Status != core.VerificationChecked {
		return v.Status + "/" + v.Reason
	}
	return fmt.Sprintf("checked via %s, %d of %d confirmed", v.Source, v.Confirmed, v.Total)
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
func measure(ctx context.Context, p *ocr.Pipeline, entries []core.Entry, demoDir string, extra []demoFile) (report, error) {
	files, err := demoFiles(demoDir)
	if err != nil {
		return report{}, fmt.Errorf("demo folder: %w", err)
	}
	// Extra files go after the demo files of their document.
	for _, x := range extra {
		i := len(files)
		for j, f := range files {
			if f.Doc == x.Doc {
				i = j + 1
			}
		}
		files = slices.Insert(files, i, x)
	}
	forced := &ocr.Pipeline{Engine: p.Engine, PDF: p.PDF, Image: p.Image, ForceOCR: true}

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

		path := filepath.Join(demoDir, f.Path)
		if f.Extra {
			path = f.Path
		}
		data, err := os.ReadFile(path)
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
			if variant == "phone" {
				variant = "phone " + strings.ToUpper(strings.TrimPrefix(filepath.Ext(f.Name), ".")) // JPG, HEIC
			}
			kind := kindPhoto
			if f.variantName() == "phone" {
				kind = kindPhone
			}
			rep.Rows = append(rep.Rows, measureFile(ctx, p, f, data, mime, variant, kind, ref))
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
		if other.Extra {
			continue // only demo files name the document's entry
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
