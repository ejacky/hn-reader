package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// PostgresRepo implements Repository against a PostgreSQL database.
type PostgresRepo struct {
	db *sql.DB
}

// NewPostgres opens a connection and runs migrations.
func NewPostgres(databaseURL string) (*PostgresRepo, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	repo := &PostgresRepo{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *PostgresRepo) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT,
		api_key TEXT UNIQUE NOT NULL,
		plan TEXT NOT NULL DEFAULT 'free',
		quota_used INTEGER NOT NULL DEFAULT 0,
		quota_limit INTEGER NOT NULL DEFAULT 50,
		quota_reset_at TIMESTAMP NOT NULL DEFAULT NOW(),
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);

	CREATE TABLE IF NOT EXISTS articles (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		hn_id BIGINT NOT NULL,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		summary TEXT NOT NULL,
		key_points JSONB NOT NULL DEFAULT '[]',
		lang TEXT NOT NULL DEFAULT 'zh',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(user_id, hn_id, lang)
	);

	CREATE INDEX IF NOT EXISTS idx_articles_user_hn_lang ON articles(user_id, hn_id, lang);

	CREATE TABLE IF NOT EXISTS comments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		hn_id BIGINT NOT NULL,
		article_id BIGINT NOT NULL,
		author TEXT NOT NULL,
		summary TEXT NOT NULL,
		lang TEXT NOT NULL DEFAULT 'zh',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(user_id, hn_id, lang)
	);

	CREATE INDEX IF NOT EXISTS idx_comments_user_hn_lang ON comments(user_id, hn_id, lang);

	CREATE TABLE IF NOT EXISTS user_signals (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		target_type TEXT NOT NULL,
		target_id BIGINT NOT NULL,
		signal_type TEXT NOT NULL,
		strength INTEGER NOT NULL,
		context JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_signals_user_created ON user_signals(user_id, created_at DESC);
	`
	_, err := r.db.Exec(schema)
	return err
}

// --- User ---

func (r *PostgresRepo) CreateAnonymousUser() (*User, error) {
	apiKey := uuid.New().String()
	row := r.db.QueryRow(`
		INSERT INTO users (api_key, plan, quota_limit)
		VALUES ($1, 'free', 50)
		RETURNING id, email, api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
	`, apiKey)
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.APIKey, &u.Plan, &u.QuotaUsed, &u.QuotaLimit, &u.QuotaResetAt, &u.CreatedAt)
	return u, err
}

func (r *PostgresRepo) GetUserByAPIKey(apiKey string) (*User, error) {
	row := r.db.QueryRow(`
		SELECT id, email, api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
		FROM users WHERE api_key = $1
	`, apiKey)
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.APIKey, &u.Plan, &u.QuotaUsed, &u.QuotaLimit, &u.QuotaResetAt, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *PostgresRepo) GetUserQuota(userID string) (*User, error) {
	// Reset quota if crossing a month boundary.
	_, _ = r.db.Exec(`
		UPDATE users
		SET quota_used = 0, quota_reset_at = NOW()
		WHERE id = $1 AND quota_reset_at < DATE_TRUNC('month', NOW())
	`, userID)
	return r.getUserByID(userID)
}

func (r *PostgresRepo) getUserByID(userID string) (*User, error) {
	row := r.db.QueryRow(`
		SELECT id, email, api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
		FROM users WHERE id = $1
	`, userID)
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.APIKey, &u.Plan, &u.QuotaUsed, &u.QuotaLimit, &u.QuotaResetAt, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *PostgresRepo) IncrementQuotaUsed(userID string, n int) error {
	_, err := r.db.Exec(`UPDATE users SET quota_used = quota_used + $2 WHERE id = $1`, userID, n)
	return err
}

// --- Article ---

func (r *PostgresRepo) GetArticle(userID string, hnID int64, lang string) (*Article, error) {
	row := r.db.QueryRow(`
		SELECT hn_id, url, title, summary, key_points, lang
		FROM articles
		WHERE user_id = $1 AND hn_id = $2 AND lang = $3
	`, userID, hnID, lang)
	a := &Article{}
	var kpJSON []byte
	err := row.Scan(&a.HNID, &a.URL, &a.Title, &a.Summary, &kpJSON, &a.Lang)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Cached = true
	json.Unmarshal(kpJSON, &a.KeyPoints)
	return a, nil
}

func (r *PostgresRepo) SaveArticle(userID string, a *Article) error {
	kpJSON, _ := json.Marshal(a.KeyPoints)
	_, err := r.db.Exec(`
		INSERT INTO articles (user_id, hn_id, url, title, summary, key_points, lang)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, hn_id, lang)
		DO UPDATE SET url = EXCLUDED.url, title = EXCLUDED.title, summary = EXCLUDED.summary, key_points = EXCLUDED.key_points
	`, userID, a.HNID, a.URL, a.Title, a.Summary, kpJSON, a.Lang)
	return err
}

func (r *PostgresRepo) GetArticlesByIDs(userID string, hnIDs []int64) ([]Article, error) {
	if len(hnIDs) == 0 {
		return nil, nil
	}
	// Build IN clause; Postgres params are positional ($1, $2, ...)
	query := `SELECT hn_id, url, title, summary, key_points, lang
		FROM articles WHERE user_id = $1 AND hn_id IN (`
	args := make([]interface{}, len(hnIDs)+1)
	args[0] = userID
	for i, id := range hnIDs {
		if i > 0 {
			query += ","
		}
		query += fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}
	query += ")"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		var kpJSON []byte
		if err := rows.Scan(&a.HNID, &a.URL, &a.Title, &a.Summary, &kpJSON, &a.Lang); err != nil {
			return nil, err
		}
		json.Unmarshal(kpJSON, &a.KeyPoints)
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// --- Comment ---

func (r *PostgresRepo) GetComment(userID string, hnID int64, lang string) (*Comment, error) {
	row := r.db.QueryRow(`
		SELECT hn_id, article_id, author, summary, lang
		FROM comments
		WHERE user_id = $1 AND hn_id = $2 AND lang = $3
	`, userID, hnID, lang)
	c := &Comment{}
	err := row.Scan(&c.HNID, &c.ArticleID, &c.Author, &c.Summary, &c.Lang)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Cached = true
	return c, nil
}

func (r *PostgresRepo) SaveComment(userID string, c *Comment) error {
	_, err := r.db.Exec(`
		INSERT INTO comments (user_id, hn_id, article_id, author, summary, lang)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, hn_id, lang)
		DO UPDATE SET article_id = EXCLUDED.article_id, author = EXCLUDED.author, summary = EXCLUDED.summary
	`, userID, c.HNID, c.ArticleID, c.Author, c.Summary, c.Lang)
	return err
}

// --- Signal ---

func (r *PostgresRepo) SaveSignal(userID string, s *UserSignal) error {
	ctxJSON, _ := json.Marshal(s.Context)
	_, err := r.db.Exec(`
		INSERT INTO user_signals (user_id, target_type, target_id, signal_type, strength, context, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userID, s.TargetType, s.TargetID, s.SignalType, s.Strength, ctxJSON, s.CreatedAt)
	return err
}

func (r *PostgresRepo) GetSignalStats(userID string) (*SignalStats, error) {
	rows, err := r.db.Query(`
		SELECT signal_type, COUNT(*)
		FROM user_signals
		WHERE user_id = $1
		GROUP BY signal_type
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &SignalStats{
		TotalSignals: 0,
		ByType:       make(map[string]int),
	}
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			continue
		}
		stats.ByType[t] = c
		stats.TotalSignals += c
	}
	return stats, nil
}

func (r *PostgresRepo) GetRecentSignals(userID string, limit int) ([]UserSignal, error) {
	rows, err := r.db.Query(`
		SELECT target_type, target_id, signal_type, strength, context, created_at
		FROM user_signals
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []UserSignal
	for rows.Next() {
		s := UserSignal{}
		var ctxJSON []byte
		err := rows.Scan(&s.TargetType, &s.TargetID, &s.SignalType, &s.Strength, &ctxJSON, &s.CreatedAt)
		if err != nil {
			continue
		}
		json.Unmarshal(ctxJSON, &s.Context)
		signals = append(signals, s)
	}
	return signals, nil
}

// Close closes the database connection.
func (r *PostgresRepo) Close() error {
	return r.db.Close()
}

// Compile-time interface check.
var _ Repository = (*PostgresRepo)(nil)
