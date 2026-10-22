package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hn-reader/internal/config"
	"hn-reader/internal/fetcher"
	"hn-reader/internal/preloader"
	"hn-reader/internal/storage"
	"hn-reader/internal/summarizer"

	"github.com/gin-gonic/gin"
)

// Server holds shared dependencies and works for both local and cloud modes.
type Server struct {
	config     *config.Config
	repo       storage.Repository
	summarizer *summarizer.Summarizer
	fetcher    *fetcher.Fetcher
	preloader  *preloader.Preloader
	version    string
	cloud      bool
	limiters   *limiters
}

// NewLocal creates a Server for the local desktop deployment (SQLite, no auth).
func NewLocal(cfg *config.Config, repo storage.Repository, version string) *Server {
	return newServer(cfg, repo, version, false)
}

// NewCloud creates a Server for the cloud deployment (Postgres, multi-user, auth).
func NewCloud(cfg *config.Config, repo storage.Repository, version string) *Server {
	return newServer(cfg, repo, version, true)
}

func newServer(cfg *config.Config, repo storage.Repository, version string, cloud bool) *Server {
	sm := summarizer.New(&summarizer.Config{
		BaseURL:         cfg.LLMBaseURL,
		APIKey:          cfg.LLMAPIKey,
		Model:           cfg.LLMModel,
		MaxConcurrency:  cfg.LLMMaxConcurrency,
		DailyBudget:     cfg.LLMDailyBudget,
		Timeout:         cfg.LLMTimeout,
		MaxOutputTokens: cfg.LLMMaxOutputTokens,
	})
	f := fetcher.New(cfg.HNBaseURL)
	s := &Server{
		config:     cfg,
		repo:       repo,
		summarizer: sm,
		fetcher:    f,
		version:    version,
		cloud:      cloud,
		limiters:   newLimiters(cfg),
	}
	s.preloader = preloader.New(repo, f, sm, 3, preloader.Options{
		Workers:           3,
		QueueSize:         cfg.PreloadQueueSize,
		MaxPendingPerUser: cfg.PreloadMaxPending,
	})
	return s
}

// Close stops background preload work.
func (s *Server) Close(ctx context.Context) {
	if s.preloader != nil {
		s.preloader.Close(ctx)
	}
}

// RegisterRoutes sets up all HTTP routes.
func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.handleHealth)
	r.POST("/auth/anonymous", s.signupLimit(), s.handleAnonymousAuth)

	api := r.Group("/api")
	api.Use(s.authMiddleware())
	api.Use(s.apiLimit())
	{
		api.POST("/summarize/article", s.summarizeLimit(), s.handleSummarizeArticle)
		api.POST("/summarize/comment", s.summarizeLimit(), s.handleSummarizeComment)
		api.POST("/summarize/article/preload", s.preloadLimit(), s.handlePreloadArticles)
		api.GET("/summarize/article/preload-status", s.handlePreloadStatus)
		api.POST("/signal", s.signalLimit(), s.handleSignal)
		api.GET("/signal/stats", s.handleSignalStats)
		api.GET("/preference/profile", s.handlePreferenceProfile)
		api.POST("/match/batch", s.handleBatchMatch)
		api.GET("/recommendations", s.handleRecommendations)
		api.GET("/account/export", s.accountLimit(), s.handleAccountExport)
		api.DELETE("/account", s.accountLimit(), s.handleAccountDelete)
	}

	auth := r.Group("/auth")
	auth.Use(s.authMiddleware())
	{
		auth.GET("/me", s.handleMe)
	}
}

// --- Request / Response types ---

type SummarizeArticleReq struct {
	HNID  int64  `json:"hn_id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Lang  string `json:"lang"`
}

type SummarizeArticleResp struct {
	Summary   string   `json:"summary"`
	KeyPoints []string `json:"key_points"`
	Cached    bool     `json:"cached"`
}

type SummarizeCommentReq struct {
	HNID      int64  `json:"hn_id"`
	ArticleID int64  `json:"article_id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	Lang      string `json:"lang"`
}

type SummarizeCommentResp struct {
	Summary string `json:"summary"`
	Cached  bool   `json:"cached"`
}

type SignalReq struct {
	TargetType string                 `json:"target_type"`
	TargetID   int64                  `json:"target_id"`
	SignalType string                 `json:"signal_type"`
	Strength   int                    `json:"strength"`
	Context    map[string]interface{} `json:"context"`
}

type BatchMatchReq struct {
	Articles []struct {
		HNID   int64  `json:"hn_id"`
		Title  string `json:"title"`
		Domain string `json:"domain"`
	} `json:"articles"`
}

type BatchMatchResp struct {
	Matches []MatchResult `json:"matches"`
}

type MatchResult struct {
	HNID        int64    `json:"hn_id"`
	MatchType   string   `json:"match_type"`
	MatchTags   []string `json:"match_tags"`
	MatchReason string   `json:"match_reason"`
}

// --- Handlers ---

func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.repo.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unavailable",
			"version": s.version,
			"db":      "error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": s.version,
		"db":      "ok",
	})
}

func (s *Server) handleAnonymousAuth(c *gin.Context) {
	user, err := s.repo.CreateAnonymousUser(s.config.FreeQuotaLimit)
	if err != nil {
		s.internalError(c, err, "create anonymous user failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": user.APIKey})
}

func (s *Server) handleMe(c *gin.Context) {
	user := getUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if s.cloud {
		if fresh, err := s.repo.GetUserQuota(user.ID); err == nil && fresh != nil {
			user = fresh
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"email":          user.Email,
		"plan":           user.Plan,
		"quota_used":     user.QuotaUsed,
		"quota_limit":    user.QuotaLimit,
		"quota_reset_at": user.QuotaResetAt,
	})
}

func (s *Server) handleSummarizeArticle(c *gin.Context) {
	var req SummarizeArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	req.Lang = normalizeLang(req.Lang)
	if req.HNID <= 0 || strings.TrimSpace(req.Title) == "" ||
		len(req.Title) > 500 || len(req.URL) > 2048 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), s.llmTimeout()+30*time.Second)
	defer cancel()

	userID := s.getUserID(c)

	if existing, err := s.repo.GetArticle(userID, req.HNID, req.Lang); err != nil {
		log.Printf("read user article cache failed: %v", err)
	} else if existing != nil && existing.Summary != "" {
		c.JSON(http.StatusOK, SummarizeArticleResp{
			Summary:   existing.Summary,
			KeyPoints: existing.KeyPoints,
			Cached:    true,
		})
		return
	}
	if cached, err := s.repo.GetGlobalArticle(req.HNID, req.Lang); err != nil {
		log.Printf("read global article cache failed: %v", err)
	} else if cached != nil && cached.Summary != "" {
		if err := s.repo.SaveArticle(userID, cached); err != nil {
			log.Printf("save cached article for user failed: %v", err)
		}
		c.JSON(http.StatusOK, SummarizeArticleResp{
			Summary:   cached.Summary,
			KeyPoints: cached.KeyPoints,
			Cached:    true,
		})
		return
	}

	canonical := false
	story, err := s.fetcher.FetchStory(ctx, req.HNID)
	if err != nil {
		if errors.Is(err, fetcher.ErrNotStory) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_hn_id"})
			return
		}
		log.Printf("canonical HN lookup failed for %d: %v", req.HNID, err)
	} else {
		canonical = true
		// Use the canonical URL even when it is empty (text post): the client
		// must not be able to substitute an arbitrary URL for a shared cache key.
		req.URL = story.URL
		if story.Title != "" {
			req.Title = story.Title
		}
	}

	if req.URL != "" {
		if _, err := fetcher.ValidateURL(ctx, req.URL); err != nil {
			if fetcher.IsBlockedError(err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_url"})
				return
			}
			log.Printf("url validation failed for %s: %v", req.URL, err)
		}
	}

	reserved := false
	if userID != "" {
		used, limit, ok, err := s.repo.ReserveQuota(userID, 1)
		if err != nil {
			s.internalError(c, err, "reserve quota failed")
			return
		}
		if !ok {
			s.writeQuotaExceeded(c, userID, used, limit)
			return
		}
		reserved = true
		defer func() {
			if reserved {
				if err := s.repo.ReleaseQuota(userID, 1); err != nil {
					log.Printf("release quota failed: %v", err)
				}
			}
		}()
	}

	content := ""
	if req.URL != "" {
		content, err = s.fetcher.FetchArticle(ctx, req.URL)
		if err != nil {
			if fetcher.IsBlockedError(err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_url"})
				return
			}
			log.Printf("fetch article failed for %s: %v", req.URL, err)
			content = ""
		}
	}

	summary, keyPoints, err := s.summarizer.SummarizeArticle(ctx, req.Title, content, req.Lang)
	if err != nil {
		s.writeLLMError(c, err)
		return
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
		if err := s.repo.SaveGlobalArticle(article); err != nil {
			log.Printf("save global article failed: %v", err)
		}
	}
	if err := s.repo.SaveArticle(userID, article); err != nil {
		log.Printf("save article failed: %v", err)
	}
	reserved = false

	c.JSON(http.StatusOK, SummarizeArticleResp{
		Summary:   summary,
		KeyPoints: keyPoints,
		Cached:    false,
	})
}

func (s *Server) handleSummarizeComment(c *gin.Context) {
	var req SummarizeCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	req.Lang = normalizeLang(req.Lang)
	if req.HNID <= 0 || strings.TrimSpace(req.Text) == "" || len(req.Text) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	userID := s.getUserID(c)
	if existing, err := s.repo.GetComment(userID, req.HNID, req.Lang); err != nil {
		log.Printf("read comment cache failed: %v", err)
	} else if existing != nil && existing.Summary != "" {
		c.JSON(http.StatusOK, SummarizeCommentResp{Summary: existing.Summary, Cached: true})
		return
	}

	reserved := false
	if userID != "" {
		used, limit, ok, err := s.repo.ReserveQuota(userID, 1)
		if err != nil {
			s.internalError(c, err, "reserve quota failed")
			return
		}
		if !ok {
			s.writeQuotaExceeded(c, userID, used, limit)
			return
		}
		reserved = true
		defer func() {
			if reserved {
				if err := s.repo.ReleaseQuota(userID, 1); err != nil {
					log.Printf("release quota failed: %v", err)
				}
			}
		}()
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), s.llmTimeout()+10*time.Second)
	defer cancel()

	summary, err := s.summarizer.SummarizeComment(ctx, req.Text, req.Lang)
	if err != nil {
		s.writeLLMError(c, err)
		return
	}

	comment := &storage.Comment{
		HNID:      req.HNID,
		ArticleID: req.ArticleID,
		Author:    req.Author,
		RawText:   req.Text,
		Summary:   summary,
		Lang:      req.Lang,
	}
	if err := s.repo.SaveComment(userID, comment); err != nil {
		log.Printf("save comment failed: %v", err)
	}
	reserved = false

	c.JSON(http.StatusOK, SummarizeCommentResp{Summary: summary, Cached: false})
}

func (s *Server) handleSignal(c *gin.Context) {
	var req SignalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if req.TargetID <= 0 || req.TargetType == "" || req.SignalType == "" ||
		len(req.TargetType) > 32 || len(req.SignalType) > 64 ||
		req.Strength < 0 || req.Strength > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if raw, err := json.Marshal(req.Context); err != nil || len(raw) > 8192 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_context"})
		return
	}

	userID := s.getUserID(c)
	signal := &storage.UserSignal{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		SignalType: req.SignalType,
		Strength:   req.Strength,
		Context:    req.Context,
		CreatedAt:  time.Now(),
	}
	if err := s.repo.SaveSignal(userID, signal); err != nil {
		s.internalError(c, err, "save signal failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleSignalStats(c *gin.Context) {
	userID := s.getUserID(c)
	stats, err := s.repo.GetSignalStats(userID)
	if err != nil {
		s.internalError(c, err, "get signal stats failed")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handlePreferenceProfile(c *gin.Context) {
	userID := s.getUserID(c)
	profile, err := s.buildProfile(userID)
	if err != nil {
		s.internalError(c, err, "build profile failed")
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (s *Server) handleBatchMatch(c *gin.Context) {
	var req BatchMatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if len(req.Articles) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too_many_articles"})
		return
	}

	userID := s.getUserID(c)
	profile, err := s.buildProfile(userID)
	if err != nil {
		c.JSON(http.StatusOK, BatchMatchResp{Matches: []MatchResult{}})
		return
	}

	topics, _ := profile["topics"].([]string)
	techStacks, _ := profile["tech_stacks"].([]string)
	authors, _ := profile["authors"].([]string)

	var matches []MatchResult
	for _, article := range req.Articles {
		titleLower := strings.ToLower(article.Title)
		var matchTags []string
		var matchReasons []string

		for _, t := range topics {
			if strings.Contains(titleLower, strings.ToLower(t)) {
				matchTags = append(matchTags, t)
				matchReasons = append(matchReasons, "topic:"+t)
			}
		}
		for _, ts := range techStacks {
			if strings.Contains(titleLower, strings.ToLower(ts)) {
				matchTags = append(matchTags, ts)
				matchReasons = append(matchReasons, "tech:"+ts)
			}
		}
		for _, a := range authors {
			if strings.Contains(titleLower, strings.ToLower(a)) {
				matchTags = append(matchTags, a)
				matchReasons = append(matchReasons, "author:"+a)
			}
		}

		matchType := "none"
		if len(matchTags) >= 2 {
			matchType = "high"
		} else if len(matchTags) == 1 {
			matchType = "medium"
		}

		reason := ""
		if len(matchReasons) > 0 {
			reason = strings.Join(matchReasons, ", ")
		}

		matches = append(matches, MatchResult{
			HNID:        article.HNID,
			MatchType:   matchType,
			MatchTags:   matchTags,
			MatchReason: reason,
		})
	}

	c.JSON(http.StatusOK, BatchMatchResp{Matches: matches})
}

func (s *Server) handleRecommendations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	userID := s.getUserID(c)
	profile, err := s.buildProfile(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"recommendations": []interface{}{}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	stories, err := s.fetcher.FetchHNTopStories(ctx)
	if err != nil {
		log.Printf("fetch HN top stories failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch_failed"})
		return
	}

	topics, _ := profile["topics"].([]string)
	var recommendations []map[string]interface{}
	for _, story := range stories {
		titleLower := strings.ToLower(story.Title)
		var matchTags []string
		for _, t := range topics {
			if strings.Contains(titleLower, strings.ToLower(t)) {
				matchTags = append(matchTags, t)
			}
		}
		if len(matchTags) > 0 {
			recommendations = append(recommendations, map[string]interface{}{
				"hn_id":      story.HNID,
				"title":      story.Title,
				"url":        story.URL,
				"match_tags": matchTags,
				"reason":     "matches your interest in " + strings.Join(matchTags, ", "),
			})
		}
		if len(recommendations) >= limit {
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": recommendations})
}

func (s *Server) handlePreloadArticles(c *gin.Context) {
	var req struct {
		Articles []preloader.ArticleRequest `json:"articles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	max := intOr(s.config.PreloadMaxPerRequest, 20)
	if len(req.Articles) > max {
		req.Articles = req.Articles[:max]
	}

	userID := s.getUserID(c)
	queued, skipped := 0, 0
	quotaExceeded := false

	for _, article := range req.Articles {
		article.Lang = normalizeLang(article.Lang)
		if article.HNID <= 0 || strings.TrimSpace(article.Title) == "" ||
			len(article.Title) > 500 || len(article.URL) > 2048 {
			skipped++
			continue
		}

		if existing, err := s.repo.GetArticle(userID, article.HNID, article.Lang); err == nil && existing != nil && existing.Summary != "" {
			skipped++
			continue
		}
		if cached, err := s.repo.GetGlobalArticle(article.HNID, article.Lang); err == nil && cached != nil && cached.Summary != "" {
			if err := s.repo.SaveArticle(userID, cached); err != nil {
				log.Printf("save preloaded cached article failed: %v", err)
			}
			skipped++
			continue
		}

		reserved := false
		if userID != "" {
			_, _, ok, err := s.repo.ReserveQuota(userID, 1)
			if err != nil {
				s.internalError(c, err, "reserve quota failed")
				return
			}
			if !ok {
				quotaExceeded = true
				break
			}
			reserved = true
		}

		accepted, reason := s.preloader.SubmitForUser(userID, article, reserved)
		if !accepted {
			if reserved {
				if err := s.repo.ReleaseQuota(userID, 1); err != nil {
					log.Printf("release preload quota failed: %v", err)
				}
			}
			log.Printf("preload skipped hn_id=%d reason=%s", article.HNID, reason)
			skipped++
			continue
		}
		queued++
	}

	c.JSON(http.StatusOK, gin.H{
		"queued":         queued,
		"skipped":        skipped,
		"quota_exceeded": quotaExceeded,
	})
}

func (s *Server) handlePreloadStatus(c *gin.Context) {
	idsParam := c.Query("ids")
	lang := normalizeLang(c.DefaultQuery("lang", "zh"))
	if idsParam == "" {
		c.JSON(http.StatusOK, gin.H{"ready_ids": []int64{}})
		return
	}

	var hnIDs []int64
	for _, part := range strings.Split(idsParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		hnIDs = append(hnIDs, id)
	}

	userID := s.getUserID(c)
	readyIDs, err := s.preloader.IsCachedForUser(c.Request.Context(), userID, hnIDs, lang)
	if err != nil {
		s.internalError(c, err, "check preload cache failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ready_ids": readyIDs})
}

func (s *Server) handleAccountExport(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not_supported"})
		return
	}
	data, err := s.repo.GetUserData(userID)
	if err != nil {
		s.internalError(c, err, "export account failed")
		return
	}
	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="hn-reader-export.json"`)
	c.JSON(http.StatusOK, data)
}

func (s *Server) handleAccountDelete(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not_supported"})
		return
	}
	if err := s.repo.DeleteUser(userID); err != nil {
		s.internalError(c, err, "delete account failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- helpers ---

func (s *Server) llmTimeout() time.Duration {
	if s.config.LLMTimeout > 0 {
		return s.config.LLMTimeout
	}
	return 90 * time.Second
}

func normalizeLang(lang string) string {
	if strings.EqualFold(strings.TrimSpace(lang), "en") {
		return "en"
	}
	return "zh"
}

func (s *Server) writeQuotaExceeded(c *gin.Context, userID string, used, limit int) {
	reset := ""
	if u, err := s.repo.GetUserQuota(userID); err == nil && u != nil {
		reset = u.QuotaResetAt.UTC().Format(time.RFC3339)
	}
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":          "quota_exceeded",
		"quota_used":     used,
		"quota_limit":    limit,
		"quota_reset_at": reset,
	})
}

func (s *Server) writeLLMError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, summarizer.ErrBudgetExceeded), errors.Is(err, summarizer.ErrBusy):
		log.Printf("llm busy: %v", err)
		c.Header("Retry-After", "5")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":               "service_busy",
			"retry_after_seconds": 5,
		})
	case errors.Is(err, summarizer.ErrAuth):
		log.Printf("llm auth error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "llm_unavailable"})
	default:
		log.Printf("llm error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "llm_unavailable"})
	}
}

func (s *Server) internalError(c *gin.Context, err error, msg string) {
	log.Printf("%s: %v", msg, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
}

// getUserID returns the user ID from the context, or empty string for local mode.
func (s *Server) getUserID(c *gin.Context) string {
	u := getUser(c)
	if u == nil {
		return ""
	}
	return u.ID
}

// buildProfile extracts a preference profile from recent user signals.
func (s *Server) buildProfile(userID string) (map[string]interface{}, error) {
	signals, err := s.repo.GetRecentSignals(userID, 100)
	if err != nil {
		return nil, err
	}

	topicFreq := make(map[string]int)
	for _, sig := range signals {
		ctx := sig.Context
		title, _ := ctx["title"].(string)
		words := extractKeywords(title)
		for _, w := range words {
			topicFreq[w] += sig.Strength
		}
	}

	var topTopics []string
	for k := range topicFreq {
		if len(k) > 2 {
			topTopics = append(topTopics, k)
		}
	}
	if len(topTopics) > 20 {
		topTopics = topTopics[:20]
	}

	return map[string]interface{}{
		"topics":      topTopics,
		"tech_stacks": []string{},
		"authors":     []string{},
		"updated_at":  time.Now().Format(time.RFC3339),
	}, nil
}

func extractKeywords(text string) []string {
	var keywords []string
	lower := strings.ToLower(text)
	candidates := []string{
		"rust", "go", "golang", "python", "javascript", "typescript", "java", "c++", "zig", "nim",
		"ai", "machine learning", "deep learning", "llm", "gpt", "neural network",
		"kubernetes", "docker", "aws", "cloud", "devops", "linux",
		"database", "sql", "postgres", "redis", "sqlite",
		"startup", "funding", "vc", "business",
		"security", "crypto", "blockchain", "privacy",
		"web", "frontend", "backend", "api", "graphql", "rest",
		"performance", "optimization", "compiler", "os", "distributed",
	}
	for _, c := range candidates {
		if strings.Contains(lower, c) {
			keywords = append(keywords, c)
		}
	}
	return keywords
}
