// Package site serves the public landing page from the cloud binary.
//
// The page is embedded rather than mounted from the host on purpose: the edge
// Caddy on h43 already proxies every path of hn-reader.whoareyou.ren to this
// process, so shipping the marketing site inside the same artifact keeps a
// deploy to one build + one container, and leaves the shared edge config (and
// its reload) untouched. Copy changes therefore need a rebuild — acceptable for
// a page that only changes with a release.
package site

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed static
var embedded embed.FS

const (
	dirRoot  = "static"
	indexKey = "index.html"
	prefix   = "assets/"
)

// contentTypes is explicit instead of relying on mime.TypeByExtension, which
// reads the Windows registry and can disagree with the Linux runtime image.
var contentTypes = map[string]string{
	".css":  "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".txt":  "text/plain; charset=utf-8",
	".webp": "image/webp",
}

// Sites holds the embedded page. Build one per router.
type Sites struct {
	files   map[string][]byte
	modTime time.Time
}

// New loads the embedded files. It panics on a broken embed (a build mistake),
// which is preferable to serving a half-empty page in production.
func New() *Sites {
	s := &Sites{files: make(map[string][]byte), modTime: time.Now()}

	err := fs.WalkDir(embedded, dirRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := embedded.ReadFile(p)
		if err != nil {
			return err
		}
		s.files[strings.TrimPrefix(p, dirRoot+"/")] = body
		return nil
	})
	if err != nil {
		panic("hn-reader: embedded site is unreadable: " + err.Error())
	}
	if len(s.files[indexKey]) == 0 {
		panic("hn-reader: embedded site has no " + indexKey)
	}
	return s
}

// Register mounts the landing page on r. Only the three paths the page needs are
// registered, so the API keeps its own 404 behaviour.
func Register(r *gin.Engine) {
	s := New()

	index := s.serve(indexKey, "no-cache")
	r.GET("/", index)
	r.HEAD("/", index)

	assets := s.serveAsset()
	r.GET("/assets/*filepath", assets)
	r.HEAD("/assets/*filepath", assets)

	// Browsers ask for /favicon.ico even when the page declares a PNG icon.
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/assets/icon.png")
	})
}

func (s *Sites) serve(name, cacheControl string) gin.HandlerFunc {
	body := s.files[name]
	return func(c *gin.Context) {
		setHeaders(c, cacheControl)
		http.ServeContent(c.Writer, c.Request, name, s.modTime, bytes.NewReader(body))
	}
}

func (s *Sites) serveAsset() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Clean first: a traversal attempt collapses to a key that is simply not
		// in the map, because lookups are exact and never touch the filesystem.
		rel := strings.TrimPrefix(path.Clean("/"+c.Param("filepath")), "/")
		body, ok := s.files[prefix+rel]
		if !ok || rel == "" || strings.HasSuffix(rel, "/") {
			c.Status(http.StatusNotFound)
			return
		}

		setHeaders(c, "public, max-age=86400")
		c.Writer.Header().Set("Content-Type", contentType(rel))
		http.ServeContent(c.Writer, c.Request, rel, s.modTime, bytes.NewReader(body))
	}
}

func setHeaders(c *gin.Context, cacheControl string) {
	h := c.Writer.Header()
	// No inline script or style: everything is served from this origin, and
	// nothing on the page needs to phone home.
	h.Set("Content-Security-Policy",
		"default-src 'none'; img-src 'self'; style-src 'self'; script-src 'self'; "+
			"base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Cache-Control", cacheControl)
}

func contentType(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
