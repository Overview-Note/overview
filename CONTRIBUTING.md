# Contributing to Overview

Thanks for your interest in improving Overview! This document explains how to set up
the project, our conventions, and how to get your changes merged.

## Ways to contribute

- 🐛 **Report bugs** — open an issue with steps to reproduce.
- 💡 **Suggest features** — open a feature request describing the problem first.
- 📝 **Improve docs** — fixes to `README.md` and `docs/` are very welcome.
- 🔧 **Send code** — bug fixes, features, tests, refactors.

New contributors: look for issues labeled [`good first issue`](https://github.com/overview-app/overview/labels/good%20first%20issue).

## Development setup

Requirements:

- **Go 1.26+**
- **Node 22+**
- **Docker** (optional, for container builds)

```bash
git clone https://github.com/overview-app/overview.git
cd overview

# terminal 1 — backend on :5230
go run ./cmd/overview

# terminal 2 — frontend hot reload on :5173 (proxies /api to :5230)
cd web && npm install && npm run dev
```

Data is written to `./data` by default.

## Project layout

```
cmd/overview        entrypoint
internal/
  core              domain models, ports (interfaces), errors
  service           use cases (notes, search, auth, ai, …)
  store             filesystem note/asset repository
  index             SQLite + FTS5 index, migrations
  textproc          CJK tokenizer, wiki-link & snippet helpers
  markdown          frontmatter parse/serialize
  history, trash    revision snapshots, soft delete
  watcher           fsnotify-based live reindex
  s3store           S3-compatible asset backend
  mcp, openapi      MCP server, OpenAPI spec
  server            HTTP adapter (routing, middleware, handlers)
  webui             embedded frontend (built output)
web/                Vue 3 + Tiptap frontend source
docs/               design document, project overview
```

**Architecture rule:** business logic lives in `service` and depends only on the
interfaces declared in `core`. Adapters (`store`, `index`, `s3store`, `server`, `mcp`)
implement or consume those interfaces. Keep HTTP handlers thin.

## Before you open a PR

Run the full check suite locally:

```bash
make fmt        # gofmt + prettier
make vet        # go vet ./...
make test       # go test ./...
make lint       # golangci-lint + vue-tsc
```

Or individually:

```bash
go test ./...
go vet ./...
cd web && npm run lint && npm run build
```

All of the above run in CI (`.github/workflows/ci.yml`) on every pull request.

## Coding guidelines

- **Go:** format with `gofmt`, keep packages focused, wrap errors with `%w`, add table-driven
  tests for new logic. Prefer explicit types and small interfaces.
- **TypeScript/Vue:** `<script setup>` with the Composition API, typed API calls via `web/src/api.ts`,
  reuse the i18n `t()` for any user-facing string (zh + en), and keep components small.
- **No comments that restate the code.** Explain *why*, not *what*.
- **Conventional commits:** `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`.

## Tests

- Backend: `go test ./...` — unit tests per package plus `httptest` integration tests.
- Frontend: `vue-tsc` type-check and `vite build`.
- When fixing a bug, add a test that fails before your fix and passes after.

## Pull request process

1. Fork and branch from `main` (`feat/short-description`).
2. Keep PRs focused; one logical change per PR.
3. Ensure `make test` and `make lint` pass.
4. Fill in the PR template; link the related issue.
5. A maintainer will review. Squash-merge is preferred.

## Reporting security issues

Please do **not** open a public issue for security problems. Email the maintainer
listed in `go.mod`/the repository profile, or use GitHub's private security advisory.

## License

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
