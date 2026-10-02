---
id: 01JDEPL0000000000000000000
title: Deployment
public: true
---

# Deployment

## Single binary

Download the binary for your OS/arch from
[Releases](https://github.com/Overview-Note/overview/releases) and run it:

```bash
OVERVIEW_DATA_DIR=/var/lib/overview ./overview
```

## Prebuilt image (GHCR)

Multi-arch images (linux/amd64, linux/arm64) are published to the GitHub
Container Registry on every `v*` tag:

```bash
docker run -d -p 5230:5230 \
  -v ov-data:/data \
  -e OVERVIEW_ADDR=":5230" \
  -e OVERVIEW_DATA_DIR=/data \
  -e OVERVIEW_AUTH=multi \
  ghcr.io/overview-note/overview:latest
```

Tags: `latest`, `X.Y.Z`, and `X.Y`.

## Docker Compose

```yaml
services:
  overview:
    image: overview:latest
    build: .
    ports:
      - "5230:5230"
    volumes:
      - ./data:/data
    environment:
      OVERVIEW_ADDR: ":5230"
      OVERVIEW_DATA_DIR: /data
      OVERVIEW_AUTH: multi
    restart: unless-stopped
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `OVERVIEW_ADDR` | `:5230` | Listen address |
| `OVERVIEW_DATA_DIR` | `./data` | Data root |
| `OVERVIEW_DB` | `<data>/overview.db` | SQLite index path |
| `OVERVIEW_MAX_UPLOAD_MB` | `32` | Upload size limit |
| `OVERVIEW_HISTORY_KEEP` | `50` | Revisions kept per note (`0` disables pruning) |
| `OVERVIEW_AUTH` | `multi` | `multi` or `none` |
| `OVERVIEW_SITE_TITLE` | `Overview` | Site title (UI and render mode) |
| `OVERVIEW_SITE_THEME` | `auto` | Static-site theme: `auto` (follow system), `light` or `dark` |
| `OVERVIEW_RENDER` | `false` | Read-only documentation-site mode |
| `OVERVIEW_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `OVERVIEW_LOG_FORMAT` | `json` | `json` or `text` |
| `OVERVIEW_LOG_FILE` | — | Log file path (stdout only when unset) |
| `OVERVIEW_LOG_MAX_MB` | `10` | Max log file size before rotation |
| `OVERVIEW_LOG_BACKUPS` | `3` | Rotated files to keep (`0` truncates) |
| `OVERVIEW_MCP_TOKEN` | — | Static bearer token for MCP (protocol `2026-07-28`); prefer UI-managed API tokens |
| `OVERVIEW_AI_BASE_URL` | — | OpenAI-compatible base URL (enables AI) |
| `OVERVIEW_AI_API_KEY` | — | AI provider key |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | AI model |
| `OVERVIEW_S3_BUCKET` | — | Enable S3-compatible asset storage |
| `OVERVIEW_S3_ENDPOINT` | — | S3 endpoint (e.g. `s3.amazonaws.com`) |
| `OVERVIEW_S3_REGION` | `us-east-1` | S3 region |
| `OVERVIEW_S3_ACCESS_KEY` / `_SECRET_KEY` | — | S3 credentials |

`overview build` (alias `overview export`) also reads `OVERVIEW_EXPORT_DIR` (default `_site`)
and `OVERVIEW_EXPORT_BASE` (URL prefix, default `/`). Pass `--all` to include non-public notes.

## Static site from the CLI

Render the vault to a deployable static site without running the server:

```bash
overview -C ./data build ./public --base /docs/   # public notes
overview -C ./data build ./public --all           # include private notes
overview -C ./data build ./public --theme dark     # force light|dark|auto
```

The exported site follows the visitor's system preference by default (`--theme auto`);
pass `--theme light` or `--theme dark` (or set `OVERVIEW_SITE_THEME`) to pin it.

## Logging

Logs are structured (`slog`). stdout always receives logs; set `OVERVIEW_LOG_FILE` to
also write to a file, which rotates by size (`OVERVIEW_LOG_MAX_MB`, `OVERVIEW_LOG_BACKUPS`).
Pick `OVERVIEW_LOG_FORMAT=json` (default) or `text`, and `OVERVIEW_LOG_LEVEL`.

```bash
OVERVIEW_LOG_FILE=/var/log/overview/overview.log \
OVERVIEW_LOG_FORMAT=text \
OVERVIEW_LOG_LEVEL=info \
./overview
```

## Documentation-site mode

Run Overview as a public, read-only documentation site — no login, only public notes:

```bash
docker run -d -p 8080:5230 \
  -e OVERVIEW_RENDER=true \
  -e OVERVIEW_SITE_TITLE="Overview Docs" \
  -v "$(pwd)/site/notes:/data/notes" \
  ghcr.io/overview-note/overview:latest
```

In this mode only a read-only API surface is exposed (public notes, the tree, health and
API docs); private endpoints return 404 and writes are rejected. This is exactly how this
documentation site is built.

## Backups

Your content is just files. Back up the whole `data/` directory:

```
data/
├── notes/        # Markdown (source of truth)
├── assets/       # uploaded attachments
├── .history/     # revision snapshots
├── .trash/       # soft-deleted notes
└── overview.db   # rebuildable index
```
