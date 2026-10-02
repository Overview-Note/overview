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
3. **Thin adapters.** The same `service` powers REST, WebDAV and MCP.
4. **Safe writes.** Atomic writes (temp → fsync → rename) and optimistic concurrency
   (content hash / ETag → 409).

## Packages

| Package | Responsibility |
| --- | --- |
| `core` | Domain models, ports, sentinel errors |
| `service` | Use cases (notes, search, auth, links, AI) |
| `store` | Filesystem note/asset repository |
| `index` | SQLite + FTS5 index, migrations |
| `textproc` | CJK tokenizer, wiki-links, snippets |
| `markdown` | Frontmatter parse/serialize |
| `history`, `trash` | Revision snapshots, soft delete |
| `watcher` | fsnotify-based live reindex |
| `s3store` | S3-compatible asset backend |
| `mcp`, `openapi` | MCP server, OpenAPI spec |
| `server` | HTTP routing, middleware, handlers |
