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
   shell over it. The asset backend is a `core.AssetStore` behind a `SwitchableAssetStore`,
   so it can be swapped between local storage and S3 at runtime.
4. **One responsive UI.** The Vue SPA adapts to phones by breakpoint (drawer sidebar,
   bottom-sheet panels, touch basics) rather than shipping a separate mobile build.
5. **Safe writes.** Atomic writes (temp → fsync → rename) and optimistic concurrency
   (content hash / ETag → 409).
6. **One rendering contract.** The editor reading state, public pages and the exported
   static site share a single stylesheet (`internal/sitegen/content.css`) and the same DOM
   contract; heavy renderers (highlight.js/KaTeX/Mermaid) are vendored at the app's versions
   and loaded on demand, so the export renders like the app and works offline.

## Packages

| Package | Responsibility |
| --- | --- |
| `core` | Domain models, ports, sentinel errors |
| `service` | Use cases (notes, search, auth, email/mail, site settings, capture, links, archive, AI, runtime storage config) |
| `store` | Filesystem note/asset repository |
| `index` | SQLite + FTS5 index, migrations |
| `textproc` | CJK tokenizer, wiki-links, snippets |
| `markdown` | Frontmatter parse/serialize |
| `history`, `trash` | Revision snapshots (with retention), soft delete |
| `watcher` | fsnotify-based live reindex |
| `archivex` | Portable ZIP export/import |
| `s3store` | S3-compatible asset backend |
| `service` (storage) | `SwitchableAssetStore` (runtime-swappable backend) + `StorageService` (validate/persist/swap) |
| `tools` | Single capability source shared by MCP and the AI agent (`Defs/Exec/Risk/Allows/Preview`) |
| `agent` | AI agent: bounded tool loop, in-process sessions, two-phase confirmation, audit |
| `mcp` | MCP server (protocol `2026-07-28`, dual-era); thin JSON-RPC adapter over `tools` |
| `cli` | Offline command-line adapter (notes, history, trash, assets, static site) |
| `openapi` | OpenAPI spec (drives `tools` schemas) |
| `sitegen` | Static documentation-site export: app-aligned DOM (`render.go`), shared `content.css`, on-demand same-version runtime enhancement (`vendor.go`), client-side search |
| `ai` | OpenAI-compatible chat client (including function/tool calling) |
| `config`, `logging` | Environment config, structured logging |
| `server`, `webui` | HTTP routing/middleware, static handler (gzip + caching), embedded frontend |

## Testing

- **Go:** `go test ./...` — unit tests per package plus `httptest` integration tests.
- **Frontend:** `npm test` (Vitest) for pure logic, `vue-tsc` type-check, `vite build`.
- **CI:** `.github/workflows/ci.yml` runs the backend race tests, frontend checks and
  `golangci-lint` on every push/PR. Pushing a `v*` tag builds a multi-arch image to GHCR
  (`.github/workflows/docker.yml`).
