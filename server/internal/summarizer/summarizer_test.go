package summarizer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSummarizeAndDailyBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Summary: A test summary.\nKey Points:\n- one\n- two"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{
		BaseURL:        srv.URL,
		APIKey:         "test",
		Model:          "test-model",
		DailyBudget:    1,
		MaxConcurrency: 1,
		Timeout:        5 * time.Second,
	})

	summary, points, err := s.SummarizeArticle(context.Background(), "Title", "Content", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "A test summary." {
		t.Fatalf("summary = %q", summary)
	}
	if len(points) != 2 {
		t.Fatalf("points = %v", points)
	}

	if _, _, err := s.SummarizeArticle(context.Background(), "Title", "Content", "en"); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected budget error, got %v", err)
	}
}

func TestSummarizeRetriesOnce(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Summary: ok"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestSummarizeAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "bad", Model: "test-model", Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); !errors.Is(err, ErrAuth) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestMaxTokensSent(t *testing.T) {
	var got int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = body.MaxTokens
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Summary: ok"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 123, Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); err != nil {
		t.Fatal(err)
	}
	if got != 123 {
		t.Fatalf("max_tokens = %d", got)
	}
}

func TestParseResponseVariants(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		summary string
		points  int
	}{
		{
			name:    "plain",
			in:      "Summary: hello world\nKey Points:\n- one\n- two",
			summary: "hello world",
			points:  2,
		},
		{
			name:    "markdown",
			in:      "**Summary:** hello **world**\n**Key Points:**\n1. one\n2. two",
			summary: "hello world",
			points:  2,
		},
		{
			name:    "chinese",
			in:      "摘要：你好世界\n要点：\n- 一\n- 二",
			summary: "你好世界",
			points:  2,
		},
		{
			name:    "json",
			in:      `{"summary":"json summary","key_points":["a","b"]}`,
			summary: "json summary",
			points:  2,
		},
		{
			name:    "no labels",
			in:      "Just a paragraph with no labels.",
			summary: "Just a paragraph with no labels.",
			points:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, points := parseResponse(tc.in)
			if summary != tc.summary {
				t.Fatalf("summary = %q, want %q", summary, tc.summary)
			}
			if len(points) != tc.points {
				t.Fatalf("points = %v", points)
			}
		})
	}
}

