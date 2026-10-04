# Overview demo vault

A tiny, ready-to-run Markdown vault that showcases Overview's editor and search.
Every note is `public: true`, so the same content also works as a read-only
documentation site.

## Run it

```bash
# from the repository root
make demo
```

Then open <http://localhost:5230>. Authentication is enabled, so the first visit
shows the setup screen where you create an admin account (or sign in if one
already exists). Database and sessions live in `demo/overview.db`.

To browse without logging in (single-user mode, no user management):

```bash
OVERVIEW_AUTH=none OVERVIEW_DATA_DIR=./demo go run ./cmd/overview
```

## Browse as a read-only site

```bash
OVERVIEW_RENDER=true OVERVIEW_DATA_DIR=./demo OVERVIEW_SITE_TITLE="Overview Demo" go run ./cmd/overview
```

Then open <http://localhost:5230/public>.

## What it shows

- `欢迎.md` — landing page with tables and wiki-links
- `功能演示.md` — code highlighting, task lists, KaTeX math, Mermaid, footnotes
- `示例/功能总览.md` — index page that wiki-links every example below
- `示例/` — table alignment, task lists, multi-language code (Go/Bash/TS/Python/Vue),
  KaTeX math, Mermaid (flowchart / sequence / state / gantt), block quotes,
  footnotes and external links, plus a long document for the TOC
- `指南/` — nested folders, search, backlinks, AI/MCP, deployment, image mixing
  (`图文混排.md`), and a non-image attachment example (`附件与下载.md`)
