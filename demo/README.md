# Overview demo vault

A tiny, ready-to-run Markdown vault that showcases Overview's editor and search.
Every note is `public: true`, so the same content also works as a read-only
documentation site.

## Run it

```bash
# from the repository root
make demo
```

Then open <http://localhost:5230>. Auth is disabled in demo mode, so there is no
login step.

Or run it directly:

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
- `指南/` — nested folders, search, backlinks, AI/MCP, deployment
