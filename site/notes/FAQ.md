---
id: 01JFAQ00000000000000000000
title: FAQ
public: true
---

# FAQ

## Is my content locked in?

No. Every note is a Markdown file with YAML frontmatter under `data/notes`. You can open
it with any editor, track it with Git, or move it to another tool. The SQLite database is
only an index and can be deleted at any time.

## Does it support folders and nesting?

Yes — physical folders are the navigation tree, with unlimited depth.

## How does search work with Chinese?

Overview indexes CJK text as unigrams + bigrams, so substring queries work
(`发模` matches `并发模型`) without an external search engine.

## Can I use it with Obsidian?

Yes, via WebDAV at `http://<host>:5230/dav/`. External edits are re-indexed automatically.

## Can AI agents use it?

Yes — the built-in MCP server (protocol `2026-07-28`) exposes 22 tools covering notes,
folders, history, trash, assets and reindex, mirroring the CLI and REST API. The same tool
surface powers the built-in **AI agent**, which can execute those actions in-app using
OpenAI-compatible function calling; dangerous operations require explicit confirmation and
every call is audited.

## Can users sign up or reset their password?

Yes. Admins can invite users by email, and configure an SMTP server (or the `OVERVIEW_MAIL_*`
variables) for verification and password-reset mail. Self-registration and the login-page
notice/ICP/link are toggled in **Settings → Site**.

## Is there a command-line tool?

Yes. `overview <command>` runs against a data directory with no server:
`list/search/read/write/move`, `history/restore`, `trash`, `asset-upload`, `import/export-zip`,
and `build` to render a deployable static site. See [[CLI]].

## Is there a mobile app?

The web app is a PWA and can be installed, and it ships a responsive phone layout (drawer
sidebar, bottom-sheet panels, touch-friendly controls). A native app is on the roadmap.

## Can I store attachments in S3?

Yes. Set the `OVERVIEW_S3_*` variables, or configure it at runtime in **Settings → Object
storage** (admin only). The settings page value wins once saved; switching backends does not
migrate existing assets.

## How do I back up?

Copy the entire `data/` directory. That's it.

## License?

MIT.
