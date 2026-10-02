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
- **MCP server** — protocol `2026-07-28` (stateless `_meta`, `server/discover`, `resultType`)
  with backward compatibility, exposing 22 tools that mirror the CLI/REST API

## Access & integration

- **Offline CLI** — `overview <command>` runs the same operations directly against `data/`
  with no server (notes, search, history, trash, assets, archives, static-site build)
- **Multi-user auth** — bcrypt, sessions, roles, first-run setup
- **WebDAV** — mount the vault in Obsidian, Finder, or mobile apps
- **REST API** — versioned under `/api/v1`, OpenAPI at `/api/docs`
- **PWA** — installable app shell
- **Quick capture** — a bookmarklet saves a page's title, URL and selection as a note

## Operations

- **Single binary** with the frontend embedded (Docker or `go run`); multi-arch images on GHCR
- **Structured logging** — JSON or text, to stdout and/or a size-rotated file
- **Portable archive** — export/import the whole vault (notes + attachments) as a ZIP
- **Version history** (with retention) and **trash** with restore
- **Live sync** — a file watcher reindexes external edits
- **Pluggable assets** — local filesystem or any S3-compatible store
- **Static export** with a client-side search and `sitemap.xml` / `robots.txt`

## Performance & UI

- **Fast first load** — routes are code-split and static assets are gzip-compressed with
  long-term caching (Lighthouse 99; initial JS ~154 KB)
- **Resizable sidebar** with stable hover states and tooltips for long names
- **Soft warm theme** (light/dark) with a clear accent and hierarchy

See [[MCP and AI]] for integration details.
