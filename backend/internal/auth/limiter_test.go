package auth

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter() (*Limiter, *fakeClock) {
	c := &fakeClock{t: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)}
	return NewLimiter(c.now), c
}

// fail runs n failed attempts and stops the test if one is refused.
func fail(t *testing.T, l *Limiter, ip, user string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		a, err := l.Begin(ip, user)
		if err != nil {
			t.Fatalf("attempt %d refused: %v", i+1, err)
		}
		a.Fail()
	}
}

// blocked returns the wait if the next attempt is refused, else 0.
func blocked(t *testing.T, l *Limiter, ip, user string) time.Duration {
	t.Helper()
	a, err := l.Begin(ip, user)
	if err == nil {
		a.Abort()
		return 0
	}
	var tm *core.TooManyAttemptsError
	if !errors.As(err, &tm) {
		t.Fatalf("err = %v, want TooManyAttemptsError", err)
	}
	return tm.RetryAfter
}

func TestLimiter_SixthAttemptLocked(t *testing.T) {
	l, _ := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 5)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != time.Minute {
		t.Errorf("wait after 5 failures = %v, want 1 min", wait)
	}
}

func TestLimiter_FourFailuresAreFine(t *testing.T) {
	l, _ := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 4)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != 0 {
		t.Errorf("locked after 4 failures (%v)", wait)
	}
}

// Dogan's point: someone guessing tester01 from elsewhere must not lock out
// the real tester.
func TestLimiter_OtherIPNotLocked(t *testing.T) {
	l, _ := newTestLimiter()
	fail(t, l, "198.51.100.66", "tester01", 5)
	if blocked(t, l, "198.51.100.66", "tester01") == 0 {
		t.Fatal("attacker's pair should be locked")
	}
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != 0 {
		t.Errorf("tester's own IP is locked (%v)", wait)
	}
	if wait := blocked(t, l, "198.51.100.66", "tester02"); wait != 0 {
		t.Errorf("another name from the attacker's IP is locked (%v)", wait)
	}
}

func TestLimiter_LockDoublesUpTo15Minutes(t *testing.T) {
	l, clock := newTestLimiter()
	want := []time.Duration{1, 2, 4, 8, 15, 15}
	for i, w := range want {
		fail(t, l, "203.0.113.7", "tester01", 5)
		wait := blocked(t, l, "203.0.113.7", "tester01")
		if wait != w*time.Minute {
			t.Errorf("lock %d: wait = %v, want %v min", i+1, wait, int(w))
		}
		clock.advance(wait)
		if blocked(t, l, "203.0.113.7", "tester01") != 0 {
			t.Fatalf("lock %d still active after its wait", i+1)
		}
	}
}

func TestLimiter_LockCountsDownAndRetryAfterShrinks(t *testing.T) {
	l, clock := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 5)
	clock.advance(20 * time.Second)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != 40*time.Second {
		t.Errorf("wait = %v, want 40 s", wait)
	}
}

func TestLimiter_LevelResetsAfterOneHourWithoutFailure(t *testing.T) {
	l, clock := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 5) // 1 min
	clock.advance(time.Minute)
	fail(t, l, "203.0.113.7", "tester01", 5) // 2 min
	clock.advance(time.Hour)
	fail(t, l, "203.0.113.7", "tester01", 5)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != time.Minute {
		t.Errorf("after an hour without failures: wait = %v, want 1 min", wait)
	}
}

func TestLimiter_SuccessResetsPair(t *testing.T) {
	l, clock := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 5) // level 1
	clock.advance(time.Minute)
	fail(t, l, "203.0.113.7", "tester01", 4)
	a, err := l.Begin("203.0.113.7", "tester01")
	if err != nil {
		t.Fatal(err)
	}
	a.Succeed()
	// Failures and level are gone: 4 more failures do not lock, the fifth
	// locks for 1 minute again (not 2).
	fail(t, l, "203.0.113.7", "tester01", 4)
	if blocked(t, l, "203.0.113.7", "tester01") != 0 {
		t.Fatal("locked although the success reset the counter")
	}
	fail(t, l, "203.0.113.7", "tester01", 1)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != time.Minute {
		t.Errorf("wait = %v, want 1 min", wait)
	}
}

func TestLimiter_FailuresOutsideWindowDoNotCount(t *testing.T) {
	l, clock := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 4)
	clock.advance(FailureWindow)
	fail(t, l, "203.0.113.7", "tester01", 4)
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != 0 {
		t.Errorf("old failures still counted (%v)", wait)
	}
}

func TestLimiter_IPLimitAcrossNames(t *testing.T) {
	l, _ := newTestLimiter()
	for i := 0; i < 30; i++ { // 30 different names, 1 failure each
		fail(t, l, "203.0.113.7", fmt.Sprintf("name%02d", i), 1)
	}
	if wait := blocked(t, l, "203.0.113.7", "tester01"); wait != IPLock {
		t.Errorf("wait = %v, want %v", wait, IPLock)
	}
	if wait := blocked(t, l, "198.51.100.1", "tester01"); wait != 0 {
		t.Errorf("another IP is locked (%v)", wait)
	}
}

func TestLimiter_SuccessDoesNotResetIP(t *testing.T) {
	l, _ := newTestLimiter()
	for i := 0; i < 29; i++ {
		fail(t, l, "203.0.113.7", fmt.Sprintf("name%02d", i), 1)
	}
	a, _ := l.Begin("203.0.113.7", "own")
	a.Succeed()
	fail(t, l, "203.0.113.7", "name99", 1)
	if blocked(t, l, "203.0.113.7", "tester01") == 0 {
		t.Error("a success must not reset the IP counter")
	}
}

func TestLimiter_EmptyUsernameCountsOnlyIP(t *testing.T) {
	l, _ := newTestLimiter()
	fail(t, l, "203.0.113.7", "", 10)
	if len(l.pairs) != 0 {
		t.Errorf("%d pairs stored for invalid usernames", len(l.pairs))
	}
	if wait := blocked(t, l, "203.0.113.7", ""); wait != 0 {
		t.Errorf("locked after 10 IP failures (%v)", wait)
	}
}

func TestLimiter_IPv6CountsPer64(t *testing.T) {
	l, _ := newTestLimiter()
	for i := 0; i < 30; i++ { // a new address in the same /64 for every attempt
		fail(t, l, fmt.Sprintf("2001:db8:1:2::%x", i+1), fmt.Sprintf("name%02d", i), 1)
	}
	if blocked(t, l, "2001:db8:1:2::ffff", "tester01") == 0 {
		t.Error("addresses of one /64 must share a counter")
	}
	if blocked(t, l, "2001:db8:1:3::1", "tester01") != 0 {
		t.Error("another /64 must not be locked")
	}
}

// Parallel requests: while 4 failures are counted and a fifth attempt is
// still being checked, another attempt must not start.
func TestLimiter_PendingAttemptsCount(t *testing.T) {
	l, _ := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 4)
	running, err := l.Begin("203.0.113.7", "tester01")
	if err != nil {
		t.Fatal(err)
	}
	if blocked(t, l, "203.0.113.7", "tester01") == 0 {
		t.Error("a second attempt passed while the fifth was running")
	}
	running.Abort()
	if blocked(t, l, "203.0.113.7", "tester01") != 0 {
		t.Error("an aborted attempt must not count")
	}
}

func TestLimiter_FinishTwiceCountsOnce(t *testing.T) {
	l, _ := newTestLimiter()
	a, _ := l.Begin("203.0.113.7", "tester01")
	a.Fail()
	a.Fail()
	a.Succeed()
	if n := len(l.pairs[pairKey{"203.0.113.7", "tester01"}].failures); n != 1 {
		t.Errorf("failures = %d, want 1", n)
	}
}

func TestLimiter_SweepRemovesIdleCounters(t *testing.T) {
	l, clock := newTestLimiter()
	fail(t, l, "203.0.113.7", "tester01", 1)
	clock.advance(PairLevelReset)
	l.mu.Lock()
	l.sweep(clock.now())
	l.mu.Unlock()
	if len(l.pairs) != 0 || len(l.ips) != 0 {
		t.Errorf("pairs = %d, ips = %d after sweep, want 0", len(l.pairs), len(l.ips))
	}
}

func TestLockDuration(t *testing.T) {
	want := []time.Duration{1, 2, 4, 8, 15, 15, 15}
	for level, w := range want {
		if got := lockDuration(level); got != w*time.Minute {
			t.Errorf("lockDuration(%d) = %v, want %v min", level, got, int(w))
		}
	}
	if lockDuration(1000) != PairMaxLock {
		t.Error("large levels must stay at the maximum")
	}
}
