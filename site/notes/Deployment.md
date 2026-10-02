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
| `OVERVIEW_AUTH` | `multi` | `multi` or `none` |
| `OVERVIEW_RENDER` | `false` | Read-only documentation-site mode |
| `OVERVIEW_SITE_TITLE` | `Overview` | Site title (render mode) |
| `OVERVIEW_MCP_TOKEN` | — | Bearer token for MCP |
| `OVERVIEW_AI_BASE_URL` | — | OpenAI-compatible base URL (enables AI) |
| `OVERVIEW_AI_API_KEY` | — | AI provider key |
| `OVERVIEW_AI_MODEL` | `gpt-4o-mini` | AI model |
| `OVERVIEW_S3_BUCKET` | — | Enable S3-compatible asset storage |

## Documentation-site mode

Run Overview as a public, read-only documentation site — no login, only public notes:

```bash
docker run -d -p 8080:5230 \
  -e OVERVIEW_RENDER=true \
  -e OVERVIEW_SITE_TITLE="Overview Docs" \
  -v "$(pwd)/site/notes:/data/notes" \
  overview:latest
```

This is exactly how this documentation site is built.

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
