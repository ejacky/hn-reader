package api

import (
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
}

// NewLocal creates a Server for the local desktop deployment (SQLite, no auth).
func NewLocal(cfg *config.Config, repo storage.Repository, version string) *Server {
	s := &Server{
		config: cfg,
		repo:   repo,
		summarizer: summarizer.New(&summarizer.Config{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
		}),
		fetcher: fetcher.New(),
		version: version,
	}
	s.preloader = preloader.New(repo, s.fetcher, s.summarizer, 3)
	return s
}

// NewCloud creates a Server for the cloud deployment (Postgres, multi-user, auth).
func NewCloud(cfg *config.Config, repo storage.Repository, version string) *Server {
	s := &Server{
		config: cfg,
		repo:   repo,
		summarizer: summarizer.New(&summarizer.Config{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
		}),
		fetcher: fetcher.New(),
		version: version,
	}
	s.preloader = preloader.New(repo, s.fetcher, s.summarizer, 3)
	return s
}

// RegisterRoutes sets up all HTTP routes.  Auth middleware (if any) is injected
// by the caller before this is called (for cloud) or is a no-op (for local).
func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.handleHealth)
	r.POST("/auth/anonymous", s.handleAnonymousAuth)

	api := r.Group("/api")
	api.Use(s.authMiddleware())
	api.Use(s.quotaMiddleware())
	{
		api.POST("/summarize/article", s.handleSummarizeArticle)
		api.POST("/summarize/comment", s.handleSummarizeComment)
		api.POST("/summarize/article/preload", s.handlePreloadArticles)
		api.GET("/summarize/article/preload-status", s.handlePreloadStatus)
		api.POST("/signal", s.handleSignal)
		api.GET("/signal/stats", s.handleSignalStats)
		api.GET("/preference/profile", s.handlePreferenceProfile)
		api.POST("/match/batch", s.handleBatchMatch)
		api.GET("/recommendations", s.handleRecommendations)
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
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": s.version,
		"port":    s.config.Port,
	})
}

func (s *Server) handleAnonymousAuth(c *gin.Context) {
	user, err := s.repo.CreateAnonymousUser()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	c.JSON(http.StatusOK, gin.H{
		"email":       user.Email,
		"plan":        user.Plan,
		"quota_used":  user.QuotaUsed,
		"quota_limit": user.QuotaLimit,
	})
}

func (s *Server) handleSummarizeArticle(c *gin.Context) {
	var req SummarizeArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Lang == "" {
		req.Lang = "zh"
	}

	userID := s.getUserID(c)
	existing, err := s.repo.GetArticle(userID, req.HNID, req.Lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing != nil && existing.Summary != "" {
		c.JSON(http.StatusOK, SummarizeArticleResp{
			Summary:   existing.Summary,
			KeyPoints: existing.KeyPoints,
			Cached:    true,
		})
		return
	}

	content, err := s.fetcher.FetchArticle(req.URL)
	if err != nil {
		log.Printf("fetch article failed: %v", err)
		content = ""
	}

	summary, keyPoints, err := s.summarizer.SummarizeArticle(req.Title, content, req.Lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.chargeQuota(c, 1)

	article := &storage.Article{
		HNID:           req.HNID,
		URL:            req.URL,
		Title:          req.Title,
		FetchedContent: content,
		Summary:        summary,
		KeyPoints:      keyPoints,
		Lang:           req.Lang,
	}
	if err := s.repo.SaveArticle(userID, article); err != nil {
		log.Printf("save article failed: %v", err)
	}

	c.JSON(http.StatusOK, SummarizeArticleResp{
		Summary:   summary,
		KeyPoints: keyPoints,
		Cached:    false,
	})
}

func (s *Server) handleSummarizeComment(c *gin.Context) {
	var req SummarizeCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Lang == "" {
		req.Lang = "zh"
	}

	userID := s.getUserID(c)
	existing, err := s.repo.GetComment(userID, req.HNID, req.Lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing != nil && existing.Summary != "" {
		c.JSON(http.StatusOK, SummarizeCommentResp{Summary: existing.Summary, Cached: true})
		return
	}

	summary, err := s.summarizer.SummarizeComment(req.Text, req.Lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.chargeQuota(c, 1)

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

	c.JSON(http.StatusOK, SummarizeCommentResp{Summary: summary, Cached: false})
}

func (s *Server) handleSignal(c *gin.Context) {
	var req SignalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleSignalStats(c *gin.Context) {
	userID := s.getUserID(c)
	stats, err := s.repo.GetSignalStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handlePreferenceProfile(c *gin.Context) {
	userID := s.getUserID(c)
	profile, err := s.buildProfile(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (s *Server) handleBatchMatch(c *gin.Context) {
	var req BatchMatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	stories, err := s.fetcher.FetchHNTopStories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := s.getUserID(c)
	for _, article := range req.Articles {
		s.preloader.SubmitForUser(userID, article)
	}

	c.JSON(http.StatusOK, gin.H{"queued": len(req.Articles)})
}

func (s *Server) handlePreloadStatus(c *gin.Context) {
	idsParam := c.Query("ids")
	lang := c.DefaultQuery("lang", "zh")
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
		if err != nil {
			continue
		}
		hnIDs = append(hnIDs, id)
	}

	userID := s.getUserID(c)
	readyIDs, err := s.preloader.IsCachedForUser(userID, hnIDs, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ready_ids": readyIDs})
}

// --- helpers ---

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
