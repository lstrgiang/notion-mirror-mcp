# notion-mirror-mcp

A local [MCP](https://modelcontextprotocol.io) server that mirrors your Notion workspace to disk and serves it to Cursor (or any MCP client) with **zero Notion API calls on reads**. Full-text search runs against a local SQLite FTS5 index; page content is served straight from `.md` files.

**Why?** Notion API calls are slow, rate-limited, and burn tokens on every round-trip. This server syncs once, then answers every read from disk — search, retrieval, and browsing are instant.

---

## How it works

```
Notion API ──(sync)──► ~/.notion-mirror/
                            ├── mirror.db          ← SQLite FTS5 index
                            ├── pages/
                            │   ├── {page-id}.md
                            │   └── {page-id}.meta.json
                            └── databases/
                                └── {db-id}/
                                    ├── schema.json
                                    └── rows.json

Cursor / Claude ──(MCP)──► notion-mirror-mcp ──► local files only
```

API calls only happen during `sync`. Everything else is pure local I/O.

---

## MCP tools

| Tool | Arguments | Description |
|------|-----------|-------------|
| `search` | `query` (required), `limit` (optional, default 10) | SQLite FTS5 full-text search across all mirrored pages, returns ranked snippets |
| `get_page` | `page_id` (required) | Returns the full markdown content of a page — reads local `.md` file, no API call |
| `sync` | `page_id` (optional) | Fetches from Notion API and updates local mirror; omit `page_id` for full workspace sync |

---

## Prerequisites

- **Go 1.21+** — [install](https://go.dev/dl/)
- **A C compiler** — required by `mattn/go-sqlite3` (comes with Xcode CLT on macOS: `xcode-select --install`)
- **A Notion integration token** — see step 1 below

---

## Setup

### 1. Create a Notion integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations) and click **New integration**
2. Give it a name (e.g. `notion-mirror`), select your workspace, click **Submit**
3. Copy the **Internal Integration Secret** (`secret_xxx...`)
4. For each Notion page or database you want to mirror: open it in Notion → **⋯ menu → Connections → add your integration**

> Only pages explicitly shared with the integration (or their children) are accessible.

### 2. Clone and configure

```bash
git clone https://github.com/lstrgiang/notion-mirror-mcp.git
cd notion-mirror-mcp

cp .env.example .env
```

Edit `.env`:

```env
NOTION_TOKEN=secret_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
MIRROR_DIR=~/.notion-mirror   # optional, this is the default
```

### 3. Build

```bash
make build
# produces ./notion-mirror-mcp
```

Or install directly to `$GOPATH/bin`:

```bash
make install
```

### 4. Run an initial full sync

The mirror starts empty. Run the binary and trigger a sync before using it in Cursor:

```bash
# start the server (it speaks MCP over stdio, so pipe via a client or use mcp-cli)
./notion-mirror-mcp

# or trigger sync directly from your shell using npx @modelcontextprotocol/cli:
npx @modelcontextprotocol/cli call ./notion-mirror-mcp sync '{}'
```

Full sync can take a few minutes for large workspaces. Progress is logged to stderr.

### 5. Add to Cursor

Open Cursor settings → **MCP** tab, or edit `.cursor/mcp.json` in your project root (project-scoped) or `~/.cursor/mcp.json` (global):

```json
{
  "mcpServers": {
    "notion-mirror": {
      "command": "/absolute/path/to/notion-mirror-mcp",
      "env": {
        "NOTION_TOKEN": "secret_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
        "MIRROR_DIR": "/Users/you/.notion-mirror"
      }
    }
  }
}
```

> Use the absolute path to the binary. If you ran `make install`, it is `$GOPATH/bin/notion-mirror-mcp` (usually `~/go/bin/notion-mirror-mcp`).

Restart Cursor. The three tools (`search`, `get_page`, `sync`) will appear in the MCP tool list.

---

## Usage in Cursor

Once connected, you can ask Cursor things like:

- *"Search my Notion for notes on the auth redesign"* → uses `search`
- *"Get the full content of this Notion page: `abc-123-...`"* → uses `get_page`
- *"Sync my Notion mirror"* → uses `sync`

### Keeping the mirror fresh

The mirror doesn't auto-update. You control when to sync:

| Scenario | What to do |
|----------|-----------|
| First time setup | Call `sync` with no arguments (full sync) |
| Daily refresh | Call `sync` with no arguments again — incremental, only fetches pages edited since last sync |
| You just edited a specific page | Call `sync` with that `page_id` |

Incremental sync is fast: it fetches pages sorted by `last_edited_time` descending and stops as soon as it reaches timestamps older than the previous sync.

---

## Local file structure

```
~/.notion-mirror/
├── mirror.db                      # SQLite FTS5 index (rebuilt on each sync)
├── pages/
│   ├── abc123de-....md            # Page content as clean markdown
│   └── abc123de-....meta.json     # Title, URL, last_edited_time, synced_at
└── databases/
    └── def456ab-.../
        ├── schema.json            # Database property definitions
        └── rows.json              # All rows as JSON
```

Pages are the source of truth — the FTS5 index is derived from them and can always be rebuilt.

---

## Tech stack

| Library | Purpose |
|---------|---------|
| [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) | MCP server, stdio transport |
| [jomei/notionapi](https://github.com/jomei/notionapi) | Notion API client |
| [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | SQLite FTS5 (built with `-tags sqlite_fts5`) |
| [joho/godotenv](https://github.com/joho/godotenv) | `.env` file loading |

Single compiled Go binary — no runtime dependencies, fast startup, low memory footprint.

---

## Development

```bash
go mod tidy        # sync dependencies
make build         # build with FTS5 support → ./notion-mirror-mcp
make install       # install to $GOPATH/bin
```

Project layout:

```
cmd/server/main.go          # MCP server entry point, tool wiring
internal/mirror/mirror.go   # filesystem read/write helpers
internal/convert/convert.go # Notion blocks → clean markdown
internal/sync/sync.go       # full + incremental Notion sync
internal/search/search.go   # SQLite FTS5 index management
```
