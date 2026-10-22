package preloader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"hn-reader/internal/fetcher"
	"hn-reader/internal/storage"
	"hn-reader/internal/summarizer"
)

func TestQueueAndPerUserLimit(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Summary: preloaded summary\nKey Points:\n- one"}}]}`)
	}))
	defer llm.Close()

	hn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":1,"type":"story","title":"Preload Title","url":""}`)
	}))
	defer hn.Close()

	repo, err := storage.NewSqlite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	f := fetcher.New(hn.URL)
	sm := summarizer.New(&summarizer.Config{
		BaseURL: llm.URL, APIKey: "test", Model: "test-model",
		Timeout: 5 * time.Second, DailyBudget: 100, MaxConcurrency: 1,
	})
	p := New(repo, f, sm, 1, Options{Workers: 1, QueueSize: 1, MaxPendingPerUser: 1})
	defer p.Close(context.Background())

	first, reason := p.SubmitForUser("u1", ArticleRequest{HNID: 1, Title: "x", Lang: "en"}, false)
	if !first {
		t.Fatalf("first submit rejected: %s", reason)
	}
	if ok, reason := p.SubmitForUser("u1", ArticleRequest{HNID: 2, Title: "x", Lang: "en"}, false); ok || reason != "user_limit" {
		t.Fatalf("expected user_limit, got ok=%v reason=%s", ok, reason)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		ready, err := p.IsCachedForUser(context.Background(), "u1", []int64{1}, "en")
		if err != nil {
			t.Fatal(err)
		}
		if len(ready) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("preload did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if ok, reason := p.SubmitForUser("u1", ArticleRequest{HNID: 2, Title: "x", Lang: "en"}, false); !ok {
		t.Fatalf("second submit after completion rejected: %s", reason)
	}
}

func TestQueueFull(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-block
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Summary: ok"}}]}`)
	}))
	defer llm.Close()

	hn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":1,"type":"story","title":"T","url":""}`)
	}))
	defer hn.Close()

	repo, err := storage.NewSqlite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	f := fetcher.New(hn.URL)
	sm := summarizer.New(&summarizer.Config{BaseURL: llm.URL, APIKey: "test", Model: "m", Timeout: 5 * time.Second, DailyBudget: 100})
	p := New(repo, f, sm, 1, Options{Workers: 1, QueueSize: 1, MaxPendingPerUser: 10})
	defer func() {
		close(block)
		p.Close(context.Background())
	}()

	if ok, reason := p.SubmitForUser("u", ArticleRequest{HNID: 1, Title: "T", Lang: "en"}, false); !ok {
		t.Fatalf("first submit: %s", reason)
	}
	<-started // worker is now blocked inside the LLM call

	if ok, reason := p.SubmitForUser("u", ArticleRequest{HNID: 2, Title: "T", Lang: "en"}, false); !ok {
		t.Fatalf("second submit should fill the queue: %s", reason)
	}
	ok, reason := p.SubmitForUser("u", ArticleRequest{HNID: 3, Title: "T", Lang: "en"}, false)
	if ok || reason != "queue_full" {
		t.Fatalf("expected queue_full, got ok=%v reason=%s", ok, reason)
	}
}
