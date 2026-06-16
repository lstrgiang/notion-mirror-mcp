package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jomei/notionapi"

	"github.com/lstrgiang/notion-mirror-mcp/internal/convert"
	"github.com/lstrgiang/notion-mirror-mcp/internal/mirror"
	"github.com/lstrgiang/notion-mirror-mcp/internal/search"
)

// Syncer pulls Notion content and writes it to the local mirror.
type Syncer struct {
	client *notionapi.Client
	dir    *mirror.Dir
	idx    *search.Index
}

// New creates a Syncer with the given Notion API token.
func New(token string, dir *mirror.Dir, idx *search.Index) *Syncer {
	return &Syncer{
		client: notionapi.NewClient(notionapi.Token(token)),
		dir:    dir,
		idx:    idx,
	}
}

// SyncAll fetches every page and database accessible via the integration.
func (s *Syncer) SyncAll(ctx context.Context) error {
	log.Println("starting full sync")
	var cursor notionapi.Cursor
	for {
		resp, err := s.client.Search.Do(ctx, &notionapi.SearchRequest{
			StartCursor: cursor,
			PageSize:    100,
		})
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		for _, result := range resp.Results {
			switch obj := result.(type) {
			case *notionapi.Page:
				if err := s.syncPage(ctx, obj); err != nil {
					log.Printf("warn: sync page %s: %v", obj.ID, err)
				}
			case *notionapi.Database:
				if err := s.syncDatabase(ctx, obj); err != nil {
					log.Printf("warn: sync database %s: %v", obj.ID, err)
				}
			}
		}
		if !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
	log.Println("full sync complete")
	return nil
}

// SyncPage fetches and mirrors a single Notion page by ID.
func (s *Syncer) SyncPage(ctx context.Context, pageID string) error {
	page, err := s.client.Page.Get(ctx, notionapi.PageID(pageID))
	if err != nil {
		return fmt.Errorf("get page: %w", err)
	}
	return s.syncPage(ctx, page)
}

// SyncIncremental mirrors only pages edited after the given timestamp.
// Pages are returned newest-first; we stop when we pass the cutoff.
func (s *Syncer) SyncIncremental(ctx context.Context, since time.Time) error {
	log.Printf("incremental sync since %s", since.Format(time.RFC3339))
	var cursor notionapi.Cursor
	for {
		resp, err := s.client.Search.Do(ctx, &notionapi.SearchRequest{
			StartCursor: cursor,
			PageSize:    100,
			Filter: notionapi.SearchFilter{
				Value:    "page",
				Property: "object",
			},
			Sort: &notionapi.SortObject{
				Direction: notionapi.SortOrderDESC,
				Timestamp: notionapi.TimestampLastEdited,
			},
		})
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		done := false
		for _, result := range resp.Results {
			page, ok := result.(*notionapi.Page)
			if !ok {
				continue
			}
			if page.LastEditedTime.Before(since) {
				done = true
				break
			}
			if err := s.syncPage(ctx, page); err != nil {
				log.Printf("warn: sync page %s: %v", page.ID, err)
			}
		}
		if done || !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
	return nil
}

func (s *Syncer) syncPage(ctx context.Context, page *notionapi.Page) error {
	title := convert.Title(page.Properties)

	blocks, err := s.fetchAllBlocks(ctx, notionapi.BlockID(page.ID))
	if err != nil {
		return fmt.Errorf("fetch blocks: %w", err)
	}

	body := convert.Blocks(blocks)
	md := fmt.Sprintf("# %s\n\n%s", title, body)

	if err := s.dir.WritePage(string(page.ID), md); err != nil {
		return err
	}
	if err := s.dir.WriteMeta(&mirror.PageMeta{
		PageID:         string(page.ID),
		Title:          title,
		LastEditedTime: page.LastEditedTime,
		SyncedAt:       time.Now(),
		URL:            page.URL,
	}); err != nil {
		return err
	}
	if err := s.idx.Upsert(string(page.ID), title, md); err != nil {
		return err
	}

	log.Printf("synced page: %q (%s)", title, page.ID)
	return nil
}

func (s *Syncer) syncDatabase(ctx context.Context, db *notionapi.Database) error {
	dbID := string(db.ID)

	if err := s.dir.WriteDBSchema(dbID, db.Properties); err != nil {
		return err
	}

	var rows []notionapi.Page
	var cursor notionapi.Cursor
	for {
		resp, err := s.client.Database.Query(ctx, notionapi.DatabaseID(dbID), &notionapi.DatabaseQueryRequest{
			StartCursor: cursor,
			PageSize:    100,
		})
		if err != nil {
			return fmt.Errorf("query db: %w", err)
		}
		rows = append(rows, resp.Results...)
		if !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}

	if err := s.dir.WriteDBRows(dbID, rows); err != nil {
		return err
	}

	log.Printf("synced database %s (%d rows)", dbID, len(rows))
	return nil
}

func (s *Syncer) fetchAllBlocks(ctx context.Context, blockID notionapi.BlockID) ([]notionapi.Block, error) {
	var all []notionapi.Block
	var cursor notionapi.Cursor
	for {
		resp, err := s.client.Block.GetChildren(ctx, blockID, &notionapi.Pagination{
			StartCursor: cursor,
			PageSize:    100,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Results...)
		if !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
	return all, nil
}
