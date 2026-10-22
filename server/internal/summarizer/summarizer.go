package summarizer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	// ErrBudgetExceeded means the process-wide daily LLM call budget is used up.
	ErrBudgetExceeded = errors.New("daily LLM budget exceeded")
	// ErrBusy means the global LLM concurrency limit could not be acquired.
	ErrBusy = errors.New("LLM concurrency limit reached")
	// ErrAuth means the provider rejected the API key.
	ErrAuth = errors.New("LLM authentication failed")
	// ErrProvider means the provider returned an error after retries.
	ErrProvider = errors.New("LLM provider error")
)

// Config holds LLM connection parameters and cost guards.
type Config struct {
	BaseURL         string
	APIKey          string
	Model           string
	MaxConcurrency  int
	DailyBudget     int
	Timeout         time.Duration
	MaxOutputTokens int
}

type Summarizer struct {
	config    *Config
	client    *http.Client
	sessionID string

	sem chan struct{}

	mu   sync.Mutex
	day  string
	used int
	now  func() time.Time
}

func New(cfg *Config) *Summarizer {
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 4
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = 600
	}
	return &Summarizer{
		config:    cfg,
		client:    &http.Client{Timeout: cfg.Timeout},
		sessionID: newSessionID(),
		sem:       make(chan struct{}, cfg.MaxConcurrency),
		now:       time.Now,
	}
}

// newSessionID returns a random hex ID sent as the x-opencode-session header.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("hn-reader-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (s *Summarizer) acquireDaily() error {
	if s.config.DailyBudget <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	day := s.now().UTC().Format("2006-01-02")
	if s.day != day {
		s.day = day
		s.used = 0
	}
	if s.used >= s.config.DailyBudget {
		return ErrBudgetExceeded
	}
	s.used++
	if s.config.DailyBudget >= 10 && s.used == s.config.DailyBudget*8/10 {
		log.Printf("llm: daily budget at 80%% used=%d limit=%d", s.used, s.config.DailyBudget)
	}
	return nil
}

func (s *Summarizer) SummarizeArticle(ctx context.Context, title, content, lang string) (string, []string, error) {
	if content == "" {
		content = title
	}
	if len(content) > 10000 {
		content = content[:10000]
	}

	langInstruction := "请用中文回答。"
	if lang == "en" {
		langInstruction = "Please respond in English."
	}

	prompt := fmt.Sprintf(`You are a reading assistant. Summarize the following article in a concise and informative way.

Title: %s

Content:
%s

Please provide:
1. A one-paragraph summary (2-3 sentences) highlighting the core value and key takeaway.
2. 3-5 key points as a bulleted list.

Format your response as:
Summary: <summary text>
Key Points:
- <point 1>
- <point 2>
- <point 3>

%s`, title, content, langInstruction)

	respText, err := s.callLLM(ctx, prompt)
	if err != nil {
		return "", nil, err
	}

	summary, keyPoints := parseResponse(respText)
	summary = strings.TrimSpace(summary)
	if summary == "" {
		log.Printf("llm: empty summary, raw response=%q", truncate(respText, 300))
		return "", nil, fmt.Errorf("%w: empty summary", ErrProvider)
	}
	if keyPoints == nil {
		keyPoints = []string{}
	}
	return summary, keyPoints, nil
}

func (s *Summarizer) SummarizeComment(ctx context.Context, text, lang string) (string, error) {
	if len(text) > 3000 {
		text = text[:3000]
	}

	langInstruction := "请用中文回答。"
	if lang == "en" {
		langInstruction = "Please respond in English."
	}

	prompt := fmt.Sprintf(`You are a reading assistant. Summarize the following Hacker News comment into 1-2 sentences capturing the core point or insight.

Comment:
%s

Summary:
%s`, text, langInstruction)

	respText, err := s.callLLM(ctx, prompt)
	if err != nil {
		return "", err
	}

	respText = strings.TrimPrefix(respText, "Summary:")
	respText = strings.TrimSpace(respText)
	if respText == "" {
		return "", fmt.Errorf("%w: empty comment summary", ErrProvider)
	}
	return respText, nil
}

func (s *Summarizer) callLLM(ctx context.Context, prompt string) (string, error) {
	if s.config.APIKey == "" {
		return "", fmt.Errorf("%w: API key not configured", ErrAuth)
	}
	if err := s.acquireDaily(); err != nil {
		return "", err
	}

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		return "", ErrBusy
	}

	reqBody := map[string]interface{}{
		"model": s.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.5,
	}
	if s.config.MaxOutputTokens > 0 {
		reqBody["max_tokens"] = s.config.MaxOutputTokens
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
		req.Header.Set("User-Agent", "hn-reader/1.0")
		req.Header.Set("x-opencode-session", s.sessionID)

		resp, err := s.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("%w: HTTP %d", ErrAuth, resp.StatusCode)
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%w: HTTP %d", ErrProvider, resp.StatusCode)
			if attempt == 0 {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(500 * time.Millisecond):
				}
				continue
			}
			return "", lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("%w: HTTP %d: %s", ErrProvider, resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}
		if result.Error != nil {
			return "", fmt.Errorf("%w: %s", ErrProvider, result.Error.Message)
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("%w: no response choices", ErrProvider)
		}
		return result.Choices[0].Message.Content, nil
	}
	if lastErr == nil {
		lastErr = ErrProvider
	}
	return "", lastErr
}

func parseResponse(text string) (string, []string) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return "", nil
	}

	// Some OpenAI-compatible providers return a JSON object even when asked
	// for plain text.
	var jr struct {
		Summary   string   `json:"summary"`
		KeyPoints []string `json:"key_points"`
	}
	if json.Unmarshal([]byte(raw), &jr) == nil && strings.TrimSpace(jr.Summary) != "" {
		if jr.KeyPoints == nil {
			jr.KeyPoints = []string{}
		}
		return strings.TrimSpace(jr.Summary), jr.KeyPoints
	}

	lines := strings.Split(raw, "\n")
	var summary strings.Builder
	var keyPoints []string
	inKeyPoints := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		stripped := stripMarkdown(line)
		label, content := splitLabel(stripped)
		lower := strings.ToLower(label)

		if isSummaryLabel(lower) {
			if content != "" {
				summary.WriteString(content)
				summary.WriteString(" ")
			}
			inKeyPoints = false
			continue
		}
		if isKeyPointsLabel(lower) || strings.Contains(strings.ToLower(stripped), "key points") {
			inKeyPoints = true
			if content != "" {
				if p, ok := bulletText(content); ok && p != "" {
					keyPoints = append(keyPoints, p)
				}
			}
			continue
		}

		if inKeyPoints {
			if p, ok := bulletText(stripped); ok {
				if p != "" {
					keyPoints = append(keyPoints, p)
				}
			} else if len(keyPoints) > 0 {
				keyPoints[len(keyPoints)-1] += " " + stripped
			} else {
				keyPoints = append(keyPoints, stripped)
			}
			continue
		}

		summary.WriteString(stripped)
		summary.WriteString(" ")
	}

	out := strings.TrimSpace(summary.String())
	if out == "" {
		// Last resort: never return an empty summary for a successful call.
		out = raw
	}
	if keyPoints == nil {
		keyPoints = []string{}
	}
	return out, keyPoints
}

func isSummaryLabel(label string) bool {
	switch label {
	case "summary", "摘要", "总结":
		return true
	}
	return false
}

func isKeyPointsLabel(label string) bool {
	switch label {
	case "key points", "keypoints", "key takeaways", "要点", "关键点", "重点":
		return true
	}
	return false
}

func splitLabel(s string) (label, content string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ":："); i >= 0 {
		_, size := utf8.DecodeRuneInString(s[i:])
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+size:])
	}
	return strings.TrimSpace(s), ""
}

func stripMarkdown(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "#> ")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func bulletText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"- ", "* ", "• "} {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimSpace(s[len(prefix):]), true
		}
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		return strings.TrimSpace(s[i+1:]), true
	}
	return "", false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
