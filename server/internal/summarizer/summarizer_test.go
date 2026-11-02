package summarizer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestSummarizeRetriesEmptyContentWithBiggerBudget(t *testing.T) {
	var calls int32
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		budgets = append(budgets, body.MaxTokens)
		mu.Unlock()
		if atomic.AddInt32(&calls, 1) == 1 {
			// Reasoning model burned the whole budget: content empty, reason "length".
			_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"thinking..."}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":"Summary: retried ok"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, Timeout: 5 * time.Second})
	summary, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "retried ok" {
		t.Fatalf("summary = %q", summary)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(budgets) != 2 {
		t.Fatalf("expected 2 calls, got %v", budgets)
	}
	if budgets[0] != 600 || budgets[1] != 1200 {
		t.Fatalf("expected budgets [600 1200], got %v", budgets)
	}
}

func TestSummarizeEmptyContentFailsAtCeiling(t *testing.T) {
	var calls int32
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		budgets = append(budgets, body.MaxTokens)
		mu.Unlock()
		atomic.AddInt32(&calls, 1)
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"length","message":{"content":"  ","reasoning_content":"still thinking"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); !errors.Is(err, ErrProvider) {
		t.Fatalf("expected provider error, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []int{600, 1200, 2400, 4800, 8000}
	if len(budgets) != len(want) {
		t.Fatalf("budgets = %v, want %v", budgets, want)
	}
	for i := range want {
		if budgets[i] != want[i] {
			t.Fatalf("budgets = %v, want %v", budgets, want)
		}
	}
}

func TestTransientRetryDoesNotConsumeBudgetRung(t *testing.T) {
	var calls int32
	var mu sync.Mutex
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		budgets = append(budgets, body.MaxTokens)
		mu.Unlock()
		if n == 2 {
			_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"thinking"}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":"Summary: ok"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// The 500 must not eat a rung: the empty response still starts at 600.
	want := []int{600, 1200}
	if len(budgets) != len(want) || budgets[0] != want[0] || budgets[1] != want[1] {
		t.Fatalf("budgets = %v, want %v", budgets, want)
	}
}

func TestBlankNonLengthResponseRetriesOnlyOnce(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); !errors.Is(err, ErrProvider) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestReasoningEffortForwarded(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":"Summary: ok"}}]}`)
	}))
	defer srv.Close()

	for _, effort := range []string{"none", "off", ""} {
		s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, ReasoningEffort: effort, Timeout: 5 * time.Second})
		if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); err != nil {
			t.Fatalf("effort %q: %v", effort, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(bodies))
	}
	if got, ok := bodies[0]["reasoning_effort"]; !ok || got != "none" {
		t.Fatalf("effort none not forwarded: %v", bodies[0])
	}
	for i, want := range []string{"off", ""} {
		if _, ok := bodies[i+1]["reasoning_effort"]; ok {
			t.Fatalf("effort %q must be omitted, got %v", want, bodies[i+1])
		}
	}
}

func TestRetriesChargeDailyBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"thinking"}}]}`)
	}))
	defer srv.Close()

	s := New(&Config{BaseURL: srv.URL, APIKey: "test", Model: "test-model", MaxOutputTokens: 600, DailyBudget: 100, Timeout: 5 * time.Second})
	if _, _, err := s.SummarizeArticle(context.Background(), "T", "C", "en"); err == nil {
		t.Fatal("expected failure")
	}
	s.mu.Lock()
	used := s.used
	s.mu.Unlock()
	if used != 5 {
		t.Fatalf("5 attempts must charge 5 budget units, got %d", used)
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

