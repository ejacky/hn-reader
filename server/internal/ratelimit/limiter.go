package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a small fixed-window counter. It is intentionally in-process:
// HN Reader runs as a single cloud instance for now, and the limits are a
// cost/abuse guard rather than a billing primitive.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	limit   int
	window  time.Duration
	now     func() time.Time
	calls   int
}

type entry struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

// New creates a limiter allowing limit events per window.
func New(limit int, window time.Duration) *Limiter {
	return NewWithClock(limit, window, time.Now)
}

// NewWithClock is New with an injectable clock for tests.
func NewWithClock(limit int, window time.Duration, now func() time.Time) *Limiter {
	if limit < 0 {
		limit = 0
	}
	if window <= 0 {
		window = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &Limiter{
		entries: make(map[string]*entry),
		limit:   limit,
		window:  window,
		now:     now,
	}
}

// Allow reports whether key may perform one more event. When denied, the
// returned duration is the time until the current window resets.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l.limit == 0 {
		return true, 0
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	if l.calls%1024 == 0 {
		l.cleanupLocked(now)
	}

	e := l.entries[key]
	if e == nil || now.Sub(e.windowStart) >= l.window {
		l.entries[key] = &entry{windowStart: now, count: 1, lastSeen: now}
		return true, 0
	}

	e.lastSeen = now
	if e.count >= l.limit {
		retry := l.window - now.Sub(e.windowStart)
		if retry < time.Second {
			retry = time.Second
		}
		return false, retry
	}
	e.count++
	return true, 0
}

// Len returns the number of tracked keys (used by tests).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func (l *Limiter) cleanupLocked(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.lastSeen) > 2*l.window {
			delete(l.entries, k)
		}
	}
}
