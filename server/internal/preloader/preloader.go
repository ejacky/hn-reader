package preloader

import (
	"log"
	"sync"

	"hn-reader/internal/fetcher"
	"hn-reader/internal/storage"
	"hn-reader/internal/summarizer"
)

// Preloader manages background article summarization with controlled concurrency.
type Preloader struct {
	sem        chan struct{}  // concurrency limiter
	inflight   map[int64]bool // hn_ids currently being processed
	inflightMu sync.RWMutex
	st         storage.Repository
	fetcher    *fetcher.Fetcher
	summarizer *summarizer.Summarizer
}

// ArticleRequest represents a single article to be preloaded.
type ArticleRequest struct {
	HNID  int64  `json:"hn_id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Lang  string `json:"lang"`
}

// New creates a Preloader with the given concurrency limit.
// userID is the owner for cloud mode; pass empty string for local mode.
func New(st storage.Repository, f *fetcher.Fetcher, sm *summarizer.Summarizer, concurrency int) *Preloader {
	if concurrency <= 0 {
		concurrency = 3
	}
	return &Preloader{
		sem:        make(chan struct{}, concurrency),
		inflight:   make(map[int64]bool),
		st:         st,
		fetcher:    f,
		summarizer: sm,
	}
}

// Submit enqueues an article for background preloading (local/single-user mode).
func (p *Preloader) Submit(req ArticleRequest) {
	p.SubmitForUser("", req)
}

// SubmitForUser enqueues an article for a specific user (cloud mode).
func (p *Preloader) SubmitForUser(userID string, req ArticleRequest) {
	if req.Lang == "" {
		req.Lang = "zh"
	}

	// Deduplicate in-flight tasks.
	p.inflightMu.Lock()
	if p.inflight[req.HNID] {
		p.inflightMu.Unlock()
		return
	}
	p.inflight[req.HNID] = true
	p.inflightMu.Unlock()

	// Check cache.
	existing, err := p.st.GetArticle(userID, req.HNID, req.Lang)
	if err != nil {
		log.Printf("preloader: cache check failed for %d: %v", req.HNID, err)
		p.clearInflight(req.HNID)
		return
	}
	if existing != nil && existing.Summary != "" {
		p.clearInflight(req.HNID)
		return
	}

	// Process in background.
	go p.process(userID, req)
}

func (p *Preloader) process(userID string, req ArticleRequest) {
	p.sem <- struct{}{}
	defer func() { <-p.sem }()
	defer p.clearInflight(req.HNID)

	content, err := p.fetcher.FetchArticle(req.URL)
	if err != nil {
		log.Printf("preloader: fetch article %d failed: %v", req.HNID, err)
		content = ""
	}

	summary, keyPoints, err := p.summarizer.SummarizeArticle(req.Title, content, req.Lang)
	if err != nil {
		log.Printf("preloader: summarize article %d failed: %v", req.HNID, err)
		return
	}

	article := &storage.Article{
		HNID:           req.HNID,
		URL:            req.URL,
		Title:          req.Title,
		FetchedContent: content,
		Summary:        summary,
		KeyPoints:      keyPoints,
		Lang:           req.Lang,
	}
	if err := p.st.SaveArticle(userID, article); err != nil {
		log.Printf("preloader: save article %d failed: %v", req.HNID, err)
	} else {
		log.Printf("preloader: article %d preloaded successfully", req.HNID)
	}
}

func (p *Preloader) clearInflight(hnID int64) {
	p.inflightMu.Lock()
	delete(p.inflight, hnID)
	p.inflightMu.Unlock()
}

// IsCached returns the subset of hnIDs that already have a cached summary for the given language.
func (p *Preloader) IsCached(hnIDs []int64, lang string) ([]int64, error) {
	return p.IsCachedForUser("", hnIDs, lang)
}

// IsCachedForUser is the cloud-mode variant of IsCached.
func (p *Preloader) IsCachedForUser(userID string, hnIDs []int64, lang string) ([]int64, error) {
	if len(hnIDs) == 0 {
		return []int64{}, nil
	}
	articles, err := p.st.GetArticlesByIDs(userID, hnIDs)
	if err != nil {
		return nil, err
	}
	var ready []int64
	for _, a := range articles {
		if a.Lang == lang && a.Summary != "" {
			ready = append(ready, a.HNID)
		}
	}
	return ready, nil
}
