---
id: 01JHOME0000000000000000000
title: Overview
public: true
---

# Overview

**A Markdown knowledge base you fully own.**

Overview is a self-hosted, folder-based Markdown knowledge base with built-in search,
a rich editor, multi-user auth, WebDAV, an AI assistant, a tool-calling AI agent, an MCP
server and an offline CLI.
Content is stored as plain Markdown files; the SQLite index is fully rebuildable.

## Why Overview?

- **Files are the truth** — every note is a Markdown file with YAML frontmatter. Point
  Git, Obsidian, or any editor at the same `data/` folder.
- **Hierarchy, not a flat feed** — folders on disk are the document tree.
- **AI-native** — an MCP server (protocol `2026-07-28`) lets AI agents read and write notes;
  a built-in assistant can chat, organize and complete, and an **agent** can perform in-app
  actions with dangerous operations gated behind confirmation.
- **Scriptable** — an offline CLI performs every vault operation without a server and can
  render a deployable static site.
- **Boring to operate** — one Go binary with the frontend embedded. `docker run` and go.

## Start here

- [[Getting Started]]
- [[Features]]
- [[CLI]]
- [[Deployment]]
- [[Architecture]]
- [[MCP and AI]]
- [[Development]]
- [[FAQ]]

## Links

- Source: <https://github.com/Overview-Note/overview>
- License: MIT
