package gemini

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/llm"
)

// The tests here work with real but short deadlines (milliseconds instead of
// 50 s), because they are exactly about how the deadlines interact.

// callTimes remembers per model how much time was left until the deadline
// when it was called.
type callTimes struct {
	mu        sync.Mutex
	remaining map[string]time.Duration
}

func (c *callTimes) record(ctx context.Context, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if deadline, ok := ctx.Deadline(); ok {
		c.remaining[model] = time.Until(deadline)
	}
}

// hangingMainModel: "main" never answers (waits until the deadline),
// "fallback" answers immediately with a valid extraction.
func hangingMainModel(ct *callTimes) generateFunc {
	return func(ctx context.Context, m string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		ct.record(ctx, m)
		if m == "main" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return response(validResponse, genai.FinishReasonStop), nil
	}
}

func shortGemini(generate generateFunc, fallback string, limit, reserve time.Duration) *Gemini {
	g := testGemini(&fakeNetwork{errs: map[string][]error{}}, fallback)
	g.generate = generate
	g.readTimeout = limit
	g.s.FallbackReserve = reserve
	return g
}

func TestReserve_HangingMainModelLeavesTimeForFallback(t *testing.T) {
	ct := &callTimes{remaining: map[string]time.Duration{}}
	g := shortGemini(hangingMainModel(ct), "fallback", 300*time.Millisecond, 120*time.Millisecond)

	start := time.Now()
	x, err := g.ReadDocument(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})
	duration := time.Since(start)

	if err != nil || x.Type != "laborbefund" {
		t.Fatalf("want success via the fallback model, got %+v, %v", x, err)
	}
	// The main model saw a deadline of ~180 ms (300 − 120), not 300 ms.
	if r := ct.remaining["main"]; r > 190*time.Millisecond || r < 150*time.Millisecond {
		t.Errorf("main model had %v until the deadline, want ~180ms", r)
	}
	// About the reserve remained for the fallback.
	if r := ct.remaining["fallback"]; r < 90*time.Millisecond {
		t.Errorf("fallback model only had %v, want ~120ms reserve", r)
	}
	if duration > 300*time.Millisecond {
		t.Errorf("total duration %v exceeds the limit", duration)
	}
}

func TestReserve_WithoutFallbackMainModelGetsTheWholeLimit(t *testing.T) {
	ct := &callTimes{remaining: map[string]time.Duration{}}
	g := shortGemini(hangingMainModel(ct), "", 300*time.Millisecond, 120*time.Millisecond)

	_, err := g.ReadDocument(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})

	if !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
	if r := ct.remaining["main"]; r < 280*time.Millisecond {
		t.Errorf("without a fallback model there must be no reserve: main model only had %v", r)
	}
	if _, called := ct.remaining["fallback"]; called {
		t.Error("fallback model called although none is configured")
	}
}

func TestReserve_ZeroMeansNoFallbackWhenHanging(t *testing.T) {
	// Exactly the problem of 2026-11-04: without a reserve the hanging main
	// model uses up everything and the fallback never gets its turn.
	ct := &callTimes{remaining: map[string]time.Duration{}}
	g := shortGemini(hangingMainModel(ct), "fallback", 200*time.Millisecond, 0)

	_, err := g.ReadDocument(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"})

	if !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
	if _, called := ct.remaining["fallback"]; called {
		t.Error("fallback called without remaining time")
	}
}

func TestReserve_MainModelErrorInLog(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	n := &fakeNetwork{errs: map[string][]error{"main": {e503, e503, e503}}}
	g := testGemini(n, "fallback")
	if _, err := g.ReadDocument(context.Background(), []byte("%PDF-"), "application/pdf", []string{"laborbefund"}); err != nil {
		t.Fatal(err)
	}

	var line string
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, "trying the fallback model once") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no log line about the fallback:\n%s", log.String())
	}
	for _, must := range []string{"model=main", "fallback=fallback", "HTTP 503", "remaining="} {
		if !strings.Contains(line, must) {
			t.Errorf("log line does not contain %q: %s", must, line)
		}
	}
	if strings.Contains(log.String(), "SECRET") {
		t.Error("key in the log")
	}
}

func TestNew_ReserveLimits(t *testing.T) {
	newWith := func(reserve time.Duration) error {
		_, err := New(context.Background(), Settings{
			APIKey: "x", Model: "a", FallbackModel: "b", EmbeddingModel: "e", FallbackReserve: reserve,
		})
		return err
	}
	// Limit: shorter limit (answering 45 s) minus 10 s minimum time = 35 s.
	for _, ok := range []time.Duration{0, 20 * time.Second, 35 * time.Second} {
		if err := newWith(ok); err != nil {
			t.Errorf("reserve %v: %v", ok, err)
		}
	}
	for _, invalid := range []time.Duration{-time.Second, 36 * time.Second, 50 * time.Second} {
		if err := newWith(invalid); err == nil {
			t.Errorf("reserve %v: want error", invalid)
		}
	}
}
