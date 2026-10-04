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
│  agent    bounded tool loop · sessions · confirmation     │
├───────────────────────────────────────────────────────────┤
│  tools    single capability source (MCP + agent)          │
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
3. **Thin adapters.** The same `service` powers REST, WebDAV, MCP, the offline CLI **and the
   in-app AI agent**; the tool surface lives once in `tools`, and `mcp` is only a JSON-RPC
   shell over it.
4. **Safe writes.** Atomic writes (temp → fsync → rename) and optimistic concurrency
   (content hash / ETag → 409).

## Packages

| Package | Responsibility |
| --- | --- |
| `core` | Domain models, ports, sentinel errors |
| `service` | Use cases (notes, search, auth, email/mail, site settings, capture, links, archive, AI) |
| `store` | Filesystem note/asset repository |
| `index` | SQLite + FTS5 index, migrations |
| `textproc` | CJK tokenizer, wiki-links, snippets |
| `markdown` | Frontmatter parse/serialize |
| `history`, `trash` | Revision snapshots (with retention), soft delete |
| `watcher` | fsnotify-based live reindex |
| `archivex` | Portable ZIP export/import |
| `s3store` | S3-compatible asset backend |
| `tools` | Single capability source shared by MCP and the AI agent (`Defs/Exec/Risk/Allows/Preview`) |
| `agent` | AI agent: bounded tool loop, in-process sessions, two-phase confirmation, audit |
| `mcp` | MCP server (protocol `2026-07-28`, dual-era); thin JSON-RPC adapter over `tools` |
| `cli` | Offline command-line adapter (notes, history, trash, assets, static site) |
| `openapi` | OpenAPI spec (drives `tools` schemas) |
| `sitegen` | Static documentation-site export (HTML + client-side search) |
| `ai` | OpenAI-compatible chat client (including function/tool calling) |
| `config`, `logging` | Environment config, structured logging |
| `server`, `webui` | HTTP routing/middleware, static handler (gzip + caching), embedded frontend |

## Testing

- **Go:** `go test ./...` — unit tests per package plus `httptest` integration tests.
- **Frontend:** `npm test` (Vitest) for pure logic, `vue-tsc` type-check, `vite build`.
- **CI:** `.github/workflows/ci.yml` runs the backend race tests, frontend checks and
  `golangci-lint` on every push/PR. Pushing a `v*` tag builds a multi-arch image to GHCR
  (`.github/workflows/docker.yml`).
