package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SqliteRepo implements Repository backed by a local SQLite database.
// It is single-user: all userID parameters are ignored.
type SqliteRepo struct {
	db *sql.DB
}

// NewSqlite opens (or creates) the SQLite database and runs migrations.
func NewSqlite(dbPath string) (*SqliteRepo, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &SqliteRepo{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// DB exposes the underlying *sql.DB for raw queries needed by the local API.
func (s *SqliteRepo) DB() *sql.DB { return s.db }

// Exec runs a raw SQL statement.
func (s *SqliteRepo) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

// Query runs a raw SQL query.
func (s *SqliteRepo) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

// QueryRow runs a raw SQL query returning at most one row.
func (s *SqliteRepo) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(query, args...)
}

// Begin starts a transaction.
func (s *SqliteRepo) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}

// --- User (all return ErrNotSupported for local mode) ---

func (s *SqliteRepo) CreateAnonymousUser(quotaLimit int) (*User, error) {
	return nil, fmt.Errorf("user management is not supported in local mode")
}

func (s *SqliteRepo) GetUserByAPIKey(apiKey string) (*User, error) {
	return nil, fmt.Errorf("user management is not supported in local mode")
}

func (s *SqliteRepo) GetUserQuota(userID string) (*User, error) {
	return nil, fmt.Errorf("user management is not supported in local mode")
}

func (s *SqliteRepo) IncrementQuotaUsed(userID string, n int) error {
	return fmt.Errorf("user management is not supported in local mode")
}

func (s *SqliteRepo) ReserveQuota(userID string, n int) (used, limit int, ok bool, err error) {
	// Local mode is single-user and has no quota.
	return 0, 0, true, nil
}

func (s *SqliteRepo) ReleaseQuota(userID string, n int) error { return nil }

func (s *SqliteRepo) GetUserData(userID string) (*UserData, error) {
	return nil, fmt.Errorf("account export is not supported in local mode")
}

func (s *SqliteRepo) DeleteUser(userID string) error {
	return fmt.Errorf("account deletion is not supported in local mode")
}

func (s *SqliteRepo) GetGlobalArticle(hnID int64, lang string) (*Article, error) {
	// Local mode is single-user, so the per-user cache is the global cache.
	return s.GetArticle("", hnID, lang)
}

func (s *SqliteRepo) SaveGlobalArticle(a *Article) error {
	return s.SaveArticle("", a)
}

func (s *SqliteRepo) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// --- Article ---

func (s *SqliteRepo) GetArticle(userID string, hnID int64, lang string) (*Article, error) {
	row := s.db.QueryRow(
		`SELECT id, hn_id, url, title, fetched_content, summary, key_points, lang, created_at, updated_at
		FROM articles WHERE hn_id = ? AND lang = ?`, hnID, lang)
	var a Article
	var kpJSON string
	err := row.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &kpJSON, &a.Lang, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(kpJSON), &a.KeyPoints)
	return &a, nil
}

func (s *SqliteRepo) SaveArticle(userID string, a *Article) error {
	kpJSON, _ := json.Marshal(a.KeyPoints)
	_, err := s.db.Exec(
		`INSERT INTO articles (hn_id, url, title, fetched_content, summary, key_points, lang, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hn_id, lang) DO UPDATE SET
			url=excluded.url, title=excluded.title, fetched_content=excluded.fetched_content,
			summary=excluded.summary, key_points=excluded.key_points, updated_at=excluded.updated_at`,
		a.HNID, a.URL, a.Title, a.FetchedContent, a.Summary, string(kpJSON), a.Lang, time.Now(),
	)
	return err
}

// --- Comment ---

func (s *SqliteRepo) GetComment(userID string, hnID int64, lang string) (*Comment, error) {
	row := s.db.QueryRow(
		`SELECT id, hn_id, article_id, author, raw_text, summary, lang, created_at
		FROM comments WHERE hn_id = ? AND lang = ?`, hnID, lang)
	var c Comment
	err := row.Scan(&c.ID, &c.HNID, &c.ArticleID, &c.Author, &c.RawText, &c.Summary, &c.Lang, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *SqliteRepo) SaveComment(userID string, c *Comment) error {
	_, err := s.db.Exec(
		`INSERT INTO comments (hn_id, article_id, author, raw_text, summary, lang)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(hn_id, lang) DO UPDATE SET
			article_id=excluded.article_id, author=excluded.author,
			raw_text=excluded.raw_text, summary=excluded.summary`,
		c.HNID, c.ArticleID, c.Author, c.RawText, c.Summary, c.Lang,
	)
	return err
}

// --- Signal ---

func (s *SqliteRepo) SaveSignal(userID string, sig *UserSignal) error {
	ctxJSON, _ := json.Marshal(sig.Context)
	_, err := s.db.Exec(
		`INSERT INTO user_signals (target_type, target_id, signal_type, strength, context)
		VALUES (?, ?, ?, ?, ?)`,
		sig.TargetType, sig.TargetID, sig.SignalType, sig.Strength, string(ctxJSON),
	)
	return err
}

func (s *SqliteRepo) GetSignalStats(userID string) (*SignalStats, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_signals`).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT signal_type, COUNT(*) FROM user_signals GROUP BY signal_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byType := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		byType[t] = c
	}

	return &SignalStats{
		TotalSignals: total,
		ByType:       byType,
	}, nil
}

func (s *SqliteRepo) GetRecentSignals(userID string, limit int) ([]UserSignal, error) {
	rows, err := s.db.Query(
		`SELECT id, target_type, target_id, signal_type, strength, context, created_at
		FROM user_signals ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []UserSignal
	for rows.Next() {
		var sig UserSignal
		var ctxJSON string
		if err := rows.Scan(&sig.ID, &sig.TargetType, &sig.TargetID, &sig.SignalType, &sig.Strength, &ctxJSON, &sig.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(ctxJSON), &sig.Context)
		signals = append(signals, sig)
	}
	return signals, rows.Err()
}

// GetSignalsByTarget returns signals for a specific target (backward-compat helper).
func (s *SqliteRepo) GetSignalsByTarget(targetType string, targetID int64) ([]UserSignal, error) {
	rows, err := s.db.Query(
		`SELECT id, target_type, target_id, signal_type, strength, context, created_at
		FROM user_signals WHERE target_type = ? AND target_id = ? ORDER BY created_at DESC`,
		targetType, targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []UserSignal
	for rows.Next() {
		var sig UserSignal
		var ctxJSON string
		if err := rows.Scan(&sig.ID, &sig.TargetType, &sig.TargetID, &sig.SignalType, &sig.Strength, &ctxJSON, &sig.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(ctxJSON), &sig.Context)
		signals = append(signals, sig)
	}
	return signals, rows.Err()
}

// GetAllArticles returns all articles (backward-compat helper).
func (s *SqliteRepo) GetAllArticles() ([]Article, error) {
	rows, err := s.db.Query(
		`SELECT id, hn_id, url, title, fetched_content, summary, key_points, lang, created_at, updated_at FROM articles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		var kpJSON string
		if err := rows.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &kpJSON, &a.Lang, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(kpJSON), &a.KeyPoints)
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// GetArticlesByIDs returns articles matching the given HN IDs.
func (s *SqliteRepo) GetArticlesByIDs(userID string, hnIDs []int64) ([]Article, error) {
	if len(hnIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, hn_id, url, title, fetched_content, summary, key_points, lang, created_at, updated_at FROM articles WHERE hn_id IN (`
	args := make([]interface{}, len(hnIDs))
	for i, id := range hnIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		var kpJSON string
		if err := rows.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &kpJSON, &a.Lang, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(kpJSON), &a.KeyPoints)
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// Close closes the database connection.
func (s *SqliteRepo) Close() error {
	return s.db.Close()
}

// --- Migration ---

func (s *SqliteRepo) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hn_id INTEGER NOT NULL,
    url TEXT,
    title TEXT,
    fetched_content TEXT,
    summary TEXT,
    key_points TEXT,
    lang TEXT DEFAULT 'zh',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hn_id, lang)
);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hn_id INTEGER NOT NULL,
    article_id INTEGER,
    author TEXT,
    raw_text TEXT,
    summary TEXT,
    lang TEXT DEFAULT 'zh',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hn_id, lang)
);

CREATE TABLE IF NOT EXISTS user_signals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    signal_type TEXT NOT NULL,
    strength INTEGER NOT NULL,
    context TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_signals_target ON user_signals(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_signals_type ON user_signals(signal_type);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	s.migrateArticlesLang()
	s.migrateCommentsLang()
	return nil
}

func (s *SqliteRepo) migrateArticlesLang() {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('articles') WHERE name = 'lang'`).Scan(&count)
	if err != nil || count > 0 {
		return
	}

	s.db.Exec(`
		CREATE TABLE articles_new (
		    id INTEGER PRIMARY KEY AUTOINCREMENT,
		    hn_id INTEGER NOT NULL,
		    url TEXT,
		    title TEXT,
		    fetched_content TEXT,
		    summary TEXT,
		    key_points TEXT,
		    lang TEXT DEFAULT 'zh',
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		    UNIQUE(hn_id, lang)
		)
	`)
	s.db.Exec(`INSERT INTO articles_new (id, hn_id, url, title, fetched_content, summary, key_points, lang, created_at, updated_at)
		SELECT id, hn_id, url, title, fetched_content, summary, key_points, 'zh', created_at, updated_at FROM articles`)
	s.db.Exec(`DROP TABLE articles`)
	s.db.Exec(`ALTER TABLE articles_new RENAME TO articles`)
}

func (s *SqliteRepo) migrateCommentsLang() {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('comments') WHERE name = 'lang'`).Scan(&count)
	if err != nil || count > 0 {
		return
	}

	s.db.Exec(`
		CREATE TABLE comments_new (
		    id INTEGER PRIMARY KEY AUTOINCREMENT,
		    hn_id INTEGER NOT NULL,
		    article_id INTEGER,
		    author TEXT,
		    raw_text TEXT,
		    summary TEXT,
		    lang TEXT DEFAULT 'zh',
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		    UNIQUE(hn_id, lang)
		)
	`)
	s.db.Exec(`INSERT INTO comments_new (id, hn_id, article_id, author, raw_text, summary, lang, created_at)
		SELECT id, hn_id, article_id, author, raw_text, summary, 'zh', created_at FROM comments`)
	s.db.Exec(`DROP TABLE comments`)
	s.db.Exec(`ALTER TABLE comments_new RENAME TO comments`)
}

// Compile-time interface check.
var _ Repository = (*SqliteRepo)(nil)
