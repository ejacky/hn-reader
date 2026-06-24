package storage

import "time"

// ========== Domain Types ==========

// User represents a registered (or anonymous) cloud user.
// Not used in local mode.
type User struct {
	ID           string
	Email        string
	APIKey       string
	Plan         string
	QuotaUsed    int
	QuotaLimit   int
	QuotaResetAt time.Time
	CreatedAt    time.Time
}

// Article represents a summarized article, shared by local and cloud.
// KeyPoints is stored as []string in memory; implementations serialize as needed.
type Article struct {
	ID             int64    `json:"id"`
	HNID           int64    `json:"hn_id"`
	URL            string   `json:"url"`
	Title          string   `json:"title"`
	FetchedContent string   `json:"fetched_content"` // raw page content (local only)
	Summary        string   `json:"summary"`
	KeyPoints      []string `json:"key_points"`
	Lang           string   `json:"lang"`
	Cached         bool     `json:"cached"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Comment represents a summarized Hacker News comment.
type Comment struct {
	ID        int64  `json:"id"`
	HNID      int64  `json:"hn_id"`
	ArticleID int64  `json:"article_id"`
	Author    string `json:"author"`
	RawText   string `json:"-"` // original comment text (local only)
	Summary   string `json:"summary"`
	Lang      string `json:"lang"`
	Cached    bool   `json:"cached"`
	CreatedAt time.Time
}

// UserSignal captures an implicit or explicit preference action.
type UserSignal struct {
	ID         int64                  `json:"id"`
	TargetType string                 `json:"target_type"`
	TargetID   int64                  `json:"target_id"`
	SignalType string                 `json:"signal_type"`
	Strength   int                    `json:"strength"`
	Context    map[string]interface{} `json:"context"`
	CreatedAt  time.Time
}

// SignalStats is a summary of saved signals.
type SignalStats struct {
	TotalSignals int            `json:"total_signals"`
	ByType       map[string]int `json:"by_type"`
}

// ========== Repository Interface ==========

// Repository is the storage abstraction used by both local (SQLite) and cloud
// (Postgres) deployments.  Methods that don't apply to a particular backend
// should return ErrNotSupported.
type Repository interface {
	// --- User (cloud) ---
	CreateAnonymousUser() (*User, error)
	GetUserByAPIKey(apiKey string) (*User, error)
	GetUserQuota(userID string) (*User, error)
	IncrementQuotaUsed(userID string, n int) error

	// --- Article ---
	GetArticle(userID string, hnID int64, lang string) (*Article, error)
	SaveArticle(userID string, a *Article) error
	GetArticlesByIDs(userID string, hnIDs []int64) ([]Article, error)

	// --- Comment ---
	GetComment(userID string, hnID int64, lang string) (*Comment, error)
	SaveComment(userID string, c *Comment) error

	// --- Signal ---
	SaveSignal(userID string, s *UserSignal) error
	GetSignalStats(userID string) (*SignalStats, error)
	GetRecentSignals(userID string, limit int) ([]UserSignal, error)

	// Close releases resources.
	Close() error
}
