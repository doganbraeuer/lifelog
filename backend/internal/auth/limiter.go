package auth

import (
	"net/netip"
	"sync"
	"time"

	"lifelog/backend/internal/core"
)

// Limits for failed attempts at login and password change (API.md 3.10).
//
//  1. Per username and IP address: 5 failed attempts within 15 minutes lock
//     this pair for 1 minute; every further lock doubles the time, at most
//     15 minutes. After one hour without a failed attempt, or after a
//     success, the next lock starts at 1 minute again. Someone guessing from
//     another IP address therefore cannot lock out an account.
//  2. Per IP address, whatever the username: 30 failed attempts within
//     15 minutes lock the address for 15 minutes. IPv6 addresses count per
//     /64 network, because one connection usually owns a whole /64.
const (
	FailureWindow   = 15 * time.Minute
	PairMaxFailures = 5
	PairFirstLock   = time.Minute
	PairMaxLock     = 15 * time.Minute
	PairLevelReset  = time.Hour
	IPMaxFailures   = 30
	IPLock          = 15 * time.Minute

	// inFlightRetry: answer for a request that comes in while earlier
	// attempts of the same pair or IP are still being checked and would
	// reach the limit. They finish within a second (one bcrypt comparison).
	inFlightRetry = time.Second

	// sweepEvery: every this many Begin calls, counters without a recent
	// failure are removed, so memory stays bounded.
	sweepEvery = 256
)

// Limiter counts failed attempts in memory. That is enough because there is
// only one server. A restart of the backend resets all counters (documented
// in backend/README.md).
//
// Use: Begin before checking a password, then exactly one of Fail (wrong
// password, unknown username, disabled account), Succeed (password correct)
// or Abort (the check could not be done, e.g. database error).
type Limiter struct {
	mu    sync.Mutex
	now   func() time.Time
	pairs map[pairKey]*counter
	ips   map[string]*counter
	ops   int
}

type pairKey struct {
	ip       string
	username string
}

type counter struct {
	failures    []time.Time // within FailureWindow, oldest first
	pending     int         // attempts started but not finished
	lockedUntil time.Time
	level       int // pairs only: locks in a row, decides the next lock time
	lastFailure time.Time
}

// NewLimiter creates an empty limiter. now is time.Now in the program and a
// fake clock in tests.
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{
		now:   now,
		pairs: map[pairKey]*counter{},
		ips:   map[string]*counter{},
	}
}

// Attempt is one password check in progress. Exactly one of Fail, Succeed
// or Abort ends it; further calls do nothing.
type Attempt struct {
	l        *Limiter
	ip       string
	pair     pairKey
	withPair bool
	done     bool
}

// Begin asks whether an attempt for username from ip may be checked. If
// not, the error is a *core.TooManyAttemptsError with the remaining wait.
//
// username "" counts only for the IP address. The caller uses that for a
// username that cannot exist (wrong format), so that random names do not
// fill the memory with pairs.
func (l *Limiter) Begin(ip, username string) (*Attempt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.ops++
	if l.ops%sweepEvery == 0 {
		l.sweep(now)
	}

	ipk := ipKey(ip)
	ic := getCounter(l.ips, ipk)
	ic.prune(now)
	if ic.lockedUntil.After(now) {
		return nil, &core.TooManyAttemptsError{RetryAfter: ic.lockedUntil.Sub(now)}
	}

	a := &Attempt{l: l, ip: ipk}
	var pc *counter
	if username != "" {
		a.pair, a.withPair = pairKey{ip: ipk, username: username}, true
		pc = getCounter(l.pairs, a.pair)
		pc.prune(now)
		if pc.lockedUntil.After(now) {
			return nil, &core.TooManyAttemptsError{RetryAfter: pc.lockedUntil.Sub(now)}
		}
		if pc.level > 0 && now.Sub(pc.lastFailure) >= PairLevelReset {
			pc.level = 0
		}
	}

	// Attempts still being checked count as if they had failed. Otherwise
	// 20 parallel requests would all pass before the first one is counted.
	if len(ic.failures)+ic.pending >= IPMaxFailures ||
		(pc != nil && len(pc.failures)+pc.pending >= PairMaxFailures) {
		return nil, &core.TooManyAttemptsError{RetryAfter: inFlightRetry}
	}
	ic.pending++
	if pc != nil {
		pc.pending++
	}
	return a, nil
}

// Fail counts a failed attempt. The fifth failure of a pair (thirtieth of an
// IP) starts the lock; this attempt itself is still answered normally, the
// next one gets 429.
func (a *Attempt) Fail() {
	a.finish(func(now time.Time, ic, pc *counter) {
		ic.failures = append(ic.failures, now)
		ic.lastFailure = now
		if len(ic.failures) >= IPMaxFailures {
			ic.lockedUntil = now.Add(IPLock)
			ic.failures = nil
		}
		if pc == nil {
			return
		}
		pc.failures = append(pc.failures, now)
		pc.lastFailure = now
		if len(pc.failures) >= PairMaxFailures {
			pc.lockedUntil = now.Add(lockDuration(pc.level))
			pc.level++
			pc.failures = nil
		}
	})
}

// Succeed ends the attempt after a correct password: the pair starts from
// scratch. The IP counter keeps its failures, otherwise someone with one
// valid account could reset it between guesses for other accounts.
func (a *Attempt) Succeed() {
	a.finish(func(_ time.Time, _, pc *counter) {
		if pc != nil {
			pc.failures = nil
			pc.level = 0
			pc.lockedUntil = time.Time{}
		}
	})
}

// Abort ends the attempt without counting it, e.g. after a database error.
func (a *Attempt) Abort() {
	a.finish(func(time.Time, *counter, *counter) {})
}

func (a *Attempt) finish(update func(now time.Time, ic, pc *counter)) {
	if a == nil || a.done {
		return
	}
	a.done = true
	l := a.l
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	ic := getCounter(l.ips, a.ip)
	ic.pending = max(ic.pending-1, 0)
	ic.prune(now)
	var pc *counter
	if a.withPair {
		pc = getCounter(l.pairs, a.pair)
		pc.pending = max(pc.pending-1, 0)
		pc.prune(now)
	}
	update(now, ic, pc)
}

// lockDuration: 1, 2, 4, 8 minutes, then always 15.
func lockDuration(level int) time.Duration {
	d := PairFirstLock
	for i := 0; i < level && d < PairMaxLock; i++ {
		d *= 2
	}
	return min(d, PairMaxLock)
}

// prune drops failures older than FailureWindow.
func (c *counter) prune(now time.Time) {
	i := 0
	for i < len(c.failures) && now.Sub(c.failures[i]) >= FailureWindow {
		i++
	}
	c.failures = c.failures[i:]
}

// idle: nothing left to remember about this counter.
func (c *counter) idle(now time.Time) bool {
	return c.pending == 0 && !c.lockedUntil.After(now) && len(c.failures) == 0 &&
		(c.level == 0 || now.Sub(c.lastFailure) >= PairLevelReset)
}

func (l *Limiter) sweep(now time.Time) {
	for k, c := range l.pairs {
		c.prune(now)
		if c.idle(now) {
			delete(l.pairs, k)
		}
	}
	for k, c := range l.ips {
		c.prune(now)
		if c.idle(now) {
			delete(l.ips, k)
		}
	}
}

func getCounter[K comparable](m map[K]*counter, k K) *counter {
	c := m[k]
	if c == nil {
		c = &counter{}
		m[k] = c
	}
	return c
}

// ipKey groups addresses: IPv4 as is, IPv6 by its /64 network. Anything
// that is not an IP address (should not happen) is used as it is.
func ipKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().String()
	}
	return addr.String()
}
