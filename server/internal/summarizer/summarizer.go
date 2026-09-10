package summarizer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds LLM connection parameters.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

type Summarizer struct {
	config    *Config
	client    *http.Client
	sessionID string
}

func New(cfg *Config) *Summarizer {
	return &Summarizer{
		config:    cfg,
		client:    &http.Client{Timeout: 60 * time.Second},
		sessionID: newSessionID(),
	}
}

// newSessionID returns a random hex ID sent as the x-opencode-session header.
// OpenCode Go requires this header to route requests efficiently (see
// https://opencode.ai/docs/go/#where-can-i-use-it); a per-process ID keeps it
// stable for the lifetime of the server.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("hn-reader-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (s *Summarizer) SummarizeArticle(title, content, lang string) (string, []string, error) {
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

	respText, err := s.callLLM(prompt)
	if err != nil {
		return "", nil, err
	}

	summary, keyPoints := parseResponse(respText)
	return summary, keyPoints, nil
}

func (s *Summarizer) SummarizeComment(text, lang string) (string, error) {
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

	respText, err := s.callLLM(prompt)
	if err != nil {
		return "", err
	}

	respText = strings.TrimPrefix(respText, "Summary:")
	respText = strings.TrimSpace(respText)
	return respText, nil
}

func (s *Summarizer) callLLM(prompt string) (string, error) {
	if s.config.APIKey == "" {
		return "", fmt.Errorf("LLM API key not configured")
	}

	reqBody := map[string]interface{}{
		"model": s.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.5,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.config.BaseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	// OpenCode Go requires clients to identify themselves with their own
	// user agent and to send a stable x-opencode-session per session so
	// requests can be routed efficiently (https://opencode.ai/docs/go/).
	// Other providers ignore these headers.
	req.Header.Set("User-Agent", "hn-reader/1.0")
	req.Header.Set("x-opencode-session", s.sessionID)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("LLM API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	return result.Choices[0].Message.Content, nil
}

func parseResponse(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	var summary strings.Builder
	var keyPoints []string
	inKeyPoints := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "summary:") {
			summary.WriteString(strings.TrimPrefix(line, "Summary:"))
			summary.WriteString(" ")
			continue
		}
		if strings.Contains(lower, "key points") {
			inKeyPoints = true
			continue
		}
		if inKeyPoints && strings.HasPrefix(line, "-") {
			keyPoints = append(keyPoints, strings.TrimSpace(strings.TrimPrefix(line, "-")))
		} else if inKeyPoints && strings.HasPrefix(line, "*") {
			keyPoints = append(keyPoints, strings.TrimSpace(strings.TrimPrefix(line, "*")))
		} else if inKeyPoints {
			keyPoints = append(keyPoints, line)
		} else {
			summary.WriteString(line)
			summary.WriteString(" ")
		}
	}

	return strings.TrimSpace(summary.String()), keyPoints
}
