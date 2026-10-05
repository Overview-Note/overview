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
| `app` | Application wiring and lifecycle shared by the headless and desktop binaries: instance lock, port file, listener, graceful shutdown, external-file import, sync-engine lifecycle |
| `sync` | Desktop ↔ server vault sync engine: HTTPS/loopback remote client, on-disk state (`sync.json` + `.sync-token`), three-way reconciliation by note `id`, conflict copies, direction control, path-based asset sync |
| `update` | GitHub Releases version check (report only; never downloads or installs) |
| `config`, `logging` | Environment config, structured logging (fault-tolerant fanout so GUI builds without stdout still log to a file) |
| `server`, `webui` | HTTP routing/middleware, static handler (gzip + caching), embedded frontend, sync endpoints (`/sync/manifest`, `raw` note writes, `PUT /assets/{path}`, `/desktop/sync*`) |

## Desktop shell

The native app (`cmd/overview-desktop`) is a **Wails v3** shell over the same `internal/app`
server. It owns only the OS-facing concerns — native window, system tray, single instance,
`.md` file associations, `overview://` deep links, drag-and-drop and launch-at-login — while
every piece of business logic stays in the shared packages. The shell source is guarded by
`//go:build windows || darwin || desktop`, so a plain `CGO_ENABLED=0 go build ./...` still
works without GTK headers.

Local-only mode tightens the server instead of forking it: it listens on loopback, defaults
to `auth=none` and disables MCP/WebDAV, and adds a **Host/origin guard** (loopback Host plus
same-origin state-changing requests) in place of cookie CSRF protection. The
`/api/v1/desktop/*` endpoints exist only in local mode — a headless server answers `404`.
The lifecycle is shared: both binaries call `app.New`/`Start`/`Stop`, and a data-directory
instance lock plus Wails' single-instance channel keep one server and one window. See
[[Desktop]].

## Vault sync

The desktop shell attaches the `sync` engine to the shared app through a `SyncFactory`; the
engine is started and stopped with the app and also implements the server's `SyncHooks`, so
the same value backs the desktop-only `/api/v1/desktop/sync*` endpoints (a headless server
registers none of them and answers `404`). The server side adds a change counter
(`notes.changed_seq`) and deletion tombstones (migration `0010`), a persistent vault id, a
strong-ETag `GET /api/v1/sync/manifest`, faithful raw note writes and path-based asset
writes. The client keeps its bookkeeping in `<DataDir>/sync.json` with the API token in the
separate `<DataDir>/.sync-token`. See [[Desktop]] for the user-facing behaviour and
[`docs/DESIGN.md`](https://github.com/Overview-Note/overview/blob/main/docs/DESIGN.md)
(ADR-067…076) for the design.

## Testing

- **Go:** `go test ./...` — unit tests per package plus `httptest` integration tests,
  including `sync` (three-way reconciliation, direction guards, conflict copies, state/token
  handling) and the sync HTTP endpoints (manifest ETag/304, raw writes, asset path guards).
- **Frontend:** `npm test` (Vitest) for pure logic, `vue-tsc` type-check, `vite build`.
- **CI:** `.github/workflows/ci.yml` runs the backend race tests, frontend checks and
  `golangci-lint` on every push/PR, plus a Windows desktop-shell compile smoke test. Pushing a
  `v*` tag builds a multi-arch image to GHCR (`.github/workflows/docker.yml`) and builds the
  desktop shells for Windows/macOS/Linux (`.github/workflows/desktop.yml`), publishing them as
  release assets.
