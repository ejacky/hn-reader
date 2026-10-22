package fetcher

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func parseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("invalid test IP %q", s)
	}
	return ip
}

func newTestServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(h)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fc00::1", "fe80::1",
	}
	for _, s := range blocked {
		ip := parseIP(t, s)
		if !IsBlockedIP(ip) {
			t.Errorf("expected %s blocked", s)
		}
	}
	for _, s := range []string{"93.184.216.34", "1.1.1.1", "2606:4700:4700::1111"} {
		ip := parseIP(t, s)
		if IsBlockedIP(ip) {
			t.Errorf("expected %s allowed", s)
		}
	}
}

func TestValidateURL(t *testing.T) {
	ctx := context.Background()
	allowed := []string{
		"http://93.184.216.34/article",
		"https://93.184.216.34:443/a",
	}
	for _, raw := range allowed {
		if _, err := ValidateURL(ctx, raw); err != nil {
			t.Errorf("expected %s allowed: %v", raw, err)
		}
	}

	blocked := []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://169.254.169.254/latest/meta-data",
		"http://100.64.0.1/",
		"http://[::1]/",
		"http://localhost/",
		"http://foo.local/",
		"http://intranet/",
		"ftp://93.184.216.34/",
		"http://93.184.216.34:8080/",
		"http://user:pass@93.184.216.34/",
	}
	for _, raw := range blocked {
		if _, err := ValidateURL(ctx, raw); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("expected %s blocked, got %v", raw, err)
		}
	}
}

func TestFetchArticleBlocked(t *testing.T) {
	f := New()
	if _, err := f.FetchArticle(context.Background(), "http://127.0.0.1/secret"); !IsBlockedError(err) {
		t.Fatalf("expected blocked error, got %v", err)
	}
}

func TestFetchArticleExtractsText(t *testing.T) {
	f := New()
	f.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(`<html><body><article><p>Hello HN readers, this is a test article with enough text.</p></article></body></html>`)),
			Request:    r,
		}, nil
	})

	text, err := f.FetchArticle(context.Background(), "http://93.184.216.34/article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Hello HN readers") {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestFetchStoryCaches(t *testing.T) {
	var hits int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, `{"id":1,"type":"story","title":"Hello","url":"https://example.com"}`)
	})
	defer srv.Close()

	f := New(srv.URL)
	s, err := f.FetchStory(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Title != "Hello" || s.URL != "https://example.com" {
		t.Fatalf("unexpected story: %+v", s)
	}
	if _, err := f.FetchStory(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected one HN request, got %d", got)
	}
}

func TestFetchStoryNotStory(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":2,"type":"comment"}`)
	})
	defer srv.Close()

	f := New(srv.URL)
	if _, err := f.FetchStory(context.Background(), 2); !errors.Is(err, ErrNotStory) {
		t.Fatalf("expected ErrNotStory, got %v", err)
	}
}
