---
id: 01JSTART000000000000000000
title: Getting Started
public: true
---

# Getting Started

## Quick start (Docker)

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
docker compose up -d --build
# open http://localhost:5230 and complete the first-run admin setup
```

## Single binary

Download the binary for your OS/arch from
[Releases](https://github.com/Overview-Note/overview/releases) and run it:

```bash
OVERVIEW_DATA_DIR=./data ./overview
# open http://localhost:5230
```

## Demo (no setup)

```bash
git clone https://github.com/Overview-Note/overview.git
cd overview
make demo            # serves demo/ on :5230 (first visit creates an admin)
```

## Local development

```bash
# backend (serves the embedded frontend) on :5230
go run ./cmd/overview

# frontend with hot reload on :5173 (proxies /api to :5230)
cd web && npm install && npm run dev
```

Requirements: **Go 1.26+**, **Node 22+**, Docker (optional). See [[Development]].

## First steps

1. Open the app and create your first note with **＋ Note**.
2. Organize with folders — the sidebar mirrors your directory tree (drag to move).
3. Type `/` in the editor for slash commands (tables, tasks, math, diagrams, code).
4. Paste or drop an image to upload it; use `[[Note]]` to link notes.
5. Toggle a note's visibility to **Public** to share a read-only page, or click
   **Public page** to view it in the docs-style layout.

## Next

- [[Features]]
- [[Deployment]]
- [[Development]]
