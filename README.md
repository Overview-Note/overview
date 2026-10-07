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

![Overview editor](assets/preview.png)

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
| Built-in AI assistant + tool-calling agent | ❌ | plugin | ✅ |
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
- **Code highlighting**, **task lists**, **math (KaTeX)** and **Mermaid diagrams**, plus
  **GFM footnotes** — all stored as plain Markdown
- **Images** — paste / drag-and-drop, with optional **client-side compression** (WebP/JPEG)
- **File attachments** — upload any file type and insert it as a download link
  (non-inline types are served with `Content-Disposition: attachment`, RFC 5987 filenames)
- **Slash commands** — type `/` for headings, lists, tasks, tables, math, diagrams, code blocks
- **Outline (TOC)** — scroll-linked table of contents
- **Table bubble menu** — row/column controls appear right above the active table
- **Focus mode** (`F9`) — hides the top bar and sidebar, with a low-key floating
  **Exit focus** button in the bottom-right corner as a visible way out (Esc and `F9` also work) —
  and a **keyboard-shortcuts** panel (`?`)
- **Create from the home empty state** — the landing screen offers **New note** / **New
  folder**, so you can start writing even when focus mode hides the sidebar

### 🔎 Search & navigation
- **Full-text search** with SQLite FTS5 and a custom **CJK unigram + bigram tokenizer**
  for real substring matching (e.g. `发模` matches `并发模型`)
- **Global search** with `Ctrl`/`Cmd` + `K`

### 🤖 AI-native
- **AI assistant** — chat, *organize* and *complete* notes, using any OpenAI-compatible
  endpoint (OpenAI, DeepSeek, Ollama, vLLM, …)
- **AI agent (tool calling)** — the built-in assistant can **perform in-app actions**, not just
  chat: list, search, read, write, move, rename and delete notes and attachments via
  OpenAI-compatible function calling. It runs a bounded loop (8 steps by default) through a
  single capability source (`internal/tools`) shared with MCP. **Dangerous actions are never
  run blindly** — they pause with a human-readable preview and require an explicit
  approve/reject in the editor's *Agent* tab; tools are trimmed by role and every call is
  audited with redacted arguments
- **MCP server** — expose notes as tools so AI agents can list/search/read/write — 22 tools
  (notes, folders, history, trash, assets, reindex, public notes) matching the CLI/REST surface
- **API tokens** — create and revoke long-lived bearer tokens in **Settings → API tokens**

### 🔐 Access & integration
- **Multi-user auth** — bcrypt, sessions, admin/member roles, first-run setup wizard
- **Email accounts** — invite users, email verification and password reset (SMTP configurable at
  runtime), plus optional **self-registration** and a customizable login page
- **WebDAV** — mount the vault in Obsidian, Finder, or mobile apps
- **REST API** — versioned under `/api/v1`, documented via OpenAPI at `/api/docs`
- **PWA** — installable app shell with a responsive **mobile layout**: a drawer sidebar
  (hamburger + backdrop), a phone-friendly top bar, full-width editor, horizontally
  scrolling toolbars, right-side panels as a bottom sheet (outline/backlinks/history/AI),
  a mobile search overlay, and touch basics (tap feedback, 16px inputs, ≥40px targets,
  `100dvh` + safe-area insets)
- **Quick capture** — paste a page URL into the top-bar dialog; the server fetches the title/body
  (SSRF-guarded) so you can save it as a note in the folder you pick

### 🛠 Operations
- **Single binary** with the frontend embedded (`docker run` or `go run`)
- **Structured logging** — JSON or text, to stdout and/or a rotating log file
- **SQLite migrations**, atomic writes, optimistic concurrency (ETag / 409)
- **Version history** (revision snapshots) and **trash** (soft delete + restore)
- **Live sync** — a file watcher reindexes external edits
- **Portable archive** — export/import the whole vault (notes + assets) as a ZIP
- **CLI** — every vault operation (list, read, write, search, move, history, trash, …)
  runs directly against `data/` with no server, for scripts and tools
- **Pluggable storage** — local filesystem or any S3-compatible object store, switchable at
  runtime under **Settings → Object storage** (no restart; existing assets are not migrated)
- **Settings page** — standalone `/settings` route: appearance (theme, font, language,
  **accent color**), editor, AI, mail server, site, object storage, data, user management,
  API tokens
- **Static export & sitemap/robots** — publish public notes as a read-only site that renders
  **exactly like the app's reading view**: a single shared stylesheet (`content.css`) and the
  same DOM contract for code, tasks, footnotes, math, diagrams, tables, images and wiki-links.
  Heavy renderers (highlight.js, KaTeX, Mermaid) ship at the **same versions as the app** and
  load **on demand**, so the export works offline and a site with no math/diagrams carries no
  extra weight

### 🖥 Desktop app
- **Native window** — the same Go service runs inside a **Wails v3** shell (an embedded
  WebView pointing at `http://127.0.0.1`), sharing `internal/app` and every service with the
  headless binary
- **System integration** — system tray, single instance, `.md` **file association**,
  `overview://` **deep links**, drag-and-drop, launch at login, and a first-run **onboarding
  wizard** to use the default vault, open an existing one, or create a new one
- **Vault management** — the resolved vault path is remembered in
  `<UserConfigDir>/Overview/desktop.json`; under **Settings → Desktop** you can **change the
  data directory** (native picker), **open it** in the file manager, and **restart the app**
  to apply the change
- **Local-first defaults** — loopback only, authentication off, MCP/WebDAV disabled; a
  Host/origin guard replaces cookie CSRF protection. `OVERVIEW_DESKTOP=true` applies these
  defaults and any of them can be overridden with the usual environment variables. With
  authentication off the caller is treated as the **owner (admin)**, so every admin setting —
  mail, site, object storage, data, users and API tokens — is reachable without a login
- **Update *notification* only** — checks GitHub Releases and links to the download page;
  it never downloads or installs anything silently
- **Per-user data** — Windows/macOS use `~/Documents/Overview`; Linux prefers
  `~/Documents/Overview` and falls back to the XDG data directory
- **Platform builds** — Windows (WebView2, no CGO) and macOS (CGO) build directly; Linux
  needs `-tags desktop` plus GTK/WebKitGTK. Tagged releases publish desktop binaries; signed
  Windows NSIS / macOS DMG installers are the next step
- **No console window** — Windows desktop builds use the GUI PE subsystem (`-H=windowsgui`),
  and desktop logs default to `<DataDir>/logs/desktop.log` (a GUI process has no stdout)
- **Upgrades load the new frontend automatically** — the shell opens the webview with the
  build version in the URL (`/?desktop=1&v=<version>`), so every release is a fresh WebView2
  cache entry; there is no stale-bundle cache to clear after upgrading

### 🔁 Desktop ↔ server sync

Keep a desktop vault and a self-hosted Overview server in step, the way Trilium does
client ↔ server synchronisation:

- **Configure in the app** — open **Settings → Sync** (desktop only) and enter the server
  URL plus an API token (create one under **Settings → API tokens** on the server). The
  token is stored separately in `<DataDir>/.sync-token` (never in the exportable state file
  or logs), and plain HTTP is only accepted for loopback hosts — remote servers must be HTTPS
- **Three-way reconciliation by note `id`** — the server exposes `GET /api/v1/sync/manifest`
  with a strong `ETag`; the client compares that inventory against its local vault and its
  last-synced state, then pulls, pushes, deletes and moves notes and attachments. Notes
  without a frontmatter `id` are assigned one on the first sync
- **Real-time + incremental** — the server broadcasts changes over Server-Sent Events at
  `GET /api/v1/sync/events` and exposes a cursor-based `GET /api/v1/sync/changes`
  (`since` note sequence + `sinceTs` tombstone time); the client subscribes to the stream and
  reconciles on change (debounced), persisting the cursor, while interval polling remains as a
  fallback. Events are a hint — the manifest comparison is still what guarantees convergence
- **Empty folders sync too** — folder creations, moves and deletions ride the same change
  sequence (migrations `0011`/`0012`), so a folder that holds no notes still propagates
- **Direction** — `both` (default), `pull` (server → desktop) or `push` (desktop → server)
- **Conflicts are never overwritten** — when both sides edit the same note, the local copy is
  written to a sibling `<name> (conflict-<device>-<ts>).md` and the remote original is kept;
  resolve each one in **Settings → Sync** with *keep local* or *keep remote*
- **Faithful writes** — sync writes the exact Markdown bytes (frontmatter preserved) with a
  `baseVersion`, so it never silently clobbers a concurrent edit
- **Attachments everywhere** — assets sync by vault-relative path, including on **S3** backends
  (`s3store.Restore`); an attachment over the server's upload limit is skipped with a warning
  rather than failing the whole sync
- **Limits** — a server is still a single shared vault (no per-user ACL), and chunked uploads
  for oversized files are not implemented yet

---

## Quick Start

### Try the demo (no setup)

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
make demo            # serves ./demo on http://localhost:5230, auth disabled
```

The `demo/` vault showcases code highlighting, task lists, KaTeX math, Mermaid
diagrams, footnotes and wiki-links. See [`demo/README.md`](demo/README.md).

### Docker (recommended)

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
docker compose up -d --build
# open http://localhost:5230 and complete the first-run admin setup
```

### Prebuilt binary

Download the binary for your OS/arch from
[Releases](https://github.com/Overview-Note/overview/releases), or install with Go:

```bash
go install github.com/Overview-Note/overview/cmd/overview@latest
OVERVIEW_DATA_DIR=./data overview
```

Open http://localhost:5230 and complete the first-run admin setup.

### Desktop app

Build the native shell for your platform (the frontend is bundled first):

```bash
make build-desktop      # -> bin/overview-desktop
make package-desktop    # + tar.gz archive for the host OS/arch
```

- **Windows** — WebView2 runtime must be installed; builds without CGO:
  `go build ./cmd/overview-desktop`
- **macOS** — CGO + Xcode command line tools:
  `go build -tags desktop ./cmd/overview-desktop`
- **Linux** — CGO + GTK/WebKitGTK (GTK4 + WebKitGTK 6.0 by default, GTK3 with `gtk3`):
  ```bash
  sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev   # GTK4 (Ubuntu 24.04)
  # or: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev   # GTK3 (Ubuntu 22.04)
  go build -tags desktop ./cmd/overview-desktop
  go build -tags "desktop gtk3" ./cmd/overview-desktop
  ```

Tagged releases (`v*`) build the Windows/macOS/Linux shells in CI and attach them to the
GitHub Release. See [`docs/DESIGN.md`](docs/DESIGN.md) (ADR-059…066) for the design, and
[`docs/PROJECT.md`](docs/PROJECT.md) for the current desktop limitations.

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
| `OVERVIEW_HISTORY_KEEP` | `50` | Revisions kept per note (`0` disables pruning) |
| `OVERVIEW_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `OVERVIEW_LOG_FORMAT` | `json` | `json` or `text` |
| `OVERVIEW_LOG_FILE` | — | Log file path (stdout only when unset); rotates on size. Desktop defaults to `<DataDir>/logs/desktop.log` |
| `OVERVIEW_LOG_MAX_MB` | `10` | Max log file size before rotation |
| `OVERVIEW_LOG_BACKUPS` | `3` | Rotated files to keep (`0` truncates) |
| `OVERVIEW_AUTH` | `multi` | `multi` (users + login) or `none` |
| `OVERVIEW_SITE_TITLE` | `Overview` | Site title (UI and render mode) |
| `OVERVIEW_SITE_THEME` | `auto` | Static-site theme: `auto` (system), `light`, `dark` |
| `OVERVIEW_RENDER` | `false` | `true` → public read-only docs site |
| `OVERVIEW_BASE_URL` | — | External origin used in email links (else derived per request) |
| `OVERVIEW_MCP_TOKEN` | — | Static bearer token for MCP; prefer UI-managed API tokens |
| `OVERVIEW_AI_BASE_URL` | — | OpenAI-compatible base URL (enables AI when set) |
| `OVERVIEW_AI_API_KEY` | — | AI provider API key |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | AI model name |
| `OVERVIEW_MAIL_HOST` | — | SMTP host (with `OVERVIEW_MAIL_FROM`, enables email) |
| `OVERVIEW_MAIL_PORT` | `587` | SMTP port |
| `OVERVIEW_MAIL_USERNAME` | — | SMTP username (empty → no auth) |
| `OVERVIEW_MAIL_PASSWORD` | — | SMTP password |
| `OVERVIEW_MAIL_FROM` | — | From address |
| `OVERVIEW_MAIL_STARTTLS` | `true` | Use STARTTLS |
| `OVERVIEW_S3_BUCKET` | — | Enables S3-compatible asset storage when set |
| `OVERVIEW_S3_ENDPOINT` | — | S3 endpoint (e.g. `s3.amazonaws.com`) |
| `OVERVIEW_S3_REGION` | `us-east-1` | S3 region |
| `OVERVIEW_S3_ACCESS_KEY` / `OVERVIEW_S3_SECRET_KEY` | — | S3 credentials |
| `OVERVIEW_S3_USE_SSL` | `true` | Use HTTPS for S3 |
| `OVERVIEW_S3_PUBLIC_URL` | — | Optional CDN / public URL prefix |
| `OVERVIEW_DESKTOP` | `false` | `true` applies desktop defaults: loopback address, `auth=none`, MCP/WebDAV off, per-user data dir |
| `OVERVIEW_ENABLE_MCP` | `true` (off on desktop) | Enable the `/mcp` endpoint |
| `OVERVIEW_ENABLE_DAV` | `true` (off on desktop) | Enable the `/dav/` endpoint |

> AI and mail can also be configured at runtime in the **Settings** page (admin only) —
> **Settings → AI assistant** and **Settings → Mail server** — with no restart needed.
> Self-registration and the login-page notice/ICP/link are configured under **Settings → Site**.
>
> The asset backend is configured under **Settings → Object storage** (admin only).
> Leave the bucket empty to keep using local storage. The `OVERVIEW_S3_*` variables seed
> the initial value; once a configuration is saved it takes precedence across restarts.
> Switching backends is immediate: new uploads and reads use the active backend, but
> existing assets are **not** migrated between local storage and the bucket.
>
> Desktop vault sync is configured in the app under **Settings → Sync** (server URL + API
> token, direction, interval) and adds **no** environment variables. The token is kept in
> `<DataDir>/.sync-token`; the (secret-free) state and conflict list live in
> `<DataDir>/sync.json`.

`overview export` (static site) additionally reads `OVERVIEW_EXPORT_DIR` (default `_site`)
and `OVERVIEW_EXPORT_BASE` (URL prefix, default `/`).

---

## Integrations

**WebDAV**

```
http://<host>:5230/dav/
```
Sign in with your Overview username and password (HTTP Basic). Edits saved over WebDAV
are re-indexed automatically.

**MCP (Model Context Protocol)**

Point an MCP client at `http://<host>:5230/mcp` with `Authorization: Bearer <token>`.
Implements the current spec (`2026-07-28`: stateless `_meta`, `server/discover`,
`resultType`) while remaining compatible with older `initialize`-handshake clients.
The tool surface mirrors the CLI/REST API: notes (list/search/read/write/delete/move/
rename/links/resolve), folders, history (list/read/restore), trash (list/restore/purge),
assets (upload/orphans/purge), reindex and public notes.

Tokens: create and revoke **API tokens** in the web UI (**Settings → API tokens**, admin only);
the secret is shown once and also works as a REST bearer token. Alternatively set the
static `OVERVIEW_MCP_TOKEN` on the server.

**OpenAPI**

- Interactive docs: `/api/docs`
- Machine-readable spec: `/api/v1/openapi.json`

---

## Command-line interface

The same operations exposed by the REST API are available as commands that run
**directly against the data directory** (no server). Handy for scripting, editors,
and AI tools — Overview becomes a plain Markdown editor/index over a folder.

```bash
overview list                                   # note tree
overview search "并发模型" --limit 10
overview read "Guide/Intro.md"                  # print the Markdown body
overview write "Guide/Intro.md" --file draft.md --public
printf '# Deploy\n\nnotes\n' | overview write "Ops/Deploy.md" --stdin
overview move "Guide/Intro.md" "Archived/Intro.md"
overview history "Guide/Intro.md"               # revisions
overview restore "Guide/Intro.md" <id>
overview export-zip vault.zip && overview import vault.zip
overview build ./public --all                   # deployable static site
```

Global flags: `-C, --data-dir DIR` (target a vault) and `--json` (machine-readable
output). Run `overview help` for the full list. See [[CLI]] in the docs site.

---

## Documentation site

This project's own documentation is **built with Overview** (dogfooding). The notes live
under [`site/notes`](site) and are rendered two ways:

- **Live server** — read-only *render mode* (no login, only public notes):
  ```bash
  make site            # http://localhost:5230
  ```
- **Static export** — for GitHub Pages and any static host:
  ```bash
  make site-export     # outputs ./_site
  ```

The static export is **rendering-identical to the app's reading view** and works fully
offline. The app, the public pages and the export all share one stylesheet
(`internal/sitegen/content.css`) and one content DOM contract, so a note looks the same in
the editor, on a public page and in the published site. Syntax highlighting, math (KaTeX)
and Mermaid diagrams are produced in the browser by libraries vendored at the **same
versions the app uses**, loaded **on demand** — a page with no math, diagrams or code pulls
none of them.

The export is deployed automatically to GitHub Pages by
[`.github/workflows/pages.yml`](.github/workflows/pages.yml).

## Data Layout

```
data/
├── notes/        # Markdown files — the source of truth (folders = hierarchy)
├── assets/       # uploaded attachments (or S3 when configured)
├── .history/     # note revision snapshots
├── .trash/       # soft-deleted notes
├── overview.db   # SQLite index (safe to delete; rebuilt on startup)
├── sync.json     # desktop sync state (no secrets)
├── .sync-token   # desktop sync API token (owner-only permissions)
└── logs/         # desktop shell logs (desktop.log by default)
```

The path above is relative to the server's working directory for self-hosted/Docker use.
The **desktop app** defaults to a per-user vault — `~/Documents/Overview` on Windows/macOS,
and `~/Documents/Overview` or the XDG data directory on Linux — and asks you to choose a
folder on first run.

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

- **Layered & dependency-inverted:** `service` depends only on interfaces in `core`,
  so backends can be swapped (e.g. S3 assets) without touching business logic.
- **Adapters stay thin:** the same `service` powers REST, WebDAV, and MCP.
- **Full details:** see [`docs/DESIGN.md`](docs/DESIGN.md) (architecture decision records included).

---

## Development

```bash
make demo       # run the sample vault in demo/ on :5230 (first visit creates an admin)
make dev        # run the backend with the embedded frontend
make test       # run all tests (Go + frontend Vitest)
make test-web   # frontend type-check + unit tests
make lint       # golangci-lint + vue-tsc
make fmt        # gofmt + prettier
make build      # build the single binary (frontend embedded)
make docker     # build the Docker image
```

Requirements: **Go 1.26+**, **Node 22+**, and **Docker** (optional).

---

## Roadmap

**Shipped**

- [x] Folder hierarchy, Markdown storage, FTS5 search
- [x] Rich editor (images, tables, slash commands, TOC)
- [x] Wiki-links & backlinks
- [x] Multi-user auth, WebDAV, public sharing
- [x] MCP server, AI assistant (chat / organize / complete)
- [x] Version history, trash, incremental indexing + file watcher
- [x] S3 assets, PWA, OpenAPI docs, settings center
- [x] Code highlighting, task lists, math (KaTeX), Mermaid diagrams, footnotes
- [x] Drag-and-drop tree, ZIP import/export, in-app quick capture, focus mode
- [x] Sitemap/robots, public pages styled as a docs site
- [x] Render-mode API whitelist, history retention, static-site search, incremental WebDAV,
  full OpenAPI spec, lossless table/wiki-link round-trip
- [x] Offline CLI (notes/search/history/trash/assets/archives) and `build` static-site command
- [x] MCP upgraded to `2026-07-28` (stateless `_meta`, `server/discover`, `resultType`) with
  22 tools matching the CLI/REST surface
- [x] Performance: route code-splitting + gzip/immutable static caching (Lighthouse 99)
- [x] Note/document logo, soft warm theme, resizable stable sidebar, GHCR multi-arch images
- [x] Gridea-style palette, customizable accent color, compact sidebar
- [x] Standalone settings page (appearance/editor/AI/mail/site/data/users/tokens)
- [x] Email user management (invite, verification, password reset), self-registration, login notice
- [x] File attachments (any type) with download headers, anonymous read-only `/assets/`
- [x] In-app quick capture with SSRF guarding; static-site palette synced with the app
- [x] AI agent with tool calling: bounded loop, two-phase confirmation for dangerous actions,
  role-trimmed tools, prompt-injection defense and redacted tool audit
  (single capability source `internal/tools`, shared with MCP)
- [x] Responsive mobile layout (drawer sidebar, bottom-sheet panels, touch basics) and
  runtime-switchable S3 object storage under **Settings → Object storage**
- [x] Static docs site rendered identically to the app (shared `content.css`, aligned DOM
  contract, on-demand same-version highlight.js/KaTeX/Mermaid); public/editor/static three
  states unified on one contract
- [x] Native desktop app (Wails v3): shared lifecycle, tray/single-instance, `.md` and
  `overview://` integration, drag-and-drop, launch at login, local-only security guard,
  update notification, per-user data dir, and a cross-platform CI build matrix
- [x] Desktop ↔ server sync (Phase 1): manifest + strong ETag, three-way reconciliation by
  note id, faithful `raw` writes with `baseVersion`, conflict copies with keep-local/remote
  resolution, `both`/`pull`/`push` direction, path-based asset sync, and a desktop-only
  **Settings → Sync** section
- [x] Console-less Windows desktop build (`-H=windowsgui`) and desktop logging to
  `<DataDir>/logs/desktop.log` via a fault-tolerant fanout writer
- [x] Desktop onboarding and vault management: remember the vault in `desktop.json`, change /
  open / restart from **Settings → Desktop**; no-auth mode acts as the **owner** so all admin
  settings are reachable
- [x] Sync real-time + incremental: SSE `GET /sync/events`, cursor-based `GET /sync/changes`
  (note sequence + tombstone time), in-process `ChangeBus`, empty-folder sync (migrations
  `0011`/`0012`), S3 path-based asset writes (`s3store.Restore`), and clearer error codes
  (`unauthorized`/`forbidden`/`method_not_allowed`/`bad_gateway`)
- [x] Desktop upgrade visibility and focus escape: content-hashed weak ETags + 404 for missing
  assets (no more stale `index.html` served from 304), versioned webview URL
  (`/?desktop=1&v=<version>`) so each release loads a fresh bundle, and a floating
  **Exit focus** button when the top bar/sidebar are hidden
- [x] Darker, more readable sidebar tree: every folder depth uses the strongest text token
  (`--text`) and deep notes use `--text-muted`, so depth is conveyed by indentation and the
  uppercase/600-weight folder treatment rather than fading deeper levels
- [x] SPA fallback and a polished 404 experience: only `assets/*` and a fixed top-level file
  allowlist count as assets, so refreshing `/note/<path>.md` / `/public/<path>.md` serves the
  SPA again while missing assets still 404 (never masked by `index.html`); unknown paths now
  show a catch-all **404 view** instead of a blank screen, a missing note shows a matching
  overlay, the server falls back to a self-contained HTML 404 page when the frontend is not
  built, and the home empty state can create a note/folder even in focus mode

**Not yet done** (see [`docs/DESIGN.md`](docs/DESIGN.md) §12 for the full backlog)

- [ ] Signed desktop installers (Windows NSIS, macOS DMG) and Linux AppImage/`.desktop`
- [ ] Desktop verification on real macOS/Linux hardware (CI only compiles/archives today)
- [ ] Sync: chunked uploads for oversized files, per-user workspaces, multi-instance SSE
  fan-out, and real two-machine event-driven verification

- [ ] Tags: management UI, tag tree, tag filtering, `#` autocomplete
- [ ] Pin / favorites, saved filter views, timeline view
- [ ] Real-time collaboration (SSE/WebSocket), comments, notifications
- [ ] Version diff view, draft persistence
- [ ] Outbound webhooks, OIDC/SSO, gRPC, mobile-native app, more languages

See [`docs/DESIGN.md`](docs/DESIGN.md) for the full roadmap and known limitations.

---

## Contributing

Contributions of all kinds are welcome — bug reports, feature ideas, docs, and code.
Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and open an issue or pull request.
Good first issues are labeled `good first issue`.

## License

[MIT](LICENSE) © Overview contributors
