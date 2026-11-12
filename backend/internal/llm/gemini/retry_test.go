package gemini

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/llm"
)

// clock is a virtual clock: sleep only moves it forward and records the
// waits. This way tests take no real seconds.
type clock struct {
	now    time.Time
	waited []time.Duration
	err    error // if set, sleep fails with it
}

func (c *clock) retrier() retrier {
	r := defaultRetrier("test")
	r.now = func() time.Time { return c.now }
	r.sleep = func(_ context.Context, d time.Duration) error {
		if c.err != nil {
			return c.err
		}
		c.waited = append(c.waited, d)
		c.now = c.now.Add(d)
		return nil
	}
	return r
}

// script returns the errors in order (nil = success) and counts calls.
type script struct {
	errs  []error
	calls int
}

func (s *script) call(context.Context) error {
	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}
	return nil
}

var (
	e429 = genai.APIError{Code: 429, Message: "Quota exceeded"}
	e503 = genai.APIError{Code: 503, Message: "high demand"}
)

// withDeadline returns a context whose deadline lies `remaining` after c.now.
func withDeadline(t *testing.T, c *clock, remaining time.Duration) context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), c.now.Add(remaining))
	t.Cleanup(cancel)
	return ctx
}

func TestRetrier(t *testing.T) {
	cases := []struct {
		name      string
		errs      []error
		remaining time.Duration // 0 = no deadline
		calls     int
		waited    []time.Duration
		result    error // nil = success
	}{
		{"immediate success", nil, 50 * time.Second, 1, nil, nil},
		{"503, 503, then success", []error{e503, e503}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, nil},
		{"429 three times: give up after two retries", []error{e429, e429, e429}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, e429},
		{"mixed 429 and 503", []error{e429, e503}, 50 * time.Second, 3,
			[]time.Duration{time.Second, 2 * time.Second}, nil},
		{"500 is not retried", []error{genai.APIError{Code: 500}}, 50 * time.Second, 1, nil, genai.APIError{Code: 500}},
		{"504 is not retried", []error{genai.APIError{Code: 504}}, 50 * time.Second, 1, nil, genai.APIError{Code: 504}},
		{"400 is not retried", []error{genai.APIError{Code: 400}}, 50 * time.Second, 1, nil, genai.APIError{Code: 400}},
		{"network error is not retried", []error{&net.DNSError{Err: "x"}}, 50 * time.Second, 1, nil, &net.DNSError{Err: "x"}},
		{"retried without deadline", []error{e503}, 0, 2, []time.Duration{time.Second}, nil},
		// Total limit: 5 s must remain after waiting.
		{"deadline 5.5 s: 1 s wait + 5 s remaining does not fit", []error{e503, e503}, 5500 * time.Millisecond, 1, nil, e503},
		{"deadline 7.5 s: first retry yes, second (2 s + 5 s with 6.5 s left) no",
			[]error{e503, e503, e503}, 7500 * time.Millisecond, 2, []time.Duration{time.Second}, e503},
		{"deadline exactly 6 s: 1 s + 5 s just fits", []error{e503}, 6 * time.Second, 2, []time.Duration{time.Second}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl := &clock{now: time.Now()}
			ctx := context.Background()
			if c.remaining > 0 {
				ctx = withDeadline(t, cl, c.remaining)
			}
			s := &script{errs: c.errs}

			err := cl.retrier().run(ctx, s.call)

			if s.calls != c.calls {
				t.Errorf("calls = %d, want %d", s.calls, c.calls)
			}
			if !slices.Equal(cl.waited, c.waited) {
				t.Errorf("waits = %v, want %v", cl.waited, c.waited)
			}
			if c.result == nil && err != nil {
				t.Errorf("want success, got %v", err)
			}
			if c.result != nil && (err == nil || err.Error() != c.result.Error()) {
				t.Errorf("error = %v, want %v", err, c.result)
			}
		})
	}
}

func TestRetrier_CanceledWhileWaiting(t *testing.T) {
	cl := &clock{now: time.Now(), err: context.Canceled}
	s := &script{errs: []error{e503, e503}}

	err := cl.retrier().run(context.Background(), s.call)

	if !errors.Is(err, context.Canceled) || s.calls != 1 {
		t.Errorf("err = %v, calls = %d; want Canceled after 1 call", err, s.calls)
	}
}

func TestSleepWithContext(t *testing.T) {
	if err := sleepWithContext(context.Background(), time.Millisecond); err != nil {
		t.Errorf("normal sleep: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepWithContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Error("sleep waited despite cancellation")
	}
}

// --- In Gemini: retries and fallback model working together ---

// fakeNetwork replaces the two SDK calls. Per model name a list of errors
// (nil = success) that are returned in order.
type fakeNetwork struct {
	errs  map[string][]error
	calls []string // model names in call order
}

func (n *fakeNetwork) next(model string) error {
	n.calls = append(n.calls, model)
	list := n.errs[model]
	if len(list) == 0 {
		return nil
	}
	n.errs[model] = list[1:]
	return list[0]
}

func (n *fakeNetwork) generate(_ context.Context, m string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if err := n.next(m); err != nil {
		return nil, err
	}
	return response(validResponse, genai.FinishReasonStop), nil
}

func (n *fakeNetwork) embed(_ context.Context, m string, contents []*genai.Content, _ *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	if err := n.next(m); err != nil {
		return nil, err
	}
	resp := &genai.EmbedContentResponse{}
	for range contents {
		resp.Embeddings = append(resp.Embeddings, &genai.ContentEmbedding{Values: make([]float32, llm.EmbeddingDimensions)})
	}
	return resp, nil
}

func testGemini(n *fakeNetwork, fallback string) *Gemini {
	return &Gemini{
		s: Settings{
			APIKey: "SECRET", Model: "main", FallbackModel: fallback, EmbeddingModel: "embed",
			FallbackReserve: 20 * time.Second,
		},
		generate:      n.generate,
		embedAPI:      n.embed,
		readTimeout:   readTimeout,
		answerTimeout: answerTimeout,
		newRetrier: func(label string) retrier {
			r := defaultRetrier(label)
			r.sleep = func(context.Context, time.Duration) error { return nil } // do not really wait
			return r
		},
	}
}

func TestReadDocument_RetriesAndFallback(t *testing.T) {
	cases := []struct {
		name        string
		errs        map[string][]error
		fallback    string
		calls       []string
		unavailable bool // expected result; otherwise success or another error
		success     bool
	}{
		{"main model works after one retry",
			map[string][]error{"main": {e503}}, "fallback",
			[]string{"main", "main"}, false, true},
		{"main model stays overloaded, fallback works",
			map[string][]error{"main": {e503, e429, e503}}, "fallback",
			[]string{"main", "main", "main", "fallback"}, false, true},
		{"fallback is tried only once, not retried",
			map[string][]error{"main": {e503, e503, e503}, "fallback": {e503, e503}}, "fallback",
			[]string{"main", "main", "main", "fallback"}, true, false},
		{"without fallback model: unavailable after three attempts",
			map[string][]error{"main": {e503, e503, e503}}, "",
			[]string{"main", "main", "main"}, true, false},
		{"504 is not retried, but the fallback applies",
			map[string][]error{"main": {genai.APIError{Code: 504}}}, "fallback",
			[]string{"main", "fallback"}, false, true},
		{"400 is no availability problem: no fallback",
			map[string][]error{"main": {genai.APIError{Code: 400}}}, "fallback",
			[]string{"main"}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := &fakeNetwork{errs: c.errs}
			g := testGemini(n, c.fallback)

			x, err := g.ReadDocument(context.Background(), []byte("%PDF-"), "application/pdf", []string{"lab_report"})

			if !slices.Equal(n.calls, c.calls) {
				t.Errorf("calls = %v, want %v", n.calls, c.calls)
			}
			if got := errors.Is(err, llm.ErrUnavailable); got != c.unavailable {
				t.Errorf("ErrUnavailable = %v, want %v (%v)", got, c.unavailable, err)
			}
			if c.success && (err != nil || x.Type != "lab_report") {
				t.Errorf("want success, got %+v, %v", x, err)
			}
			if !c.success && err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestEmbed_RetriesWithoutFallback(t *testing.T) {
	n := &fakeNetwork{errs: map[string][]error{"embed": {e429}}}
	g := testGemini(n, "fallback")
	v, err := g.EmbedChunks(context.Background(), []string{"a", "b"})
	if err != nil || len(v) != 2 {
		t.Fatalf("v = %d, err = %v", len(v), err)
	}
	if !slices.Equal(n.calls, []string{"embed", "embed"}) {
		t.Errorf("calls = %v", n.calls)
	}

	// If the embedding model stays overloaded, there is no fallback.
	n = &fakeNetwork{errs: map[string][]error{"embed": {e503, e503, e503}}}
	g = testGemini(n, "fallback")
	if _, err := g.EmbedQuestion(context.Background(), "x"); !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
	if !slices.Equal(n.calls, []string{"embed", "embed", "embed"}) {
		t.Errorf("calls = %v (no other model allowed)", n.calls)
	}
}

func TestEmbed_EmptyListWithoutCall(t *testing.T) {
	n := &fakeNetwork{errs: map[string][]error{}}
	v, err := testGemini(n, "").EmbedChunks(context.Background(), nil)
	if err != nil || v == nil || len(v) != 0 || len(n.calls) != 0 {
		t.Errorf("v = %#v, err = %v, calls = %v", v, err, n.calls)
	}
}

func TestNew_FallbackEqualToMainIsIgnored(t *testing.T) {
	g, err := New(context.Background(), Settings{
		APIKey: "x", Model: "gemini-3.8-flash", FallbackModel: "gemini-3.8-flash", EmbeddingModel: "e",
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.s.FallbackModel != "" {
		t.Errorf("fallback = %q, want empty", g.s.FallbackModel)
	}
}
