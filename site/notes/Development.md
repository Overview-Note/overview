---
id: 01JDEV000000000000000000000
title: Development
public: true
---

# Development

## Layout

- `cmd/overview` — server entry point (`overview`, `overview export`)
- `internal/` — layered Go packages (see [[Architecture]])
- `web/` — Vue 3 + Vite frontend, built into `internal/webui/dist` and embedded
- `site/notes/` — the notes that generate this documentation site
- `demo/` — a sample vault used by `make demo`

## Common tasks

```bash
make demo     # sample vault on :5230
make dev      # backend with the embedded frontend
make build    # frontend + single binary
make docker   # Docker image

make test     # all tests (Go + frontend)
make test-go  # go test ./...
make test-web # vue-tsc + Vitest
make lint     # golangci-lint + vue-tsc
make fmt      # gofmt + prettier
```

## Tests

- **Go:** `go test ./...` — per-package unit tests plus `httptest` integration tests.
- **Frontend:** `npm test` (Vitest) for pure logic; `npm run lint` for type-checking.

## Logging

Overview writes structured JSON (or text) logs to stdout. Set `OVERVIEW_LOG_FILE` to also
write to a size-rotated file:

```bash
OVERVIEW_LOG_FILE=./data/logs/overview.log OVERVIEW_LOG_FORMAT=text ./bin/overview
```

See [[Deployment]] for the full environment-variable list.

## Contributing

See [`CONTRIBUTING.md`](https://github.com/Overview-Note/overview/blob/main/CONTRIBUTING.md).
