package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	path string
	db   *sql.DB
}

// NewSQLiteStore creates a store at the given database path.
func NewSQLiteStore(path string) *SQLiteStore {
	return &SQLiteStore{path: path}
}

func (s *SQLiteStore) Open() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open sqlite %s: %w", s.path, err)
	}
	s.db = db
	return s.migrate()
}

func (s *SQLiteStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dir TEXT NOT NULL,
			provider TEXT NOT NULL,
			platform TEXT NOT NULL,
			backend TEXT NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			status TEXT NOT NULL,
			metadata_json TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("migrate sessions: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Create(rec Record) (int64, error) {
	meta, err := json.Marshal(rec.Metadata)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}
	res, err := s.db.Exec(
		`INSERT INTO sessions (dir, provider, platform, backend, started_at, ended_at, status, metadata_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Dir,
		string(rec.Provider),
		rec.Platform,
		rec.Backend,
		rec.StartedAt.UTC().Format(time.RFC3339),
		formatTime(rec.EndedAt),
		string(rec.Status),
		string(meta),
	)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

func (s *SQLiteStore) Update(rec Record) error {
	meta, err := json.Marshal(rec.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.Exec(
		`UPDATE sessions SET ended_at = ?, status = ?, metadata_json = ? WHERE id = ?`,
		formatTime(rec.EndedAt),
		string(rec.Status),
		string(meta),
		rec.ID,
	)
	if err != nil {
		return fmt.Errorf("update session %d: %w", rec.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Get(id int64) (Record, error) {
	row := s.db.QueryRow(
		`SELECT id, dir, provider, platform, backend, started_at, ended_at, status, metadata_json FROM sessions WHERE id = ?`,
		id,
	)
	return scanRecord(row)
}

func (s *SQLiteStore) List(limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, dir, provider, platform, backend, started_at, ended_at, status, metadata_json
		 FROM sessions ORDER BY started_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete session rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session %d not found", id)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var rec Record
	var provider, status string
	var started, ended sql.NullString
	var metaJSON string
	if err := row.Scan(
		&rec.ID, &rec.Dir, &provider, &rec.Platform, &rec.Backend,
		&started, &ended, &status, &metaJSON,
	); err != nil {
		return rec, fmt.Errorf("scan session: %w", err)
	}
	rec.Provider = Provider(provider)
	rec.Status = Status(status)
	if started.Valid {
		rec.StartedAt, _ = time.Parse(time.RFC3339, started.String)
	}
	if ended.Valid && ended.String != "" {
		rec.EndedAt, _ = time.Parse(time.RFC3339, ended.String)
	}
	if err := json.Unmarshal([]byte(metaJSON), &rec.Metadata); err != nil {
		return rec, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return rec, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func writeFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
