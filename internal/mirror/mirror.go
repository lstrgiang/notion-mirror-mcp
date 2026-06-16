package mirror

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PageMeta stores metadata about a locally synced Notion page.
type PageMeta struct {
	PageID         string    `json:"page_id"`
	Title          string    `json:"title"`
	LastEditedTime time.Time `json:"last_edited_time"`
	SyncedAt       time.Time `json:"synced_at"`
	URL            string    `json:"url"`
}

// Dir manages the local mirror directory structure.
type Dir struct {
	Root string
}

// New creates and initializes the mirror directory.
func New(root string) (*Dir, error) {
	d := &Dir{Root: root}
	return d, d.init()
}

func (d *Dir) init() error {
	for _, sub := range []string{"", "pages", "databases"} {
		if err := os.MkdirAll(filepath.Join(d.Root, sub), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	return nil
}

// DBPath returns the SQLite database path.
func (d *Dir) DBPath() string {
	return filepath.Join(d.Root, "mirror.db")
}

// PagePath returns the markdown file path for a page.
func (d *Dir) PagePath(pageID string) string {
	return filepath.Join(d.Root, "pages", pageID+".md")
}

// MetaPath returns the metadata file path for a page.
func (d *Dir) MetaPath(pageID string) string {
	return filepath.Join(d.Root, "pages", pageID+".meta.json")
}

// WritePage writes markdown content for a page.
func (d *Dir) WritePage(pageID, content string) error {
	return os.WriteFile(d.PagePath(pageID), []byte(content), 0o644)
}

// ReadPage reads the markdown content of a locally mirrored page.
func (d *Dir) ReadPage(pageID string) (string, error) {
	data, err := os.ReadFile(d.PagePath(pageID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteMeta persists page metadata as JSON.
func (d *Dir) WriteMeta(meta *PageMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.MetaPath(meta.PageID), data, 0o644)
}

// ReadMeta reads metadata for a page.
func (d *Dir) ReadMeta(pageID string) (*PageMeta, error) {
	data, err := os.ReadFile(d.MetaPath(pageID))
	if err != nil {
		return nil, err
	}
	var meta PageMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ListPageIDs returns all page IDs stored in the local mirror.
func (d *Dir) ListPageIDs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(d.Root, "pages"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".md" {
			ids = append(ids, name[:len(name)-3])
		}
	}
	return ids, nil
}

// DatabaseDir returns the directory for a database.
func (d *Dir) DatabaseDir(dbID string) string {
	return filepath.Join(d.Root, "databases", dbID)
}

// WriteDBSchema writes a database's property schema.
func (d *Dir) WriteDBSchema(dbID string, schema any) error {
	if err := os.MkdirAll(d.DatabaseDir(dbID), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.DatabaseDir(dbID), "schema.json"), data, 0o644)
}

// WriteDBRows writes the rows of a database.
func (d *Dir) WriteDBRows(dbID string, rows any) error {
	if err := os.MkdirAll(d.DatabaseDir(dbID), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.DatabaseDir(dbID), "rows.json"), data, 0o644)
}
