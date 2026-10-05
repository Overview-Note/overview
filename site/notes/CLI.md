---
id: 01JCLI000000000000000000000
title: CLI
public: true
---

# CLI

Everyday vault operations are available as **command-line commands** that run
directly against the data directory — no HTTP server required. This makes
Overview usable as a plain Markdown editor/index over a folder of files, and
easy to drive from scripts, editors, and AI tools.

```bash
overview [global flags] <command> [args]
```

## Global flags

| Flag | Description |
| --- | --- |
| `-C, --data-dir DIR` | Vault data directory (default `$OVERVIEW_DATA_DIR` or `./data`) |
| `--json` | Machine-readable output where supported |

All `OVERVIEW_*` environment variables still apply, so `overview` and
`overview -C ./my-vault` target the same layout the server uses.

## Notes

```bash
overview list                                  # tree (add --json for JSON)
overview search "并发模型" --limit 10            # full-text search
overview read "Guide/Intro.md"                 # print the Markdown body
overview get "Guide/Intro.md"                  # metadata + body as JSON
overview write "Guide/Intro.md" --file draft.md --public
overview write "note.md" --stdin < draft.md     # body from stdin
overview delete "Guide/Intro.md"               # -> trash
overview move "Guide/Intro.md" "Archived/Intro.md"
overview rename "Intro.md" "Overview.md"
overview mkdir "Guide"
overview links "Guide/Intro.md"                # outgoing + backlinks
overview resolve Setup                          # resolve a [[wiki]] target
```

Flags may appear before or after the positional arguments.

## History and trash

```bash
overview history "Guide/Intro.md"              # list revisions
overview revision "Guide/Intro.md" <id>        # print a revision body
overview restore "Guide/Intro.md" <id>         # restore a revision
overview trash                                 # list trashed items
overview trash-restore <id>
overview trash-purge <id> | --all
```

## Assets and archives

```bash
overview asset-upload ./diagram.png            # uploads, prints JSON
overview assets-orphans                        # unreferenced assets
overview assets-purge
overview export-zip vault.zip                  # whole vault (notes + assets)
overview import vault.zip
```

## Scripting example

```bash
# Create/update a note from another tool, then verify it is searchable.
printf '# Deploy\n\nrun `make docker`\n' | overview -C ./data write "Ops/Deploy.md" --stdin
overview -C ./data search Deploy --json | jq -r '.[].path'
```

## Static site

Render the vault to a **deployable static site** — plain HTML, CSS and a
dependency-free client-side search index — ready for GitHub Pages, Netlify, S3,
or any static host:

```bash
overview build ./public                 # public notes only (default _site)
overview build ./public --base /docs/   # URL prefix for sub-path hosting
overview build ./public --all           # include notes not marked public
overview build ./public --title "My Docs"
overview build ./public --theme dark    # auto (default) | light | dark
```

Output contains one `.html` per note, `index.html`, `style.css`, the shared reading
stylesheet `content.css`, `search.js`, `search-index.json`, the runtime enhancer
`readonly-enhance.js` and — only for the features a page actually uses — a `vendor/`
folder (`vendor/highlight`, `vendor/katex`, `vendor/mermaid`). Local `assets/` are copied
recursively; when assets live in S3 the export points at `OVERVIEW_S3_PUBLIC_URL` instead
and copies nothing.

The generated site renders **exactly like the app's reading view**: the same
`content.css` and the same DOM contract for code blocks, task lists, footnotes, math,
Mermaid diagrams, tables, images and wiki-links. Highlighting, math and diagrams are
rendered in the browser by libraries vendored at the **same versions the app uses** and
loaded **on demand**, so the export is self-contained and offline-capable. The theme
defaults to `auto` (the visitor's system preference); `--theme light|dark` (or
`OVERVIEW_SITE_THEME`) pins it. `export` is an alias of `build`, so the older
`overview export` command keeps working.

## Relationship to the server

The CLI and the HTTP server share the same `service` layer (see
[[Architecture]]), so behaviour is identical: optimistic concurrency
(`--if-version`), revision snapshots, trash, and reindexing on write.
