package preloader

import (
	"encoding/json"
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
	st         *storage.Storage
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
func New(st *storage.Storage, f *fetcher.Fetcher, sm *summarizer.Summarizer, concurrency int) *Preloader {
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

// Submit enqueues an article for background preloading.
// It is non-blocking and deduplicates by hn_id.
func (p *Preloader) Submit(req ArticleRequest) {
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

	// Check SQLite cache.
	existing, err := p.st.GetArticleByHNID(req.HNID, req.Lang)
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
	go p.process(req)
}

func (p *Preloader) process(req ArticleRequest) {
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

	kpJSON, _ := json.Marshal(keyPoints)
	article := &storage.Article{
		HNID:           req.HNID,
		URL:            req.URL,
		Title:          req.Title,
		FetchedContent: content,
		Summary:        summary,
		KeyPoints:      string(kpJSON),
		Lang:           req.Lang,
	}
	if err := p.st.SaveArticle(article); err != nil {
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
	if len(hnIDs) == 0 {
		return []int64{}, nil
	}
	articles, err := p.st.GetArticlesByIDs(hnIDs)
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
