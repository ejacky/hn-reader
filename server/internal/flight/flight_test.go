package flight

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoDeduplicates(t *testing.T) {
	var calls int32
	g := &Group{}
	var wg sync.WaitGroup
	results := make([]any, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := g.Do(context.Background(), "same", func() (any, error) {
				atomic.AddInt32(&calls, 1)
				time.Sleep(20 * time.Millisecond)
				return "value", nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			results[i] = v
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
	for i, v := range results {
		if v != "value" {
			t.Fatalf("result %d = %v", i, v)
		}
	}
}

func TestDoContextCancel(t *testing.T) {
	g := &Group{}
	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = g.Do(context.Background(), "k", func() (any, error) {
			close(started)
			<-release
			return nil, nil
		})
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := g.Do(ctx, "k", func() (any, error) { return nil, nil }); err == nil {
		t.Fatal("expected context error for waiter")
	}
	close(release)
}
