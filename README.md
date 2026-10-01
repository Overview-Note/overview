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

## REST API (v1)

See [`docs/DESIGN.md`](docs/DESIGN.md) for the full API. Base path: `/api/v1`.

## WebDAV

```
http://<host>:5230/dav/
```

Use your Overview username and password (HTTP Basic). Any edits saved over WebDAV
are re-indexed automatically.

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
