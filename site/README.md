# Overview documentation site

The project's documentation site is built with **Overview itself**, running in
read-only *render mode*.

## Structure

```
site/
├── notes/            # the documentation, as an Overview Markdown vault
│   ├── Home.md
│   ├── Getting Started.md
│   ├── Features.md
│   ├── Deployment.md
│   ├── Architecture.md
│   ├── MCP and AI.md
│   └── FAQ.md
└── docker-compose.yml
```

Every note has `public: true` in its frontmatter so it is visible to anonymous readers.

## Run locally

With the binary:

```bash
OVERVIEW_RENDER=true \
OVERVIEW_SITE_TITLE="Overview Docs" \
OVERVIEW_DATA_DIR=./site \
go run ./cmd/overview
# open http://localhost:5230
```

With Docker:

```bash
docker compose -f site/docker-compose.yml up -d --build
# open http://localhost:8080
```

In render mode the server:

- disables authentication,
- exposes only notes marked `public: true`,
- rejects all state-changing API requests (read-only),
- serves the site at `/` with notes at `/note/<path>`.

## Editing the docs

Just edit the Markdown files under `site/notes/`. To add a page, create a new `.md` file
with `public: true` frontmatter; it appears in the index automatically.
