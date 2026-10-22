package preloader

import (
	"context"
	"errors"
	"log"
	"strconv"
	"sync"

	"hn-reader/internal/fetcher"
	"hn-reader/internal/flight"
	"hn-reader/internal/storage"
	"hn-reader/internal/summarizer"
)

// Options controls the bounded preload queue.
type Options struct {
	Workers           int
	QueueSize         int
	MaxPendingPerUser int
}

// ArticleRequest represents a single article to be preloaded.
type ArticleRequest struct {
	HNID  int64  `json:"hn_id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Lang  string `json:"lang"`
}

type task struct {
	userID   string
	req      ArticleRequest
	reserved bool
}

// Preloader manages background article summarization with a bounded queue,
// per-user pending limits, and cross-user deduplication.
type Preloader struct {
	ctx    context.Context
	cancel context.CancelFunc

	queue chan *task
	wg    sync.WaitGroup

	flights *flight.Group

	st         storage.Repository
	fetcher    *fetcher.Fetcher
	summarizer *summarizer.Summarizer

	maxPending int

	mu       sync.Mutex
	pending  map[string]int
	inflight map[string]bool
	closed   bool
}

// New creates a Preloader and starts its workers.
// workers is kept as the fourth argument for compatibility with existing
// callers; options can override queue and per-user limits.
func New(st storage.Repository, f *fetcher.Fetcher, sm *summarizer.Summarizer, workers int, opts ...Options) *Preloader {
	opt := Options{Workers: workers, QueueSize: 200, MaxPendingPerUser: 20}
	if len(opts) > 0 {
		if opts[0].Workers > 0 {
			opt.Workers = opts[0].Workers
		}
		if opts[0].QueueSize > 0 {
			opt.QueueSize = opts[0].QueueSize
		}
		if opts[0].MaxPendingPerUser > 0 {
			opt.MaxPendingPerUser = opts[0].MaxPendingPerUser
		}
	}
	if opt.Workers <= 0 {
		opt.Workers = 3
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Preloader{
		ctx:        ctx,
		cancel:     cancel,
		queue:      make(chan *task, opt.QueueSize),
		flights:    &flight.Group{},
		st:         st,
		fetcher:    f,
		summarizer: sm,
		maxPending: opt.MaxPendingPerUser,
		pending:    make(map[string]int),
		inflight:   make(map[string]bool),
	}
	for i := 0; i < opt.Workers; i++ {
		go p.worker()
	}
	return p
}

// Submit enqueues an article for local/single-user mode.
func (p *Preloader) Submit(req ArticleRequest) (bool, string) {
	return p.SubmitForUser("", req, false)
}

// SubmitForUser enqueues an article for a user. reserved indicates the caller
// already reserved one quota unit and the worker must refund it on a cache hit
// or failure.
func (p *Preloader) SubmitForUser(userID string, req ArticleRequest, reserved bool) (bool, string) {
	if req.Lang == "" {
		req.Lang = "zh"
	}
	key := userID + "|" + strconv.FormatInt(req.HNID, 10) + "|" + req.Lang

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false, "closed"
	}
	if p.inflight[key] {
		return false, "duplicate"
	}
	if p.pending[userID] >= p.maxPending {
		return false, "user_limit"
	}

	t := &task{userID: userID, req: req, reserved: reserved}
	select {
	case p.queue <- t:
		p.pending[userID]++
		p.inflight[key] = true
		p.wg.Add(1)
		return true, ""
	default:
		return false, "queue_full"
	}
}

func (p *Preloader) worker() {
	for t := range p.queue {
		p.process(t)
		p.finish(t)
	}
}

func (p *Preloader) finish(t *task) {
	key := t.userID + "|" + strconv.FormatInt(t.req.HNID, 10) + "|" + t.req.Lang
	p.mu.Lock()
	if p.pending[t.userID] > 1 {
		p.pending[t.userID]--
	} else {
		delete(p.pending, t.userID)
	}
	delete(p.inflight, key)
	p.mu.Unlock()
	p.wg.Done()
}

func (p *Preloader) process(t *task) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("preloader: panic for hn_id=%d: %v", t.req.HNID, r)
			p.refund(t)
		}
	}()

	req := t.req
	if req.Lang == "" {
		req.Lang = "zh"
	}

	// Global cache first: a cached summary needs neither HN lookup nor LLM.
	if cached, err := p.st.GetGlobalArticle(req.HNID, req.Lang); err == nil && cached != nil && cached.Summary != "" {
		p.saveUser(t.userID, cached)
		p.refund(t)
		return
	} else if err != nil {
		log.Printf("preloader: global cache read failed for hn_id=%d: %v", req.HNID, err)
	}

	// Resolve the canonical story. This prevents cache poisoning and keeps
	// preload fetching on the same URL a direct summary would use.
	canonical := false
	story, err := p.fetcher.FetchStory(p.ctx, req.HNID)
	if err != nil {
		if errors.Is(err, fetcher.ErrNotStory) {
			log.Printf("preloader: hn_id=%d is not a story", req.HNID)
			p.refund(t)
			return
		}
		log.Printf("preloader: canonical lookup failed for hn_id=%d: %v", req.HNID, err)
	} else {
		canonical = true
		// Empty URL is meaningful for text posts; never fall back to the
		// client-supplied URL for a globally cached entry.
		req.URL = story.URL
		if story.Title != "" {
			req.Title = story.Title
		}
	}

	key := strconv.FormatInt(req.HNID, 10) + "|" + req.Lang
	val, err := p.flights.Do(p.ctx, key, func() (any, error) {
		// Double-check inside the flight leader.
		if cached, err := p.st.GetGlobalArticle(req.HNID, req.Lang); err == nil && cached != nil && cached.Summary != "" {
			return cached, nil
		}

		if req.URL != "" {
			if _, err := fetcher.ValidateURL(p.ctx, req.URL); err != nil {
				return nil, err
			}
			content, err := p.fetcher.FetchArticle(p.ctx, req.URL)
			if err != nil {
				if fetcher.IsBlockedError(err) {
					return nil, err
				}
				log.Printf("preloader: fetch article hn_id=%d failed: %v", req.HNID, err)
			}
			return p.generate(req, content, canonical)
		}
		return p.generate(req, "", canonical)
	})
	if err != nil {
		log.Printf("preloader: summarize hn_id=%d failed: %v", req.HNID, err)
		p.refund(t)
		return
	}

	article, ok := val.(*storage.Article)
	if !ok || article == nil {
		p.refund(t)
		return
	}
	p.saveUser(t.userID, article)
}

func (p *Preloader) generate(req ArticleRequest, content string, canonical bool) (any, error) {
	summary, keyPoints, err := p.summarizer.SummarizeArticle(p.ctx, req.Title, content, req.Lang)
	if err != nil {
		return nil, err
	}
	article := &storage.Article{
		HNID:      req.HNID,
		URL:       req.URL,
		Title:     req.Title,
		Summary:   summary,
		KeyPoints: keyPoints,
		Lang:      req.Lang,
	}
	if canonical {
		if err := p.st.SaveGlobalArticle(article); err != nil {
			log.Printf("preloader: save global article hn_id=%d failed: %v", req.HNID, err)
		}
	}
	return article, nil
}

func (p *Preloader) saveUser(userID string, a *storage.Article) {
	if err := p.st.SaveArticle(userID, a); err != nil {
		log.Printf("preloader: save user article hn_id=%d failed: %v", a.HNID, err)
	}
}

func (p *Preloader) refund(t *task) {
	if !t.reserved || t.userID == "" {
		return
	}
	if err := p.st.ReleaseQuota(t.userID, 1); err != nil {
		log.Printf("preloader: refund quota user=%s failed: %v", t.userID, err)
	}
}

// IsCached returns the subset of hnIDs already cached for the local user.
func (p *Preloader) IsCached(ctx context.Context, hnIDs []int64, lang string) ([]int64, error) {
	return p.IsCachedForUser(ctx, "", hnIDs, lang)
}

// IsCachedForUser checks the user cache and then the shared global cache.
func (p *Preloader) IsCachedForUser(ctx context.Context, userID string, hnIDs []int64, lang string) ([]int64, error) {
	if len(hnIDs) == 0 {
		return []int64{}, nil
	}
	articles, err := p.st.GetArticlesByIDs(userID, hnIDs)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(hnIDs))
	var ready []int64
	for _, a := range articles {
		if a.Lang == lang && a.Summary != "" {
			ready = append(ready, a.HNID)
			seen[a.HNID] = true
		}
	}
	for _, id := range hnIDs {
		if seen[id] {
			continue
		}
		if g, err := p.st.GetGlobalArticle(id, lang); err == nil && g != nil && g.Summary != "" {
			ready = append(ready, id)
		}
	}
	return ready, nil
}

// Close stops accepting tasks, cancels in-flight work, and waits for workers.
func (p *Preloader) Close(ctx context.Context) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.queue)
	p.mu.Unlock()

	p.cancel()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
