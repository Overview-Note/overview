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
- **Images** — paste / drag-and-drop with optional client-side compression
- **Slash commands** — type `/` for headings, lists, tables, code blocks
- **Outline (TOC)** — scroll-linked table of contents
- **Table bubble menu** — row/column controls above the active table

## Search

- **Full-text search** — SQLite FTS5 with a custom CJK unigram + bigram tokenizer for
  real substring matching (e.g. `发模` matches `并发模型`)
- **Global search** with `Ctrl`/`Cmd` + `K`

## AI-native

- **AI assistant** — chat, *organize* and *complete* notes
- **MCP server** — expose notes as tools for AI agents

## Access & integration

- **Multi-user auth** — bcrypt, sessions, roles, first-run setup
- **WebDAV** — mount the vault in Obsidian, Finder, or mobile apps
- **REST API** — versioned under `/api/v1`, OpenAPI at `/api/docs`
- **PWA** — installable app shell

## Operations

- **Single binary** with the frontend embedded
- **Version history** and **trash** with restore
- **Live sync** — a file watcher reindexes external edits
- **Pluggable assets** — local filesystem or any S3-compatible store

See [[MCP and AI]] for integration details.
