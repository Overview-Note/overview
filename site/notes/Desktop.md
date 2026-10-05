---
id: 01JDESK0000000000000000000
title: Desktop
public: true
---

# Desktop app

Overview also ships as a **native desktop application**. The same Go binary that powers the
server opens a native window instead of (or in addition to) serving the browser UI.

The shell is built with [Wails v3](https://v3.wails.io) (`v3.0.0-beta.26`): a small Go
process hosts a WebView pointing at the local Overview server. All application behaviour —
files, index, search, AI, everything — lives in the shared packages, so the desktop app and
the headless server are the same program with a different front end.

## What the shell adds

- **Native window** — an embedded WebView (WebView2 on Windows, WKWebView on macOS,
  WebKitGTK on Linux) pointed at `http://127.0.0.1:<port>`
- **System tray** — open the window or quit from the tray
- **Single instance** — launching again raises the existing window; a file or deep link
  passed to the second launch is forwarded to the first
- **File association** — opening a `.md` file imports/navigates to it
- **Deep links** — `overview://note/<path>` and `overview://open?path=<path>`
- **Drag and drop** — drop Markdown files onto the window
- **Launch at login** — available as a toggle under **Settings → Desktop**
- **Update notification** — checks GitHub Releases and links to the download page; it never
  downloads or installs silently
- **First-run folder picker** — choose which vault to open the first time

## Vault sync (desktop ↔ server)

The desktop app can keep its local vault in step with a self-hosted Overview server, using
the same client ↔ server model as Trilium. Open **Settings → Sync** (desktop only) and enter
the server URL and an API token; the status card, conflict list and *Sync now* button live
there too.

- **Identity by note `id`** — frontmatter `id` (ULID) is the stable cross-device identity;
  paths are just locations, so moves and renames don't create duplicates. Notes without an
  `id` get one on the first sync.
- **Manifest + strong ETag** — the server exposes `GET /api/v1/sync/manifest`
  (`vaultId`, every note with its content version, and every folder including empty ones)
  behind a strong `ETag`, and answers `304` when nothing changed.
- **Three-way reconciliation** — the engine compares the remote manifest with the local vault
  and its last-synced state, then pulls, pushes, deletes and moves notes and attachments.
  Writes go through a **faithful raw channel** (`PUT /api/v1/note` with `raw: true`) that
  stores the exact Markdown bytes and carries a `baseVersion`, so a concurrent edit is never
  silently overwritten.
- **Direction** — `both` (default), `pull` (server → desktop) or `push` (desktop → server).
- **Conflicts are kept, not merged** — if both sides edit the same note, the local copy is
  saved as a sibling `<name> (conflict-<device>-<ts>).md` and the remote original stays put.
  Resolve each conflict from the settings page with *keep local* or *keep remote*.
- **Attachments** — synced by vault-relative path; pushes are de-duplicated by content hash
  and pulls only fill in missing files.
- **Polling, not streaming** — Phase 1 checks on an interval (default 60s) and is also woken
  by the local file watcher; there is no SSE/push yet.

The token is stored separately in `<DataDir>/.sync-token` (never in the exportable
`sync.json` or logs), and plain HTTP is only accepted for loopback hosts — a remote server
must use HTTPS.

## Local-first defaults

On the desktop the app is a single-user, local-only tool:

- listens on `127.0.0.1` only
- authentication is off
- MCP and WebDAV are off
- the data directory is per-user

Because authentication may be off, write requests are protected by a **Host and origin
guard** (the Host header must be a loopback name, and state-changing browser requests must be
same-origin) rather than cookie CSRF protection. The desktop-only API endpoints
(`/api/v1/desktop/*`) are not even registered by a headless server — it answers `404`.

Setting `OVERVIEW_DESKTOP=true` switches on these defaults; every one of them can be
overridden with the usual environment variables (`OVERVIEW_ADDR`, `OVERVIEW_AUTH`,
`OVERVIEW_ENABLE_MCP`, `OVERVIEW_ENABLE_DAV`, `OVERVIEW_DATA_DIR`).

## Data directory

| Platform | Default |
| --- | --- |
| Windows | `%USERPROFILE%\Documents\Overview` |
| macOS | `~/Documents/Overview` |
| Linux | `~/Documents/Overview`, else `$XDG_DATA_HOME/overview` or `~/.local/share/overview` |

If the directory is missing or empty, the first launch asks you to pick a folder (cancelling
keeps the default).

## Building

The frontend is bundled into the binary, so build the web app first:

```bash
make build-desktop      # -> bin/overview-desktop (host platform)
make package-desktop    # + tar.gz archive
```

Or build the shell directly:

| Platform | Command | Requirements |
| --- | --- | --- |
| Windows | `go build ./cmd/overview-desktop` | WebView2 runtime (no CGO) |
| macOS | `go build -tags desktop ./cmd/overview-desktop` | CGO + Xcode command line tools |
| Linux | `go build -tags desktop ./cmd/overview-desktop` | CGO + GTK4 + WebKitGTK 6.0 |
| Linux (older) | `go build -tags "desktop gtk3" ./cmd/overview-desktop` | CGO + GTK3 + WebKit2GTK 4.1 |

Windows builds are produced with the **GUI PE subsystem** (`-ldflags "-H=windowsgui"`), so
launching the `.exe` does not open a console window. Because a GUI process has no stdout,
the desktop shell logs to a file by default: `<DataDir>/logs/desktop.log` (set
`OVERVIEW_LOG_FILE` to override). The logger writes through a fault-tolerant fanout, so one
unavailable destination (such as stdout under the GUI subsystem) never stops the file log.

The shell source is guarded by the build tag `windows || darwin || desktop`, so a plain
`CGO_ENABLED=0 go build ./...` on a machine without GTK headers still works for the backend.

Tagged releases build the Windows/macOS/Linux shells in CI
(`.github/workflows/desktop.yml`) and attach them to the GitHub Release.

## Current limitations

- The macOS and Linux builds are compiled and archived in CI but have **not been verified on
  real hardware**.
- There are **no installers yet** (Windows NSIS, macOS DMG, Linux AppImage/`.desktop`); the
  release assets are plain binaries in a `.zip`/`.tar.gz`.
- The `wails3` packaging pipeline is not wired into the repo build; the shell is built with a
  plain `go build` (see `build/config.yml` for the metadata that the packager would use).
- Updates are only *notified* — there is no silent auto-update.
- If the preferred port is taken, the app falls back to a random loopback port (written to
  `.overview-port`); the port is not guaranteed to be stable.
- Sync Phase 1 polls the full manifest (default 60s, plus file-watcher triggers) rather than
  streaming; a server is a single shared vault with no per-user ACL; and S3 asset backends
  are skipped (`PUT /assets/{path}` returns `501`).

See [[Architecture]] for how the shell fits into the codebase, and
[`docs/DESIGN.md`](https://github.com/Overview-Note/overview/blob/main/docs/DESIGN.md)
(ADR-059…066) for the design decisions.
