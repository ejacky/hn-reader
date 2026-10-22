package storage

import (
	"context"
	"time"
)

// ========== Domain Types ==========

// User represents a registered (or anonymous) cloud user.
// Not used in local mode.
type User struct {
	ID           string
	Email        string
	APIKey       string `json:"-"`
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

// UserData is the account export payload. Global summary cache entries are
// intentionally excluded: they are public article content, not user data.
type UserData struct {
	User     *User        `json:"user"`
	Articles []Article    `json:"articles"`
	Comments []Comment    `json:"comments"`
	Signals  []UserSignal `json:"signals"`
}

// ========== Repository Interface ==========

// Repository is the storage abstraction used by both local (SQLite) and cloud
// (Postgres) deployments. Methods that don't apply to a particular backend
// should return ErrNotSupported.
type Repository interface {
	// --- User (cloud) ---
	CreateAnonymousUser(quotaLimit int) (*User, error)
	GetUserByAPIKey(apiKey string) (*User, error)
	GetUserQuota(userID string) (*User, error)
	IncrementQuotaUsed(userID string, n int) error
	ReserveQuota(userID string, n int) (used, limit int, ok bool, err error)
	ReleaseQuota(userID string, n int) error
	GetUserData(userID string) (*UserData, error)
	DeleteUser(userID string) error

	// --- Article ---
	GetArticle(userID string, hnID int64, lang string) (*Article, error)
	SaveArticle(userID string, a *Article) error
	GetArticlesByIDs(userID string, hnIDs []int64) ([]Article, error)

	// --- Global summary cache (cloud) ---
	GetGlobalArticle(hnID int64, lang string) (*Article, error)
	SaveGlobalArticle(a *Article) error

	// --- Comment ---
	GetComment(userID string, hnID int64, lang string) (*Comment, error)
	SaveComment(userID string, c *Comment) error

	// --- Signal ---
	SaveSignal(userID string, s *UserSignal) error
	GetSignalStats(userID string) (*SignalStats, error)
	GetRecentSignals(userID string, limit int) ([]UserSignal, error)

	// Ping verifies the database is reachable.
	Ping(ctx context.Context) error

	// Close releases resources.
	Close() error
}
