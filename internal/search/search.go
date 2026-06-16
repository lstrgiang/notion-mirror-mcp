package search

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Result is a single FTS5 search hit.
type Result struct {
	PageID  string
	Title   string
	Snippet string
}

// IndexEntry is input for bulk index rebuilds.
type IndexEntry struct {
	PageID  string
	Title   string
	Content string
}

// Index wraps a SQLite FTS5 virtual table for full-text search.
type Index struct {
	db *sql.DB
}

// Open opens (or creates) the search index at dbPath.
func Open(dbPath string) (*Index, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	idx := &Index{db: db}
	return idx, idx.migrate()
}

func (idx *Index) migrate() error {
	_, err := idx.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
			page_id UNINDEXED,
			title,
			content,
			tokenize='porter ascii'
		)
	`)
	return err
}

// Close releases the database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// Upsert inserts or replaces a page in the index.
func (idx *Index) Upsert(pageID, title, content string) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM pages_fts WHERE page_id = ?`, pageID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO pages_fts(page_id, title, content) VALUES (?, ?, ?)`,
		pageID, title, content,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// Delete removes a page from the index.
func (idx *Index) Delete(pageID string) error {
	_, err := idx.db.Exec(`DELETE FROM pages_fts WHERE page_id = ?`, pageID)
	return err
}

// Search performs a full-text search and returns up to limit results.
func (idx *Index) Search(query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := idx.db.Query(`
		SELECT page_id, title,
			snippet(pages_fts, 2, '[', ']', '...', 30)
		FROM pages_fts
		WHERE pages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.PageID, &r.Title, &r.Snippet); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Rebuild replaces the entire index with the provided entries (atomic swap).
func (idx *Index) Rebuild(entries []IndexEntry) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM pages_fts`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO pages_fts(page_id, title, content) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.PageID, e.Title, e.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}
