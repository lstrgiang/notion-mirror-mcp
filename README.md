# notion-mirror-mcp

A local MCP server that mirrors Notion pages to disk and serves them to Cursor (or any MCP client) with **zero Notion API calls on reads**. Full-text search is powered by a SQLite FTS5 index built from local `.md` files.

## How it works

```
Notion API ──(sync)──► ~/.notion-mirror/
                            ├── mirror.db        ← SQLite FTS5 index
                            ├── pages/
                            │   ├── {page-id}.md
                            │   └── {page-id}.meta.json
                            └── databases/
                                └── {db-id}/
                                    ├── schema.json
                                    └── rows.json

Cursor ──(MCP tools)──► notion-mirror-mcp ──► local files only
```

On reads, the server never touches the Notion API — it reads from disk. API calls only happen during `sync`.

## MCP tools

| Tool | Description |
|------|-------------|
| `search(query, limit?)` | SQLite FTS5 full-text search, returns top-K chunks with snippets |
| `get_page(page_id)` | Reads local `.md` file directly — no API call |
| `sync(page_id?)` | Triggers Notion API fetch; omit `page_id` for full sync |

## Setup

### 1. Create a Notion integration

1. Go to <https://www.notion.so/my-integrations>
2. Create a new integration, copy the **Internal Integration Token**
3. Share the pages/databases you want to mirror with the integration

### 2. Configure environment

```bash
cp .env.example .env
# Edit .env and set NOTION_TOKEN=secret_xxx...
```

### 3. Build

Requires Go 1.21+ and a C compiler (for cgo/sqlite3).

```bash
make build
# produces ./notion-mirror-mcp
```

### 4. Initial sync

Run the binary once manually to do a full sync before using it in Cursor:

```bash
NOTION_TOKEN=secret_xxx ./notion-mirror-mcp
# In another terminal or via the MCP client, call sync()
```

### 5. Configure Cursor

Add to your Cursor MCP config (`.cursor/mcp.json` or global MCP settings):

```json
{
  "mcpServers": {
    "notion-mirror": {
      "command": "/path/to/notion-mirror-mcp",
      "env": {
        "NOTION_TOKEN": "secret_xxx",
        "MIRROR_DIR": "/Users/you/.notion-mirror"
      }
    }
  }
}
```

## Sync strategy

- **First run**: call `sync()` with no arguments — crawls all pages and databases accessible to the integration.
- **Incremental**: call `sync()` again at any time — internally fetches pages sorted by `last_edited_time` descending and stops when it reaches already-seen timestamps.
- **Single page**: call `sync(page_id)` to refresh one specific page immediately.

The FTS5 index is updated atomically on every sync (upsert per page, full rebuild on demand).

## Tech stack

- **Go** — single compiled binary, fast startup, low memory
- [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) — MCP server (stdio transport)
- [`jomei/notionapi`](https://github.com/jomei/notionapi) — Notion API client
- [`mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) — SQLite FTS5 (built with `-tags sqlite_fts5`)
- [`joho/godotenv`](https://github.com/joho/godotenv) — `.env` file loading

## Development

```bash
go mod tidy          # sync dependencies
make build           # build binary with FTS5 support
make install         # install to $GOPATH/bin
```
