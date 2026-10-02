# Overview

> A self-hosted, folder-based Markdown knowledge base — **files as the source of truth**, AI-native, and deployable as a single binary.

[![CI](https://github.com/Overview-Note/overview/actions/workflows/ci.yml/badge.svg)](https://github.com/Overview-Note/overview/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)](https://vuejs.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white)](Dockerfile)

Overview is what you get when you take the simplicity of [memos](https://github.com/usememos/memos),
add the **hierarchy memos lacks**, and keep your content as **plain Markdown files** you
fully own. It ships as one Go binary with the frontend embedded, indexes content with
SQLite FTS5, and exposes the vault over REST, **WebDAV**, and the **Model Context Protocol (MCP)**.

---

## Why Overview?

| | memos | Obsidian | Overview |
|---|:---:|:---:|:---:|
| Self-hosted web app | ✅ | ❌ | ✅ |
| Folder hierarchy / nested tree | ❌ | ✅ | ✅ |
| Plain-Markdown files on disk | ❌ (DB) | ✅ | ✅ |
| Built-in full-text search (CJK-aware) | ⚠️ | ✅ | ✅ |
| REST API + WebDAV | partial | ❌ | ✅ |
| MCP server for AI agents | ❌ | ❌ | ✅ |
| Built-in AI assistant | ❌ | plugin | ✅ |
| Single binary / one `docker run` | ✅ | ❌ | ✅ |

**Design goals**

1. **Files are the truth.** Every note is a Markdown file with YAML frontmatter; the
   SQLite index is disposable and fully rebuildable. Point any editor, Git, or another
   tool at the same `data/` directory.
2. **Hierarchy without lock-in.** Folders on disk are the document tree.
3. **Boring to operate.** One process, no external services required (SQLite + files).
4. **Open by design.** Everything is reachable through well-defined ports: REST, WebDAV, MCP.

---

## Feature Highlights

### 📚 Content & organization
- **Hierarchical folders** — physical directories map directly to the navigation tree
- **Markdown as the source of truth** — Git-friendly, portable, no vendor lock-in
- **Wiki-links & backlinks** — `[[Note]]` / `[[Note|alias]]`, with a backlinks panel;
  targets resolve by full path, title, or file name
- **Per-note visibility** — Private or Public (share a read-only **globe page** at `/public/<path>`)

### ✍️ Editing
- **Rich editor** — Vue 3 + Tiptap: headings, lists, quotes, code, images, tables
- **Images** — paste / drag-and-drop, with optional **client-side compression** (WebP/JPEG)
- **Slash commands** — type `/` for headings, lists, tables, dividers, code blocks
- **Outline (TOC)** — scroll-linked table of contents
- **Table bubble menu** — row/column controls appear right above the active table

### 🔎 Search & navigation
- **Full-text search** with SQLite FTS5 and a custom **CJK unigram + bigram tokenizer**
  for real substring matching (e.g. `发模` matches `并发模型`)
- **Global search** with `Ctrl`/`Cmd` + `K`

### 🤖 AI-native
- **AI assistant** — chat, *organize* and *complete* notes, using any OpenAI-compatible
  endpoint (OpenAI, DeepSeek, Ollama, vLLM, …)
- **MCP server** — expose notes as tools so AI agents can list/search/read/write
  (`notes_list`, `notes_search`, `notes_read`, `notes_write`, `notes_delete`, `notes_links`)

### 🔐 Access & integration
- **Multi-user auth** — bcrypt, sessions, admin/member roles, first-run setup wizard
- **WebDAV** — mount the vault in Obsidian, Finder, or mobile apps
- **REST API** — versioned under `/api/v1`, documented via OpenAPI at `/api/docs`
- **PWA** — installable app shell

### 🛠 Operations
- **Single binary** with the frontend embedded (`docker run` or `go run`)
- **SQLite migrations**, atomic writes, optimistic concurrency (ETag / 409)
- **Version history** (revision snapshots) and **trash** (soft delete + restore)
- **Live sync** — a file watcher reindexes external edits
- **Pluggable storage** — local filesystem or any S3-compatible object store
- **Settings center** — theme (system/light/dark), font size, language, compression, AI config

---

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
docker compose up -d --build
# open http://localhost:5230 and complete the first-run admin setup
```

### Prebuilt binary

```bash
go install github.com/Overview-Note/overview/cmd/overview@latest
OVERVIEW_DATA_DIR=./data overview
```

### Local development

```bash
# backend (serves the embedded frontend) on :5230
go run ./cmd/overview

# frontend with hot reload on :5173 (proxies /api to :5230)
cd web && npm install && npm run dev
```

Build a single binary with the frontend embedded:

```bash
make build        # == cd web && npm run build && go build -o bin/overview ./cmd/overview
```

---

## Configuration

Overview is configured entirely through environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `OVERVIEW_ADDR` | `:5230` | Listen address |
| `OVERVIEW_DATA_DIR` | `./data` | Data root (`notes/`, `assets/`, DB, …) |
| `OVERVIEW_DB` | `<data>/overview.db` | SQLite index path |
| `OVERVIEW_MAX_UPLOAD_MB` | `32` | Upload size limit |
| `OVERVIEW_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `OVERVIEW_AUTH` | `multi` | `multi` (users + login) or `none` |
| `OVERVIEW_MCP_TOKEN` | — | Bearer token for MCP; falls back to session tokens |
| `OVERVIEW_AI_BASE_URL` | — | OpenAI-compatible base URL (enables AI when set) |
| `OVERVIEW_AI_API_KEY` | — | AI provider API key |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | AI model name |
| `OVERVIEW_S3_BUCKET` | — | Enables S3-compatible asset storage when set |
| `OVERVIEW_S3_ENDPOINT` | — | S3 endpoint (e.g. `s3.amazonaws.com`) |
| `OVERVIEW_S3_REGION` | `us-east-1` | S3 region |
| `OVERVIEW_S3_ACCESS_KEY` / `OVERVIEW_S3_SECRET_KEY` | — | S3 credentials |
| `OVERVIEW_S3_USE_SSL` | `true` | Use HTTPS for S3 |
| `OVERVIEW_S3_PUBLIC_URL` | — | Optional CDN / public URL prefix |

> AI can also be configured at runtime in **Settings → AI** (admin only) — no restart needed.

---

## Integrations

**WebDAV**

```
http://<host>:5230/dav/
```
Sign in with your Overview username and password (HTTP Basic). Edits saved over WebDAV
are re-indexed automatically.

**MCP (Model Context Protocol)**

Point an MCP client at `http://<host>:5230/mcp` with `Authorization: Bearer <OVERVIEW_MCP_TOKEN>`.
Tools: `notes_list`, `notes_search`, `notes_read`, `notes_write`, `notes_delete`, `notes_links`.

**OpenAPI**

- Interactive docs: `/api/docs`
- Machine-readable spec: `/api/v1/openapi.json`

---

## Data Layout

```
data/
├── notes/        # Markdown files — the source of truth (folders = hierarchy)
├── assets/       # uploaded attachments (or S3 when configured)
├── .history/     # note revision snapshots
├── .trash/       # soft-deleted notes
└── overview.db   # SQLite index (safe to delete; rebuilt on startup)
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  Browser (Vue 3 SPA)   │  Mobile / third-party (REST)     │
└──────────────┬───────────────────────────────────────────┘
               │ HTTP
┌──────────────▼───────────────────────────────────────────┐
│  server   HTTP adapter · /api/v1 · middleware · ETag      │
├───────────────────────────────────────────────────────────┤
│  service  use cases · transaction boundaries · tree build │
├───────────────────────────────────────────────────────────┤
│  core     domain models · ports (interfaces) · errors     │
├────────────┬───────────────┬──────────────────────────────┤
│  store     │  index        │  textproc                    │
│ filesystem │ SQLite + FTS5 │ CJK tokenizer / snippets     │
└────────────┴───────────────┴──────────────────────────────┘
```

- **Layered & dependency-inverted:** `service` depends only on interfaces in `core`,
  so backends can be swapped (e.g. S3 assets) without touching business logic.
- **Adapters stay thin:** the same `service` powers REST, WebDAV, and MCP.
- **Full details:** see [`docs/DESIGN.md`](docs/DESIGN.md) (architecture decision records included).

---

## Development

```bash
make dev        # run backend with embedded frontend
make test       # go test ./...
make test-web   # frontend type-check (vue-tsc)
make lint       # golangci-lint + vue-tsc
make fmt        # gofmt + prettier
make build      # build the single binary
make docker     # build the Docker image
```

Requirements: **Go 1.26+**, **Node 22+**, and **Docker** (optional).

---

## Roadmap

- [x] Folder hierarchy, Markdown storage, FTS5 search
- [x] Rich editor (images, tables, slash commands, TOC)
- [x] Wiki-links & backlinks
- [x] Multi-user auth, WebDAV, public sharing
- [x] MCP server, AI assistant (chat / organize / complete)
- [x] Version history, trash, incremental indexing + file watcher
- [x] S3 assets, PWA, OpenAPI docs, settings center
- [ ] Mobile-native app
- [ ] Collaboration / real-time editing
- [ ] Graph view & tags management UI

See [`docs/DESIGN.md`](docs/DESIGN.md) for the full roadmap.

---

## Contributing

Contributions of all kinds are welcome — bug reports, feature ideas, docs, and code.
Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and open an issue or pull request.
Good first issues are labeled `good first issue`.

## License

[MIT](LICENSE) © Overview contributors
