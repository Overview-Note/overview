---
id: 01JFEAT0000000000000000000
title: Features
public: true
---

# Features

## Content & organization

- **Hierarchical folders** — physical directories map to the navigation tree
- **Markdown as the source of truth** — Git-friendly and portable
- **Wiki-links & backlinks** — `[[Note]]` with a backlinks panel
- **Per-note visibility** — Private or Public share link

## Editing

- **Rich editor** — Vue 3 + Tiptap: headings, lists, quotes, code, images, tables
- **Code highlighting, task lists, math (KaTeX), Mermaid diagrams, GFM footnotes** — all
  stored as plain Markdown
- **Images** — paste / drag-and-drop with optional client-side compression
- **File attachments** — upload any file type and insert it as a download link; non-inline
  types are served with `Content-Disposition: attachment` (RFC 5987 filenames)
- **Slash commands** — type `/` for headings, lists, tasks, tables, math, diagrams, code
- **Outline (TOC)** — scroll-linked table of contents
- **Table bubble menu** — row/column controls above the active table
- **Focus mode** (`F9`) and a keyboard-shortcuts panel (`?`)

## Search

- **Full-text search** — SQLite FTS5 with a custom CJK unigram + bigram tokenizer for
  real substring matching (e.g. `发模` matches `并发模型`)
- **Global search** with `Ctrl`/`Cmd` + `K`

## AI-native

- **AI assistant** — chat, *organize* and *complete* notes
- **AI agent (tool calling)** — the assistant can **execute in-app actions**: read, search,
  write, move, rename and delete notes and attachments via OpenAI-compatible function
  calling. A bounded loop (8 steps by default) runs against a single capability source
  (`internal/tools`) shared with MCP. **Dangerous actions pause for an explicit
  approve/reject** with a human-readable preview; tools are trimmed by role and every call
  is audited with redacted arguments
- **MCP server** — protocol `2026-07-28` (stateless `_meta`, `server/discover`, `resultType`)
  with backward compatibility, exposing 22 tools that mirror the CLI/REST API

## Access & integration

- **Offline CLI** — `overview <command>` runs the same operations directly against `data/`
  with no server (notes, search, history, trash, assets, archives, static-site build)
- **Multi-user auth** — bcrypt, sessions, roles, first-run setup
- **Email accounts** — invite users, email verification and password reset (SMTP configurable at
  runtime under **Settings → Mail server**), optional **self-registration**, and a customizable
  login page (notice, ICP, link)
- **WebDAV** — mount the vault in Obsidian, Finder, or mobile apps
- **REST API** — versioned under `/api/v1`, OpenAPI at `/api/docs`
- **PWA** — installable app shell
- **Quick capture** — paste a page URL in the top-bar dialog; the server fetches the title and body
  (SSRF-guarded) and saves it as a note in the folder you pick

## Operations

- **Single binary** with the frontend embedded (Docker or `go run`); multi-arch images on GHCR
- **Structured logging** — JSON or text, to stdout and/or a size-rotated file
- **Portable archive** — export/import the whole vault (notes + attachments) as a ZIP
- **Version history** (with retention) and **trash** with restore
- **Live sync** — a file watcher reindexes external edits
- **Pluggable assets** — local filesystem or any S3-compatible store
- **Static export** with a client-side search and `sitemap.xml` / `robots.txt`

## Appearance & settings

- **Gridea-style palette** — warm amber accent (`#D4870E`) with light and dark variants,
  shared with the exported static site
- **Custom accent color** — pick a preset or any hex; the UI derives a light/dark ramp
- **Standalone settings page** (`/settings`) — appearance, editor, AI, mail, site, data,
  user management and API tokens

## Performance & UI

- **Fast first load** — routes are code-split and static assets are gzip-compressed with
  long-term caching (Lighthouse 99; initial JS ~154 KB)
- **Resizable sidebar** with stable hover states and tooltips for long names
- **Compact sidebar** (15px notes / 13px folders) and a soft warm theme with a clear hierarchy

See [[MCP and AI]] for integration details.
