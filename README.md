# Overview

A self-hosted Markdown knowledge base with **folder hierarchy**, full-text search,
wiki-links, multi-user authentication and WebDAV access. Content is stored as plain
Markdown files; the SQLite index is fully rebuildable.

## Features

- **Hierarchical folders** — physical directories map to the document tree
- **Markdown as source of truth** — Git-friendly, portable, no lock-in
- **Rich editor** — Vue 3 + Tiptap: images (paste/drop), tables, slash commands
- **Wiki-links & backlinks** — `[[Note]]` with a backlinks panel
- **Full-text search** — SQLite FTS5 with CJK unigram+bigram segmentation (substring search)
- **Multi-user auth** — bcrypt, sessions, admin/member roles, first-run setup
- **WebDAV** — mount the vault in Obsidian, mobile apps, etc. (`/dav`)
- **Portable attachments** — stored as `assets/...` relative paths
- **i18n** — Chinese and English UI
- **Table of contents** — outline panel with scroll-linked highlighting
- **Public sharing** — mark a note public and share an anonymous read-only link
- **MCP server** — AI agents can list/search/read/write notes via `POST /mcp`
- **AI assistant** — chat, organize and complete notes (any OpenAI-compatible API)
- **Version history & trash** — revision snapshots and soft delete with restore
- **Live sync** — a file watcher reindexes external edits; incremental indexing
- **Pluggable assets** — local filesystem or any S3-compatible object store
- **PWA & OpenAPI** — installable app shell; typed API docs at `/api/docs`
- **Single binary** — frontend embedded, one `docker run`

## Quick start (Docker)

```bash
docker compose up -d --build
# open http://localhost:5230 and complete first-run admin setup
```

## Local development

```bash
# backend (embedded frontend) on :5230
go run ./cmd/overview

# frontend with hot reload on :5173 (proxies /api to :5230)
cd web && npm install && npm run dev
```

Build the single binary:

```bash
cd web && npm run build && cd ..
go build -ldflags="-X main.version=0.5.0" -o bin/overview ./cmd/overview
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `OVERVIEW_ADDR` | `:5230` | Listen address |
| `OVERVIEW_DATA_DIR` | `./data` | Data root (`notes/`, `assets/`, DB) |
| `OVERVIEW_DB` | `<data>/overview.db` | SQLite index path |
| `OVERVIEW_MAX_UPLOAD_MB` | `32` | Upload limit |
| `OVERVIEW_LOG_LEVEL` | `info` | Log level |
| `OVERVIEW_AUTH` | `multi` | `multi` or `none` |
| `OVERVIEW_MCP_TOKEN` | — | Bearer token for MCP; falls back to session tokens |
| `OVERVIEW_AI_BASE_URL` | — | OpenAI-compatible base URL (enables AI when set) |
| `OVERVIEW_AI_API_KEY` | — | API key for the AI provider |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | AI model name |
| `OVERVIEW_S3_BUCKET` | — | Enables S3-compatible asset storage when set |
| `OVERVIEW_S3_ENDPOINT` | — | S3 endpoint (e.g. `s3.amazonaws.com`) |
| `OVERVIEW_S3_REGION` | `us-east-1` | S3 region |
| `OVERVIEW_S3_ACCESS_KEY` / `OVERVIEW_S3_SECRET_KEY` | — | S3 credentials |
| `OVERVIEW_S3_USE_SSL` | `true` | Use HTTPS for S3 |
| `OVERVIEW_S3_PUBLIC_URL` | — | Optional CDN/public URL prefix |

## API documentation

- Interactive list: `/api/docs`
- OpenAPI spec: `/api/v1/openapi.json`

## Data layout

```
data/
├── notes/        # Markdown files (the source of truth)
├── assets/       # uploaded attachments (or S3 when configured)
├── .history/     # note revision snapshots
├── .trash/       # soft-deleted notes
└── overview.db   # rebuildable SQLite index
```

## REST API (v1)

See [`docs/DESIGN.md`](docs/DESIGN.md) for the full API. Base path: `/api/v1`.

## WebDAV

```
http://<host>:5230/dav/
```

Use your Overview username and password (HTTP Basic). Any edits saved over WebDAV
are re-indexed automatically.

## MCP (Model Context Protocol)

Point an MCP client at `http://<host>:5230/mcp` and authenticate with
`Authorization: Bearer <OVERVIEW_MCP_TOKEN>`. Available tools:

`notes_list`, `notes_search`, `notes_read`, `notes_write`, `notes_delete`, `notes_links`.

## AI assistant

Set `OVERVIEW_AI_BASE_URL` (and `OVERVIEW_AI_API_KEY`) to any OpenAI-compatible
endpoint, e.g.:

```bash
OVERVIEW_AI_BASE_URL=https://api.openai.com/v1
OVERVIEW_AI_API_KEY=sk-...
OVERVIEW_AI_MODEL=gpt-4o-mini
```

The editor then shows an AI panel with **chat**, **organize** and **complete**.

## Public sharing

Toggle **Share** in the editor toolbar to publish a note. Anonymous readers get a
read-only view at `/public/<path>` and a listing at `/public`. Private notes are
never exposed.

## Development

```bash
make test        # go test ./...
make lint        # golangci-lint + vue-tsc
make fmt         # gofmt + prettier
make build       # frontend + single binary
```

## Architecture

Layered: `core` (domain + ports) → `service` (use cases) → `store`/`index`
(adapters) → `server` (HTTP). See [`docs/DESIGN.md`](docs/DESIGN.md) for details
and the architecture decision records.
