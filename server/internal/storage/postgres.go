package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// PostgresRepo implements Repository against a PostgreSQL database.
type PostgresRepo struct {
	db *sql.DB
}

// NewPostgres opens a connection, configures the pool, and runs migrations.
func NewPostgres(databaseURL string) (*PostgresRepo, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

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
		email TEXT NOT NULL DEFAULT '',
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

	CREATE TABLE IF NOT EXISTS global_articles (
		hn_id BIGINT NOT NULL,
		lang TEXT NOT NULL,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		summary TEXT NOT NULL,
		key_points JSONB NOT NULL DEFAULT '[]',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (hn_id, lang)
	);

	CREATE INDEX IF NOT EXISTS idx_global_articles_updated ON global_articles(updated_at);
	`
	if _, err := r.db.Exec(schema); err != nil {
		return err
	}

	// Idempotent fix for databases created before the anonymous-auth fix:
	// users.email was nullable and scanning NULL into a Go string failed.
	if _, err := r.db.Exec(`UPDATE users SET email = '' WHERE email IS NULL`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`ALTER TABLE users ALTER COLUMN email SET DEFAULT ''`); err != nil {
		return err
	}
	if _, err := r.db.Exec(`ALTER TABLE users ALTER COLUMN email SET NOT NULL`); err != nil {
		return err
	}
	return nil
}

// --- User ---

func (r *PostgresRepo) CreateAnonymousUser(quotaLimit int) (*User, error) {
	if quotaLimit <= 0 {
		quotaLimit = 50
	}
	apiKey := uuid.New().String()
	row := r.db.QueryRow(`
		INSERT INTO users (api_key, plan, quota_limit)
		VALUES ($1, 'free', $2)
		RETURNING id, COALESCE(email, ''), api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
	`, apiKey, quotaLimit)
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.APIKey, &u.Plan, &u.QuotaUsed, &u.QuotaLimit, &u.QuotaResetAt, &u.CreatedAt)
	return u, err
}

func (r *PostgresRepo) GetUserByAPIKey(apiKey string) (*User, error) {
	row := r.db.QueryRow(`
		SELECT id, COALESCE(email, ''), api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
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
		SELECT id, COALESCE(email, ''), api_key, plan, quota_used, quota_limit, quota_reset_at, created_at
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

// ReserveQuota atomically adds n to quota_used only if it fits under the limit.
func (r *PostgresRepo) ReserveQuota(userID string, n int) (used, limit int, ok bool, err error) {
	if n <= 0 {
		u, err := r.GetUserQuota(userID)
		if err != nil || u == nil {
			return 0, 0, false, err
		}
		return u.QuotaUsed, u.QuotaLimit, true, nil
	}

	u, err := r.GetUserQuota(userID)
	if err != nil {
		return 0, 0, false, err
	}
	if u == nil {
		return 0, 0, false, fmt.Errorf("user not found")
	}
	if u.QuotaUsed+n > u.QuotaLimit {
		return u.QuotaUsed, u.QuotaLimit, false, nil
	}

	err = r.db.QueryRow(`
		UPDATE users SET quota_used = quota_used + $2
		WHERE id = $1 AND quota_used + $2 <= quota_limit
		RETURNING quota_used, quota_limit
	`, userID, n).Scan(&used, &limit)
	if err == sql.ErrNoRows {
		// Lost a race with another reservation; report the current state.
		return u.QuotaUsed, u.QuotaLimit, false, nil
	}
	return used, limit, err == nil, err
}

// ReleaseQuota refunds n reservations, never going below zero.
func (r *PostgresRepo) ReleaseQuota(userID string, n int) error {
	if n <= 0 {
		return nil
	}
	_, err := r.db.Exec(`
		UPDATE users SET quota_used = GREATEST(quota_used - $2, 0) WHERE id = $1
	`, userID, n)
	return err
}

func (r *PostgresRepo) DeleteUser(userID string) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (r *PostgresRepo) GetUserData(userID string) (*UserData, error) {
	user, err := r.getUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	data := &UserData{User: user, Articles: []Article{}, Comments: []Comment{}, Signals: []UserSignal{}}

	rows, err := r.db.Query(`
		SELECT hn_id, url, title, summary, key_points, lang, created_at
		FROM articles WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10000
	`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a Article
		var kp []byte
		if err := rows.Scan(&a.HNID, &a.URL, &a.Title, &a.Summary, &kp, &a.Lang, &a.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal(kp, &a.KeyPoints)
		data.Articles = append(data.Articles, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.Query(`
		SELECT hn_id, article_id, author, summary, lang, created_at
		FROM comments WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10000
	`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.HNID, &c.ArticleID, &c.Author, &c.Summary, &c.Lang, &c.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		data.Comments = append(data.Comments, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.Query(`
		SELECT target_type, target_id, signal_type, strength, context, created_at
		FROM user_signals WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10000
	`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s UserSignal
		var ctx []byte
		if err := rows.Scan(&s.TargetType, &s.TargetID, &s.SignalType, &s.Strength, &ctx, &s.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal(ctx, &s.Context)
		data.Signals = append(data.Signals, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return data, nil
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
	_ = json.Unmarshal(kpJSON, &a.KeyPoints)
	return a, nil
}

func (r *PostgresRepo) SaveArticle(userID string, a *Article) error {
	kpJSON, _ := json.Marshal(a.KeyPoints)
	_, err := r.db.Exec(`
		INSERT INTO articles (user_id, hn_id, url, title, summary, key_points, lang)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, hn_id, lang)
		DO UPDATE SET url = EXCLUDED.url, title = EXCLUDED.title, summary = EXCLUDED.summary, key_points = EXCLUDED.key_points
	`, userID, a.HNID, a.URL, a.Title, a.Summary, string(kpJSON), a.Lang)
	return err
}

func (r *PostgresRepo) GetArticlesByIDs(userID string, hnIDs []int64) ([]Article, error) {
	if len(hnIDs) == 0 {
		return nil, nil
	}
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
		_ = json.Unmarshal(kpJSON, &a.KeyPoints)
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// --- Global summary cache ---

func (r *PostgresRepo) GetGlobalArticle(hnID int64, lang string) (*Article, error) {
	row := r.db.QueryRow(`
		SELECT hn_id, url, title, summary, key_points, lang, updated_at
		FROM global_articles WHERE hn_id = $1 AND lang = $2
	`, hnID, lang)
	a := &Article{}
	var kp []byte
	err := row.Scan(&a.HNID, &a.URL, &a.Title, &a.Summary, &kp, &a.Lang, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Cached = true
	_ = json.Unmarshal(kp, &a.KeyPoints)
	return a, nil
}

func (r *PostgresRepo) SaveGlobalArticle(a *Article) error {
	kpJSON, _ := json.Marshal(a.KeyPoints)
	_, err := r.db.Exec(`
		INSERT INTO global_articles (hn_id, lang, url, title, summary, key_points)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (hn_id, lang)
		DO UPDATE SET url = EXCLUDED.url, title = EXCLUDED.title, summary = EXCLUDED.summary,
			key_points = EXCLUDED.key_points, updated_at = NOW()
	`, a.HNID, a.Lang, a.URL, a.Title, a.Summary, string(kpJSON))
	return err
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
	`, userID, s.TargetType, s.TargetID, s.SignalType, s.Strength, string(ctxJSON), s.CreatedAt)
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

	stats := &SignalStats{ByType: make(map[string]int)}
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
		_ = json.Unmarshal(ctxJSON, &s.Context)
		signals = append(signals, s)
	}
	return signals, nil
}

func (r *PostgresRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *PostgresRepo) Close() error {
	return r.db.Close()
}

var _ Repository = (*PostgresRepo)(nil)
