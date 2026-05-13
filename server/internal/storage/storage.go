package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

type Article struct {
	ID             int64     `json:"id"`
	HNID           int64     `json:"hn_id"`
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	FetchedContent string    `json:"fetched_content"`
	Summary        string    `json:"summary"`
	KeyPoints      string    `json:"key_points"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	HNID      int64     `json:"hn_id"`
	ArticleID int64     `json:"article_id"`
	Author    string    `json:"author"`
	RawText   string    `json:"raw_text"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSignal struct {
	ID         int64     `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   int64     `json:"target_id"`
	SignalType string    `json:"signal_type"`
	Strength   int       `json:"strength"`
	Context    string    `json:"context"`
	CreatedAt  time.Time `json:"created_at"`
}

func New(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hn_id INTEGER UNIQUE NOT NULL,
    url TEXT,
    title TEXT,
    fetched_content TEXT,
    summary TEXT,
    key_points TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hn_id INTEGER NOT NULL,
    article_id INTEGER,
    author TEXT,
    raw_text TEXT,
    summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	return err
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) GetArticleByHNID(hnID int64) (*Article, error) {
	row := s.db.QueryRow(`SELECT id, hn_id, url, title, fetched_content, summary, key_points, created_at, updated_at FROM articles WHERE hn_id = ?`, hnID)
	var a Article
	err := row.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &a.KeyPoints, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Storage) SaveArticle(a *Article) error {
	_, err := s.db.Exec(
		`INSERT INTO articles (hn_id, url, title, fetched_content, summary, key_points, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hn_id) DO UPDATE SET
			url=excluded.url, title=excluded.title, fetched_content=excluded.fetched_content,
			summary=excluded.summary, key_points=excluded.key_points, updated_at=excluded.updated_at`,
		a.HNID, a.URL, a.Title, a.FetchedContent, a.Summary, a.KeyPoints, time.Now(),
	)
	return err
}

func (s *Storage) SaveSignal(signal *UserSignal) error {
	ctxJSON, _ := json.Marshal(signal.Context)
	_, err := s.db.Exec(
		`INSERT INTO user_signals (target_type, target_id, signal_type, strength, context)
		VALUES (?, ?, ?, ?, ?)`,
		signal.TargetType, signal.TargetID, signal.SignalType, signal.Strength, string(ctxJSON),
	)
	return err
}

func (s *Storage) GetSignalsByTarget(targetType string, targetID int64) ([]UserSignal, error) {
	rows, err := s.db.Query(
		`SELECT id, target_type, target_id, signal_type, strength, context, created_at FROM user_signals
		WHERE target_type = ? AND target_id = ? ORDER BY created_at DESC`,
		targetType, targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []UserSignal
	for rows.Next() {
		var s UserSignal
		if err := rows.Scan(&s.ID, &s.TargetType, &s.TargetID, &s.SignalType, &s.Strength, &s.Context, &s.CreatedAt); err != nil {
			return nil, err
		}
		signals = append(signals, s)
	}
	return signals, rows.Err()
}

func (s *Storage) GetAllArticles() ([]Article, error) {
	rows, err := s.db.Query(`SELECT id, hn_id, url, title, fetched_content, summary, key_points, created_at, updated_at FROM articles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &a.KeyPoints, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

func (s *Storage) GetSignalStats() (map[string]interface{}, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM user_signals`).Scan(&total)
	if err != nil {
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

	return map[string]interface{}{
		"total_signals": total,
		"by_type":       byType,
	}, nil
}

func (s *Storage) GetCommentByHNID(hnID int64) (*Comment, error) {
	row := s.db.QueryRow(`SELECT id, hn_id, article_id, author, raw_text, summary, created_at FROM comments WHERE hn_id = ?`, hnID)
	var c Comment
	err := row.Scan(&c.ID, &c.HNID, &c.ArticleID, &c.Author, &c.RawText, &c.Summary, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Storage) SaveComment(c *Comment) error {
	_, err := s.db.Exec(
		`INSERT INTO comments (hn_id, article_id, author, raw_text, summary)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(hn_id) DO UPDATE SET
			article_id=excluded.article_id, author=excluded.author,
			raw_text=excluded.raw_text, summary=excluded.summary`,
		c.HNID, c.ArticleID, c.Author, c.RawText, c.Summary,
	)
	return err
}

func (s *Storage) GetArticlesByIDs(hnIDs []int64) ([]Article, error) {
	if len(hnIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, hn_id, url, title, fetched_content, summary, key_points, created_at, updated_at FROM articles WHERE hn_id IN (`
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
		if err := rows.Scan(&a.ID, &a.HNID, &a.URL, &a.Title, &a.FetchedContent, &a.Summary, &a.KeyPoints, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

func (s *Storage) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

func (s *Storage) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

func (s *Storage) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(query, args...)
}

func (s *Storage) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}

func (s *Storage) DB() *sql.DB {
	return s.db
}
