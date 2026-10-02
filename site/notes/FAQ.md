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

Yes — the built-in MCP server exposes list/search/read/write/delete tools.

## Is there a mobile app?

The web app is a PWA and can be installed. A native app is on the roadmap.

## How do I back up?

Copy the entire `data/` directory. That's it.

## License?

MIT.
