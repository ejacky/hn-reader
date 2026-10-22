package ratelimit

import (
	"testing"
	"time"
)

func TestAllowWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewWithClock(2, time.Hour, func() time.Time { return now })

	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("first request should be allowed")
	}
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("second request should be allowed")
	}
	if ok, retry := l.Allow("a"); ok {
		t.Fatal("third request should be denied")
	} else if retry < time.Minute {
		t.Fatalf("retry too small: %v", retry)
	}

	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key should be independent")
	}

	now = now.Add(2 * time.Hour)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("window should reset")
	}
}

func TestUnlimited(t *testing.T) {
	l := New(0, time.Minute)
	for i := 0; i < 100; i++ {
		if ok, _ := l.Allow("x"); !ok {
			t.Fatal("limit 0 should be unlimited")
		}
	}
}
