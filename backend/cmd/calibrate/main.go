// calibrate measures how well the threshold core.MinScore separates
// relevant from unrelated questions. It asks a fixed list of synthetic
// questions (questions.go), embeds each one as POST /questions does and prints
// the best score of the vector search per question. The language model
// (answerer) is NOT called; it only costs one embedding per question.
//
// Before, the seed data (go run ./cmd/seed, with chunks) and the synthetic
// demo data (lab reports, orthopedics, TBE, notes; uploaded) must be in the
// database. The program finds the demo entries by type and document date; if
// one is missing or there twice, it stops before the first embedding. It
// only reads from the database.
//
//	go run ./cmd/calibrate                    # with the threshold core.MinScore
//	go run ./cmd/calibrate -threshold 0.82    # compute another threshold
//
// It searches the module health across all chunks. The table names type
// and document date (MM-DD) of the best hit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/gemini"
	"lifelog/backend/internal/store"
)

func main() {
	threshold := flag.Float64("threshold", core.MinScore, "threshold to evaluate with (Atlas score 0 … 1)")
	flag.Parse()

	if err := run(*threshold); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(threshold float64) error {
	if threshold <= 0 || threshold > 1 {
		return fmt.Errorf("-threshold must be between 0 and 1")
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
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		return err
	}
	defer db.Disconnect(context.Background())

	status, found, err := db.VectorIndexStatus(ctx)
	if err != nil {
		return err
	}
	if !found || status != "READY" {
		return fmt.Errorf("vector index %q is not ready (status %q); start the server once or see backend/README.md, section vector index",
			store.VectorIndexName, status)
	}

	// Only the embedder is used; read and answer models stay untouched.
	g, err := gemini.New(ctx, gemini.Settings{
		APIKey:         c.GeminiKey,
		Model:          c.GeminiModel,
		EmbeddingModel: c.GeminiEmbeddingModel,
	})
	if err != nil {
		return err
	}

	// Read only: resolve targets (type + document date or seed ID) to IDs
	// before the first embedding costs quota.
	list, err := db.ListSummaries(ctx, "health")
	if err != nil {
		return err
	}
	qs := questions()
	ids, err := resolve(qs, list)
	if err != nil {
		return fmt.Errorf("entries for the questions are missing or not unique (seed data: go run ./cmd/seed; demo data: upload):\n%w", err)
	}
	names := map[string]string{}
	for _, e := range list {
		names[e.ID] = shortName(e)
	}

	fmt.Printf("%d questions, %d entries, embedding model %s, database %q, top %d\n\n",
		len(qs), len(list), c.GeminiEmbeddingModel, c.MongoDB, core.MaxHits)
	fmt.Println(" No  Group      Best    Hit                Right          Question")

	measurements := make([]measurement, 0, len(qs))
	for i, q := range qs {
		vector, err := g.EmbedQuestion(ctx, q.Text)
		if err != nil {
			if errors.Is(err, llm.ErrUnavailable) {
				return fmt.Errorf("question %d: embedding model unavailable, try again later: %w", i+1, err)
			}
			return fmt.Errorf("question %d: %w", i+1, err)
		}
		hits, err := db.SimilarChunks(ctx, vector, "health", core.MaxHits)
		if err != nil {
			return fmt.Errorf("question %d: %w", i+1, err)
		}

		expected := make([]string, len(q.Expected))
		for j, t := range q.Expected {
			expected[j] = ids[t]
		}
		m := measure(q, expected, hits)
		measurements = append(measurements, m)
		fmt.Printf("%3d  %-9s  %s   %-18s %-14s %s\n",
			i+1, q.Group, score(m.Best), entryName(names, m.BestEntry), mark(m), q.Text)
	}

	fmt.Println()
	fmt.Print(evaluate(measurements, threshold).Summary())
	return nil
}

// shortName for the table, e.g. "lab_report 07-22".
func shortName(e core.EntrySummary) string {
	if e.DocumentDate == nil || len(*e.DocumentDate) != 10 {
		return e.Type + " no date"
	}
	return e.Type + " " + (*e.DocumentDate)[5:]
}

func entryName(names map[string]string, id string) string {
	if id == "" {
		return "–"
	}
	if n, ok := names[id]; ok {
		return n
	}
	return "unknown"
}

// mark (only for relevant): ✓ if an expected entry is on top, otherwise ✗;
// after it, with several expected entries their scores, with one the score
// if it is not on top (– = not among the hits).
func mark(m measurement) string {
	if m.Question.Group != relevant {
		return ""
	}
	sign := "✗"
	if m.RightOnTop() {
		sign = "✓"
	}
	if len(m.Expected) == 1 && m.RightOnTop() {
		return sign
	}
	values := make([]string, len(m.Expected))
	for i, s := range m.Expected {
		if s == 0 {
			values[i] = "–"
		} else {
			values[i] = score(s)
		}
	}
	return sign + " " + strings.Join(values, "/")
}
