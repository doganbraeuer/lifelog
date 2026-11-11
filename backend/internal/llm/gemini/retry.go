package gemini

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/genai"
)

// retrier runs a call and repeats it if Gemini is overloaded (503) or
// reports a limit (429): at most `retries` more times, with exponentially
// growing waits (base, 2·base, …).
//
// Everything stays within the deadline of the context (the total limit, e.g.
// 50 s for reading): if the remaining time is no longer enough for waiting
// plus a sensible new attempt, the retrier gives up and returns the last
// error.
//
// Other errors (400, 404, 500, 504, network) are not retried.
type retrier struct {
	retries      int           // further attempts after the first
	base         time.Duration // first wait
	minRemaining time.Duration // this much time must remain after waiting
	now          func() time.Time
	sleep        func(context.Context, time.Duration) error
	label        string // for the log, e.g. "read gemini-3.8-flash"
}

func defaultRetrier(label string) retrier {
	return retrier{
		retries:      2,
		base:         time.Second,
		minRemaining: 5 * time.Second,
		now:          time.Now,
		sleep:        sleepWithContext,
		label:        label,
	}
}

func (r retrier) run(ctx context.Context, call func(context.Context) error) error {
	wait := r.base
	for attempt := 1; ; attempt++ {
		err := call(ctx)
		if err == nil || !retryable(err) || attempt > r.retries {
			return err
		}
		if !r.hasTime(ctx, wait) {
			slog.Warn("Gemini overloaded, no time left for another attempt",
				"call", r.label, "attempt", attempt)
			return err
		}
		slog.Warn("Gemini overloaded, retrying",
			"call", r.label, "attempt", attempt, "wait", wait, "http", httpCode(err))
		if serr := r.sleep(ctx, wait); serr != nil {
			return serr // deadline passed or request canceled
		}
		wait *= 2
	}
}

// hasTime: does minRemaining still remain until the deadline after waiting?
func (r retrier) hasTime(ctx context.Context, wait time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return true
	}
	return deadline.Sub(r.now()) >= wait+r.minRemaining
}

// retryable: only 429 (limit) and 503 (overloaded).
func retryable(err error) bool {
	code := httpCode(err)
	return code == 429 || code == 503
}

// httpCode returns the HTTP status of a Gemini error, or 0.
func httpCode(err error) int {
	var api genai.APIError
	if asError(err, &api) {
		return api.Code
	}
	return 0
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
