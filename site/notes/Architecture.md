---
id: 01JARCH0000000000000000000
title: Architecture
public: true
---

# Architecture

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

## Principles

1. **Files are the source of truth.** Notes are Markdown files; the database is a
   disposable, rebuildable index.
2. **Dependency inversion.** `service` depends only on interfaces declared in `core`,
   so backends (e.g. S3 assets) can be swapped without touching business logic.
3. **Thin adapters.** The same `service` powers REST, WebDAV, MCP **and the offline CLI**.
4. **Safe writes.** Atomic writes (temp → fsync → rename) and optimistic concurrency
   (content hash / ETag → 409).

## Packages

| Package | Responsibility |
| --- | --- |
| `core` | Domain models, ports, sentinel errors |
| `service` | Use cases (notes, search, auth, links, archive, AI) |
| `store` | Filesystem note/asset repository |
| `index` | SQLite + FTS5 index, migrations |
| `textproc` | CJK tokenizer, wiki-links, snippets |
| `markdown` | Frontmatter parse/serialize |
| `history`, `trash` | Revision snapshots (with retention), soft delete |
| `watcher` | fsnotify-based live reindex |
| `archivex` | Portable ZIP export/import |
| `s3store` | S3-compatible asset backend |
| `mcp` | MCP server (protocol `2026-07-28`, dual-era, 22 tools) |
| `cli` | Offline command-line adapter (notes, history, trash, assets, static site) |
| `openapi` | OpenAPI spec (also drives MCP tool schemas) |
| `sitegen` | Static documentation-site export (HTML + client-side search) |
| `ai` | OpenAI-compatible chat client |
| `config`, `logging` | Environment config, structured logging |
| `server`, `webui` | HTTP routing/middleware, static handler (gzip + caching), embedded frontend |

## Testing

- **Go:** `go test ./...` — unit tests per package plus `httptest` integration tests.
- **Frontend:** `npm test` (Vitest) for pure logic, `vue-tsc` type-check, `vite build`.
- **CI:** `.github/workflows/ci.yml` runs the backend race tests, frontend checks and
  `golangci-lint` on every push/PR. Pushing a `v*` tag builds a multi-arch image to GHCR
  (`.github/workflows/docker.yml`).
