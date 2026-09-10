package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Fetcher struct {
	client *http.Client
}

type HNStory struct {
	HNID  int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

func New() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			// 15s is too tight for slow sites; allow 45s before giving up.
			Timeout: 45 * time.Second,
		},
	}
}

func (f *Fetcher) FetchArticle(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty url")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
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

func (f *Fetcher) FetchHNTopStories() ([]HNStory, error) {
	resp, err := f.client.Get("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
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

	var stories []HNStory
	for _, id := range ids {
		story, err := f.fetchHNItem(id)
		if err != nil {
			continue
		}
		if story.URL != "" {
			stories = append(stories, *story)
		}
	}
	return stories, nil
}

func (f *Fetcher) fetchHNItem(id int64) (*HNStory, error) {
	url := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id)
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var item struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	if item.Type != "story" {
		return nil, fmt.Errorf("not a story")
	}
	return &HNStory{HNID: item.ID, Title: item.Title, URL: item.URL}, nil
}
