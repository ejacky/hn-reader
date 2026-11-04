package site

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)
	return r
}

func get(t *testing.T, r *gin.Engine, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestIndexIsServedWithSecurityHeaders(t *testing.T) {
	w := get(t, newRouter(), "/")

	if w.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache so copy edits show up on reload", cc)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy"} {
		if w.Header().Get(h) == "" {
			t.Errorf("missing %s header", h)
		}
	}

	body := w.Body.String()
	for _, want := range []string{"HN Reader", "/assets/site.css", "/assets/site.js"} {
		if !strings.Contains(body, want) {
			t.Errorf("index.html does not mention %q", want)
		}
	}
}

func TestAssetsAreServedWithTheirContentType(t *testing.T) {
	r := newRouter()
	cases := []struct {
		target      string
		contentType string
	}{
		{"/assets/site.css", "text/css; charset=utf-8"},
		{"/assets/site.js", "text/javascript; charset=utf-8"},
		{"/assets/icon.png", "image/png"},
		{"/assets/hn-list.png", "image/png"},
	}

	for _, tc := range cases {
		w := get(t, r, tc.target)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.target, w.Code)
			continue
		}
		if ct := w.Header().Get("Content-Type"); ct != tc.contentType {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.target, ct, tc.contentType)
		}
		if w.Body.Len() == 0 {
			t.Errorf("GET %s returned an empty body", tc.target)
		}
	}
}

// Every asset the page references must exist in the embed, otherwise the page
// ships with a broken stylesheet or a missing screenshot.
func TestReferencedAssetsExist(t *testing.T) {
	s := New()
	index := string(s.files[indexKey])

	ref := regexp.MustCompile(`(?:src|href)="(/assets/[^"?#]+)"`)
	for _, m := range ref.FindAllStringSubmatch(index, -1) {
		key := strings.TrimPrefix(strings.TrimPrefix(m[1], "/"), "assets/")
		if len(s.files[prefix+key]) == 0 {
			t.Errorf("index.html references %s, which is not embedded", m[1])
		}
	}
}

func TestUnknownAssetIsNotFound(t *testing.T) {
	r := newRouter()
	for _, target := range []string{
		"/assets/nope.css",
		"/assets/",
		"/assets/../index.html",
		"/assets/%2e%2e/site.go",
	} {
		w := get(t, r, target)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", target, w.Code)
		}
		if strings.Contains(w.Body.String(), "HN Reader") {
			t.Errorf("GET %s leaked the index page", target)
		}
	}
}

func TestFaviconRedirectsToIcon(t *testing.T) {
	w := get(t, newRouter(), "/favicon.ico")
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /favicon.ico = %d, want 301", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/assets/icon.png" {
		t.Errorf("Location = %q, want /assets/icon.png", loc)
	}
}

func TestUnknownPathKeepsDefault404(t *testing.T) {
	w := get(t, newRouter(), "/definitely-not-a-page")
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /definitely-not-a-page = %d, want 404", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if strings.Contains(string(body), "<html") {
		t.Error("unknown path served the landing page")
	}
}
