package flight

import (
	"context"
	"sync"
)

// Group deduplicates concurrent calls with the same key. It is the small
// in-process equivalent of golang.org/x/sync/singleflight, kept local to avoid
// adding a dependency for a single call site.
type Group struct {
	mu    sync.Mutex
	calls map[string]*call
}

type call struct {
	done chan struct{}
	val  any
	err  error
}

// Do runs fn once per key. Concurrent callers wait for the leader and receive
// the same result. A canceled waiter returns ctx.Err().
func (g *Group) Do(ctx context.Context, key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		select {
		case <-c.done:
			return c.val, c.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	c := &call{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()

	var val any
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = context.Canceled
			}
		}()
		val, err = fn()
	}()

	c.val, c.err = val, err
	close(c.done)

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return val, err
}

// InFlight returns the number of active keys (used by tests).
func (g *Group) InFlight() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}
