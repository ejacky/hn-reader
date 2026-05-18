package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hn-reader/internal/config"
	"hn-reader/internal/fetcher"
	"hn-reader/internal/storage"
	"hn-reader/internal/summarizer"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config     *config.Config
	storage    *storage.Storage
	summarizer *summarizer.Summarizer
	fetcher    *fetcher.Fetcher
	version    string
}

func New(cfg *config.Config, st *storage.Storage, version string) *Server {
	return &Server{
		config:     cfg,
		storage:    st,
		summarizer: summarizer.New(cfg),
		fetcher:    fetcher.New(),
		version:    version,
	}
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.handleHealth)
	r.POST("/api/summarize/article", s.handleSummarizeArticle)
	r.POST("/api/summarize/comment", s.handleSummarizeComment)
	r.POST("/api/signal", s.handleSignal)
	r.GET("/api/signal/stats", s.handleSignalStats)
	r.GET("/api/preference/profile", s.handlePreferenceProfile)
	r.POST("/api/match/batch", s.handleBatchMatch)
	r.GET("/api/recommendations", s.handleRecommendations)
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": s.version,
		"port":    s.config.Port,
	})
}

type SummarizeArticleReq struct {
	HNID  int64  `json:"hn_id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Lang  string `json:"lang"`
}

type SummarizeArticleResp struct {
	Summary    string   `json:"summary"`
	KeyPoints  []string `json:"key_points"`
	Cached     bool     `json:"cached"`
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

	existing, err := s.storage.GetArticleByHNID(req.HNID, req.Lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing != nil && existing.Summary != "" {
		var kps []string
		if existing.KeyPoints != "" {
			json.Unmarshal([]byte(existing.KeyPoints), &kps)
		}
		c.JSON(http.StatusOK, SummarizeArticleResp{
			Summary:   existing.Summary,
			KeyPoints: kps,
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
	if err := s.storage.SaveArticle(article); err != nil {
		log.Printf("save article failed: %v", err)
	}

	c.JSON(http.StatusOK, SummarizeArticleResp{
		Summary:   summary,
		KeyPoints: keyPoints,
		Cached:    false,
	})
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

func (s *Server) handleSummarizeComment(c *gin.Context) {
	var req SummarizeCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Lang == "" {
		req.Lang = "zh"
	}

	existing, err := s.storage.GetCommentByHNID(req.HNID, req.Lang)
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

	comment := &storage.Comment{
		HNID:      req.HNID,
		ArticleID: req.ArticleID,
		Author:    req.Author,
		RawText:   req.Text,
		Summary:   summary,
		Lang:      req.Lang,
	}
	if err := s.storage.SaveComment(comment); err != nil {
		log.Printf("save comment failed: %v", err)
	}

	c.JSON(http.StatusOK, SummarizeCommentResp{Summary: summary, Cached: false})
}

type SignalReq struct {
	TargetType string                 `json:"target_type"`
	TargetID   int64                  `json:"target_id"`
	SignalType string                 `json:"signal_type"`
	Strength   int                    `json:"strength"`
	Context    map[string]interface{} `json:"context"`
}

func (s *Server) handleSignal(c *gin.Context) {
	var req SignalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctxJSON, _ := json.Marshal(req.Context)
	signal := &storage.UserSignal{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		SignalType: req.SignalType,
		Strength:   req.Strength,
		Context:    string(ctxJSON),
	}
	if err := s.storage.SaveSignal(signal); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleSignalStats(c *gin.Context) {
	stats, err := s.storage.GetSignalStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handlePreferenceProfile(c *gin.Context) {
	profile, err := buildProfile(s.storage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
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

func (s *Server) handleBatchMatch(c *gin.Context) {
	var req BatchMatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile, err := buildProfile(s.storage)
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

	profile, err := buildProfile(s.storage)
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

func buildProfile(st *storage.Storage) (map[string]interface{}, error) {
	rows, err := st.Query(`
		SELECT a.title, a.fetched_content, s.strength, s.signal_type
		FROM user_signals s
		JOIN articles a ON s.target_id = a.hn_id
		WHERE s.target_type = 'article'
		ORDER BY s.created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topicFreq := make(map[string]int)
	authorFreq := make(map[string]int)
	for rows.Next() {
		var title, content string
		var strength int
		var signalType string
		if err := rows.Scan(&title, &content, &strength, &signalType); err != nil {
			continue
		}
		words := extractKeywords(title + " " + content)
		for _, w := range words {
			topicFreq[w] += strength
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
		"authors":     authorFreqToList(authorFreq),
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

func authorFreqToList(freq map[string]int) []string {
	var authors []string
	for a := range freq {
		authors = append(authors, a)
	}
	return authors
}

func (s *Server) fetchContent(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text := string(body)
	if len(text) > 10000 {
		text = text[:10000]
	}
	return text, nil
}
