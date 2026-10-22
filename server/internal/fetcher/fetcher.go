package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	maxArticleBytes = 2 << 20 // 2 MiB
	maxHNItemBytes  = 1 << 20 // 1 MiB
)

type Fetcher struct {
	client    *http.Client
	hnClient  *http.Client
	hnBaseURL string
	dialer    *net.Dialer
	hnCache   sync.Map // int64 -> hnCacheEntry
}

type hnCacheEntry struct {
	story   HNStory
	expires time.Time
}

type HNStory struct {
	HNID  int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// New creates a fetcher. The optional hnBaseURL is primarily for tests; the
// production default is the public Hacker News Firebase endpoint.
func New(hnBaseURL ...string) *Fetcher {
	base := "https://hacker-news.firebaseio.com/v0"
	if len(hnBaseURL) > 0 && hnBaseURL[0] != "" {
		base = strings.TrimRight(hnBaseURL[0], "/")
	}

	f := &Fetcher{
		hnBaseURL: base,
		dialer:    &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}

	transport := &http.Transport{
		// Do not use an environment proxy: it would make the SSRF check dial
		// the proxy address instead of the article host.
		Proxy:                 nil,
		DialContext:           f.safeDialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
	}
	f.client = &http.Client{
		Transport:     transport,
		Timeout:       45 * time.Second,
		CheckRedirect: f.checkRedirect,
	}
	f.hnClient = &http.Client{Timeout: 10 * time.Second}
	return f
}

func (f *Fetcher) safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if port != "80" && port != "443" {
		return nil, fmt.Errorf("%w: port %s", ErrBlockedURL, port)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve %s: no addresses", host)
	}

	var lastErr error
	for _, ip := range ips {
		if IsBlockedIP(ip.IP) {
			lastErr = fmt.Errorf("%w: %s", ErrBlockedURL, ip.IP)
			continue
		}
		conn, err := f.dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = ErrBlockedURL
	}
	return nil, lastErr
}

func (f *Fetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("too many redirects")
	}
	if _, err := ValidateURL(req.Context(), req.URL.String()); err != nil {
		return err
	}
	return nil
}

// FetchArticle downloads and extracts the readable text of an article.
func (f *Fetcher) FetchArticle(ctx context.Context, rawURL string) (string, error) {
	if _, err := ValidateURL(ctx, rawURL); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxArticleBytes {
		return "", fmt.Errorf("response too large: %d bytes", resp.ContentLength)
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); ct != "" &&
		!strings.HasPrefix(ct, "text/html") &&
		!strings.HasPrefix(ct, "application/xhtml+xml") &&
		!strings.HasPrefix(ct, "text/plain") {
		return "", fmt.Errorf("unsupported content type %q", ct)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxArticleBytes))
	if err != nil {
		return "", err
	}

	var text strings.Builder
	doc.Find("article, main, .post, .entry, [role='main']").Each(func(i int, s *goquery.Selection) {
		text.WriteString(s.Text())
		text.WriteString("\n")
	})

	if text.Len() == 0 {
		doc.Find("p, h1, h2, h3, h4, li").Each(func(i int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if len(t) > 10 {
				text.WriteString(t)
				text.WriteString("\n")
			}
		})
	}

	result := strings.TrimSpace(text.String())
	if len(result) > 12000 {
		result = result[:12000]
	}
	return result, nil
}

// FetchStory resolves an HN item id to its canonical title/url. It is used to
// prevent clients from poisoning the shared summary cache with arbitrary
// title/url pairs for a valid HN id.
func (f *Fetcher) FetchStory(ctx context.Context, hnID int64) (*HNStory, error) {
	if v, ok := f.hnCache.Load(hnID); ok {
		if e := v.(hnCacheEntry); time.Now().Before(e.expires) {
			s := e.story
			return &s, nil
		}
		f.hnCache.Delete(hnID)
	}

	url := fmt.Sprintf("%s/item/%d.json", f.hnBaseURL, hnID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.hnClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HN item %d: HTTP %d", hnID, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHNItemBytes))
	if err != nil {
		return nil, err
	}
	var item struct {
		ID      int64  `json:"id"`
		Type    string `json:"type"`
		Title   string `json:"title"`
		URL     string `json:"url"`
		Deleted bool   `json:"deleted"`
		Dead    bool   `json:"dead"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	if (item.Type != "story" && item.Type != "job" && item.Type != "poll") || item.Deleted || item.Dead {
		return nil, ErrNotStory
	}

	story := HNStory{HNID: item.ID, Title: item.Title, URL: item.URL}
	f.hnCache.Store(hnID, hnCacheEntry{story: story, expires: time.Now().Add(5 * time.Minute)})
	return &story, nil
}

func (f *Fetcher) FetchHNTopStories(ctx context.Context) ([]HNStory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.hnBaseURL+"/topstories.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.hnClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHNItemBytes))
	if err != nil {
		return nil, err
	}

	var ids []int64
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, err
	}
	if len(ids) > 30 {
		ids = ids[:30]
	}

	stories := make([]HNStory, 0, len(ids))
	for _, id := range ids {
		story, err := f.FetchStory(ctx, id)
		if err != nil || story.URL == "" {
			continue
		}
		stories = append(stories, *story)
	}
	return stories, nil
}

// IsBlockedError reports whether err came from the SSRF guard.
func IsBlockedError(err error) bool {
	return errors.Is(err, ErrBlockedURL)
}
