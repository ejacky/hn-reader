//go:build !cloud

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hn-reader/internal/config"
	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

func TestLocalSummarizeUsesCacheAndRejectsBlockedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var llmCalls int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&llmCalls, 1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Summary: test summary.\nKey Points:\n- one\n- two"}}]}`))
	}))
	defer llm.Close()

	hn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/item/1.json") {
			_, _ = w.Write([]byte(`{"id":1,"type":"story","title":"Canonical Title","url":""}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/item/2.json") {
			_, _ = w.Write([]byte(`{"id":2,"type":"story","title":"Blocked","url":"http://127.0.0.1/secret"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":3,"type":"comment"}`))
	}))
	defer hn.Close()

	repo, err := storage.NewSqlite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	cfg := &config.Config{
		LLMBaseURL:        llm.URL,
		LLMAPIKey:         "test",
		LLMModel:          "test-model",
		LLMTimeout:        5 * time.Second,
		LLMDailyBudget:    100,
		LLMMaxConcurrency: 1,
		HNBaseURL:         hn.URL,
	}
	s := NewLocal(cfg, repo, "test")
	defer s.Close(context.Background())

	r := gin.New()
	s.RegisterRoutes(r)

	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	first := post("/api/summarize/article", `{"hn_id":1,"url":"","title":"Client Title","lang":"en"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body=%s", first.Code, first.Body.String())
	}
	var resp SummarizeArticleResp
	if err := json.Unmarshal(first.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Summary != "test summary." || len(resp.KeyPoints) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	second := post("/api/summarize/article", `{"hn_id":1,"url":"","title":"Client Title","lang":"en"}`)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d", second.Code)
	}
	if got := atomic.LoadInt32(&llmCalls); got != 1 {
		t.Fatalf("cache miss: LLM calls = %d", got)
	}

	blocked := post("/api/summarize/article", `{"hn_id":2,"url":"http://127.0.0.1/secret","title":"Blocked","lang":"en"}`)
	if blocked.Code != http.StatusBadRequest || !strings.Contains(blocked.Body.String(), "invalid_url") {
		t.Fatalf("blocked status = %d, body=%s", blocked.Code, blocked.Body.String())
	}

	invalid := post("/api/summarize/article", `{"hn_id":3,"url":"","title":"Comment","lang":"en"}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "invalid_hn_id") {
		t.Fatalf("invalid status = %d, body=%s", invalid.Code, invalid.Body.String())
	}
}
