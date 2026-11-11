package gemini

// Live tests against the real Gemini API. They only run with
//
//	LIFELOG_LIVE=1 go test ./internal/llm/gemini/ -run Live -v
//
// They cost requests from the quota and send only the synthetic document
// from backend/testdata/ to Google. The API key comes from the .env in the
// repository root and is never printed.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

func liveGemini(t *testing.T) *Gemini {
	t.Helper()
	if os.Getenv("LIFELOG_LIVE") != "1" {
		t.Skip("live test skipped (set LIFELOG_LIVE=1)")
	}
	if err := config.FindDotenv(); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(context.Background(), Settings{
		APIKey:          c.GeminiKey,
		Model:           c.GeminiModel,
		FallbackModel:   c.GeminiFallbackModel,
		FallbackReserve: c.GeminiFallbackReserve,
		EmbeddingModel:  c.GeminiEmbeddingModel,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("models: %s (fallback %q, reserve %v) / %s", c.GeminiModel, g.s.FallbackModel, c.GeminiFallbackReserve, c.GeminiEmbeddingModel)
	return g
}

func testFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLive_ReadDocument(t *testing.T) {
	g := liveGemini(t)
	types := core.AllowedTypes("health")

	for _, name := range []string{"lab_report.pdf", "lab_report.png"} {
		t.Run(name, func(t *testing.T) {
			data := testFile(t, name)
			mime, err := core.CheckFile(data)
			if err != nil {
				t.Fatal(err)
			}

			start := time.Now()
			x, err := g.ReadDocument(context.Background(), data, mime, types)
			duration := time.Since(start)
			if err != nil {
				t.Fatalf("after %v: %v", duration.Round(time.Millisecond), err)
			}
			t.Logf("duration %v", duration.Round(time.Millisecond))

			entry, dropped, err := core.CheckExtraction(x, "health")
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if len(dropped) > 0 {
				t.Logf("dropped: %v", dropped)
			}

			if entry.Type != "lab_report" {
				t.Errorf("type = %q", entry.Type)
			}
			if entry.DocumentDate == nil || *entry.DocumentDate != "2026-10-14" {
				t.Errorf("document date = %v (confused with the date of birth?)", deref(entry.DocumentDate))
			}
			if entry.Issuer == nil || !strings.Contains(*entry.Issuer, "Labor Muster") {
				t.Errorf("issuer = %v", deref(entry.Issuer))
			}
			if !strings.Contains(entry.FullText, "Hämoglobin") {
				t.Error("full text does not contain Hämoglobin (umlaut lost?)")
			}
			// The prompt is English, the document German: the full text must
			// stay German, otherwise chunks and vectors change.
			for _, word := range []string{"LABORBEFUND", "Befunddatum", "Leukozyten", "Befundart: Blutbild klein"} {
				if !strings.Contains(entry.FullText, word) {
					t.Errorf("full text does not contain %q (translated?)", word)
				}
			}
			for _, word := range []string{"Hemoglobin", "Leukocytes", "LAB REPORT"} {
				if strings.Contains(entry.FullText, word) {
					t.Errorf("full text contains %q: translated", word)
				}
			}

			want := map[string][2]string{ // name → value, unit
				"Hämoglobin": {"14,2", "g/dl"},
				"CRP":        {"<0,5", "mg/dl"},
			}
			for _, f := range entry.Facts {
				t.Logf("  %s: %s = %q %s", f.ID, f.Name, f.Value, deref(f.Unit))
				if w, ok := want[f.Name]; ok {
					if f.Value != w[0] || deref(f.Unit) != w[1] {
						t.Errorf("%s = %q %s, want %q %s", f.Name, f.Value, deref(f.Unit), w[0], w[1])
					}
					delete(want, f.Name)
				}
			}
			for name := range want {
				t.Errorf("fact %s missing", name)
			}
		})
	}
}

func TestLive_ReadText(t *testing.T) {
	g := liveGemini(t)
	text := string(testFile(t, "lab_report.txt"))

	start := time.Now()
	x, err := g.ReadText(context.Background(), text, core.AllowedTypes("health"))
	duration := time.Since(start).Round(time.Millisecond)
	if err != nil {
		t.Fatalf("after %v: %v", duration, err)
	}
	t.Logf("duration %v", duration)

	// As in the service: the full text is the entered text.
	x.FullText = text
	entry, dropped, err := core.CheckExtraction(x, "health")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(dropped) > 0 {
		t.Logf("dropped: %v", dropped)
	}
	if entry.Type != "lab_report" {
		t.Errorf("type = %q", entry.Type)
	}
	if deref(entry.DocumentDate) != "2026-10-14" {
		t.Errorf("document date = %v", deref(entry.DocumentDate))
	}
	if !strings.Contains(deref(entry.Issuer), "Labor Muster") {
		t.Errorf("issuer = %v", deref(entry.Issuer))
	}
	hb := false
	for _, f := range entry.Facts {
		t.Logf("  %s: %s = %q %s", f.ID, f.Name, f.Value, deref(f.Unit))
		hb = hb || (f.Name == "Hämoglobin" && f.Value == "14,2" && deref(f.Unit) == "g/dl")
	}
	if !hb {
		t.Error("Hämoglobin 14,2 g/dl missing")
	}
}

// TestLive_Answer: the answer prompt is English, but answers must be German,
// with references, and the fixed sentence must come verbatim.
func TestLive_Answer(t *testing.T) {
	g := liveGemini(t)
	p := func(s string) *string { return &s }
	excerpts := []core.Source{{Number: 1, EntryID: "a", Module: "health", Type: "lab_report",
		DocumentDate: p("2026-10-14"), Issuer: p("Labor Muster GmbH, Salzburg"),
		Excerpt: "Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)"}}

	text, err := g.Answer(context.Background(), "Welchen Hämoglobinwert hatte ich im Oktober?", excerpts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("answer: %s", text)
	if !strings.Contains(text, "14,2") || !strings.Contains(text, "[1]") {
		t.Error("answer must name 14,2 with reference [1]")
	}
	if !strings.Contains(text, "Hämoglobin") {
		t.Error("answer is not German (Hämoglobin missing)")
	}
	// Informal like the app, and the type named as in the app.
	for _, word := range []string{"Ihr ", "Ihre ", "Laborbericht"} {
		if strings.Contains(text, word) {
			t.Errorf("answer contains %q (want „du“ and „Laborbefund“)", word)
		}
	}
	for _, word := range []string{" the ", "The ", " was ", "hemoglobin"} {
		if strings.Contains(text, word) {
			t.Errorf("answer contains %q: English", word)
		}
	}

	text, err = g.Answer(context.Background(), "Welche Allergien habe ich?", excerpts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("answer: %s", text)
	if strings.TrimSpace(text) != core.NotFoundAnswer {
		t.Errorf("want the fixed sentence verbatim, got %q", text)
	}
}

func TestLive_BlankPageIsUnreadable(t *testing.T) {
	g := liveGemini(t)

	// A plain white image, generated in memory.
	img := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for x := range 400 {
		for y := range 400 {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	x, err := g.ReadDocument(context.Background(), buf.Bytes(), core.MimePNG, core.AllowedTypes("health"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.CheckExtraction(x, "health"); !errors.Is(err, core.ErrUnreadable) {
		t.Errorf("blank page: want ErrUnreadable, got %v (readable=%v)", err, x.Readable)
	}
}

func TestLive_Embed(t *testing.T) {
	g := liveGemini(t)
	ctx := context.Background()

	// Synthetic German document content and questions, as users write them.
	chunks := []string{
		"Laborbefund vom 14.10.2026\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l",
		"Arztbrief vom 02.09.2026\nVorstellung zur Routinekontrolle. Blutdruck 128/82 mmHg.",
		"Rezept vom 14.10.2026\nBeispielin 100 mg, 1-0-0, 30 Stück",
	}
	vectors, err := g.EmbedChunks(ctx, chunks)
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != len(chunks) {
		t.Fatalf("%d vectors for %d chunks", len(vectors), len(chunks))
	}
	for i, v := range vectors {
		if len(v) != llm.EmbeddingDimensions {
			t.Errorf("vector %d has %d dimensions", i, len(v))
		}
		t.Logf("vector %d: length (norm) %.4f", i, norm(v))
	}
	if slices.Equal(vectors[0], vectors[1]) {
		t.Fatal("different chunks give the same vector (merged instead of separate?)")
	}

	questions := []struct {
		question string
		want     int // index of the best-matching chunk, -1 = none fits
	}{
		{"Welchen Hämoglobinwert hatte ich im Oktober?", 0},
		{"Wie hoch war mein Blutdruck?", 1},
		{"Welches Medikament wurde mir verschrieben?", 2},
		{"Wie wird morgen das Wetter in Salzburg?", -1},
	}
	for _, q := range questions {
		qv, err := g.EmbedQuestion(ctx, q.question)
		if err != nil {
			t.Fatal(err)
		}
		best, bestValue := -1, -2.0
		var values []string
		for i, v := range vectors {
			c := cosine(qv, v)
			values = append(values, formatScores(c))
			if c > bestValue {
				best, bestValue = i, c
			}
		}
		t.Logf("%-46q → %s", q.question, strings.Join(values, " | "))
		if q.want >= 0 && best != q.want {
			t.Errorf("%q: best chunk %d, want %d", q.question, best, q.want)
		}
	}
}

// formatScores shows the cosine similarity and the score that Atlas Vector
// Search reports with similarity "cosine": (1 + cos) / 2.
func formatScores(c float64) string {
	return fmt.Sprintf("cos %.3f / Atlas %.3f", c, (1+c)/2)
}

func cosine(a, b []float32) float64 {
	var s, na, nb float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return s / (math.Sqrt(na) * math.Sqrt(nb))
}

func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}
