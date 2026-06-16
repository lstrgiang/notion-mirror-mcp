package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/lstrgiang/notion-mirror-mcp/internal/mirror"
	"github.com/lstrgiang/notion-mirror-mcp/internal/search"
	internalsync "github.com/lstrgiang/notion-mirror-mcp/internal/sync"
)

func main() {
	_ = godotenv.Load()

	token := os.Getenv("NOTION_TOKEN")
	if token == "" {
		log.Fatal("NOTION_TOKEN environment variable is required")
	}

	mirrorPath := os.Getenv("MIRROR_DIR")
	if mirrorPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("get home dir: %v", err)
		}
		mirrorPath = filepath.Join(home, ".notion-mirror")
	}

	dir, err := mirror.New(mirrorPath)
	if err != nil {
		log.Fatalf("init mirror dir: %v", err)
	}

	idx, err := search.Open(dir.DBPath())
	if err != nil {
		log.Fatalf("open search index: %v", err)
	}
	defer idx.Close()

	syncer := internalsync.New(token, dir, idx)

	s := server.NewMCPServer("notion-mirror-mcp", "1.0.0",
		server.WithToolCapabilities(true),
	)

	// search — full-text search over local FTS5 index, zero Notion API calls
	s.AddTool(
		mcp.NewTool("search",
			mcp.WithDescription("Full-text search across locally mirrored Notion pages (SQLite FTS5). Zero API calls."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query string"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max results to return (default 10)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			query, _ := args["query"].(string)
			if query == "" {
				return mcp.NewToolResultError("query is required"), nil
			}
			limitF, _ := args["limit"].(float64)
			limit := int(limitF)
			if limit <= 0 {
				limit = 10
			}

			results, err := idx.Search(query, limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("search error: %v", err)), nil
			}
			if len(results) == 0 {
				return mcp.NewToolResultText("No results found. Run sync first if the mirror is empty."), nil
			}

			var sb strings.Builder
			for i, r := range results {
				fmt.Fprintf(&sb, "%d. **%s** (`%s`)\n   %s\n\n", i+1, r.Title, r.PageID, r.Snippet)
			}
			return mcp.NewToolResultText(sb.String()), nil
		},
	)

	// get_page — reads a local .md file directly, no API call
	s.AddTool(
		mcp.NewTool("get_page",
			mcp.WithDescription("Read a locally mirrored Notion page as clean markdown. No API call."),
			mcp.WithString("page_id",
				mcp.Required(),
				mcp.Description("Notion page ID (UUID)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageID, _ := req.GetArguments()["page_id"].(string)
			if pageID == "" {
				return mcp.NewToolResultError("page_id is required"), nil
			}

			content, err := dir.ReadPage(pageID)
			if err != nil {
				if os.IsNotExist(err) {
					return mcp.NewToolResultError(
						fmt.Sprintf("page %q not found locally — run sync first", pageID),
					), nil
				}
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(content), nil
		},
	)

	// sync — triggers Notion API fetch to refresh the local cache
	s.AddTool(
		mcp.NewTool("sync",
			mcp.WithDescription("Sync Notion content to local mirror. Omit page_id for a full sync of all accessible pages."),
			mcp.WithString("page_id",
				mcp.Description("Specific page ID to sync. Omit for full sync."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageID, _ := req.GetArguments()["page_id"].(string)

			if pageID != "" {
				if err := syncer.SyncPage(ctx, pageID); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("sync failed: %v", err)), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf("synced page %s", pageID)), nil
			}

			if err := syncer.SyncAll(ctx); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("full sync failed: %v", err)), nil
			}
			return mcp.NewToolResultText("full sync complete"), nil
		},
	)

	log.Printf("notion-mirror-mcp started (mirror: %s)", mirrorPath)
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
