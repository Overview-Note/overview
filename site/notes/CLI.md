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

## Relationship to the server

The CLI and the HTTP server share the same `service` layer (see
[[Architecture]]), so behaviour is identical: optimistic concurrency
(`--if-version`), revision snapshots, trash, and reindexing on write.
