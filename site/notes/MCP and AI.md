---
id: 01JMCPAI000000000000000000
title: MCP and AI
public: true
---

# MCP and AI

## AI assistant

Configure any OpenAI-compatible endpoint — OpenAI, DeepSeek, Ollama, vLLM, and more.

```bash
OVERVIEW_AI_BASE_URL=https://api.openai.com/v1
OVERVIEW_AI_API_KEY=sk-...
OVERVIEW_AI_MODEL=gpt-4o-mini
```

Or set it at runtime in **Settings → AI** (admin only); changes apply immediately.

The editor then shows an AI panel with:

- **Chat** — ask questions, generate text
- **Organize** — restructure a note (replaces the body)
- **Complete** — continue a note naturally (appends)

## AI agent (tool calling)

The assistant can do more than chat: with OpenAI-compatible **function/tool calling** it
executes real actions on your vault — list, search, read, write, move, rename and delete
notes, create folders, restore revisions/trash and manage attachments. Switch the editor's
right panel from **Assistant** to **Agent**, describe what you want, and the agent runs a
bounded loop (8 steps by default) until it is done, pauses for confirmation, or hits the
step ceiling.

Safety model:

- **Single capability source** — the tool surface lives in `internal/tools` and is shared
  with the MCP server, so both stay in lock-step.
- **Role-trimmed tools** — members may run read/write tools; only administrators (or the
  no-auth owner) may run dangerous ones. An optional `allowedTools` whitelist narrows this
  further.
- **Dangerous actions require confirmation** — deleting, purging and other destructive calls
  pause with a human-readable preview and an approve/reject bar. You can even edit the
  arguments before approving. The confirmation policy is `dangerous` (default), `all` or
  `none`.
- **Prompt-injection defense** — note bodies, titles, search results and tool outputs are
  treated as untrusted data, never as instructions.
- **Audit** — every tool call is recorded in `ai_tool_audit` with redacted arguments (secrets
  become `[redacted]`, note bodies/uploads become a SHA-256 hash plus length), plus the note
  version / trash / revision ids needed to undo destructive calls.
- **Rate limiting** — agent calls are throttled per user (20 requests / 5 minutes).
- **Graceful degradation** — if the provider refuses tool calling, the agent is disabled with
  `tool_calling_unsupported` instead of failing repeatedly.

Agent endpoints: `POST /ai/agent`, `POST /ai/agent/confirm`, `POST /ai/agent/stop`
(session-based; `GET /ai/status` reports `toolCalling`, `agentEnabled`, `maxSteps` and
`confirmPolicy`). Admins tune the agent under **Settings → AI assistant**.

## MCP server

Overview speaks the Model Context Protocol so AI agents can operate your notes.

```
POST http://<host>:5230/mcp
Authorization: Bearer <OVERVIEW_MCP_TOKEN>
```

The server implements the current spec (`2026-07-28`) and stays compatible with
older clients (a **dual-era** server):

- **Modern (2026-07-28)** — stateless requests that carry their protocol version
  in per-request `_meta` (`io.modelcontextprotocol/protocolVersion`).
  `server/discover` reports the supported versions, capabilities and identity.
  Every result includes `resultType: "complete"`.
- **Legacy (2025-11-25 and earlier)** — the `initialize` handshake is still
  answered for existing clients.

If a request declares a version the server does not support it replies with
`UnsupportedProtocolVersionError` (`-32022`) listing the supported versions.

Methods: `server/discover`, `initialize`, `tools/list`, `tools/call`, `ping`
(legacy). The tool list comes from the shared capability source (`internal/tools`),
whose schemas are generated from the OpenAPI document, so MCP and the AI agent
always expose the same surface.

### Tokens

Create and revoke **API tokens** in the web UI (**Settings → API tokens**,
admin only). The secret is shown once; the same bearer token also authenticates the
REST API. Alternatively set `OVERVIEW_MCP_TOKEN` on the server for a single
static token, or use a login session token.

Tools:

| Tool | Description |
| --- | --- |
| `notes_list` | List the note tree |
| `notes_search` | Full-text search |
| `notes_read` | Read a note by path |
| `notes_write` | Create or update a note |
| `notes_delete` | Delete a note or folder (to trash) |
| `notes_move` | Move/rename a note or folder |
| `notes_rename` | Rename within the same folder |
| `notes_links` | Outgoing links and backlinks |
| `notes_resolve` | Resolve a `[[wiki]]` target to a path |
| `folder_create` | Create a folder |
| `notes_history` | List a note's revisions |
| `notes_revision` | Read a revision's body |
| `notes_restore` | Restore a revision |
| `trash_list` | List soft-deleted items |
| `trash_restore` | Restore a trashed item |
| `trash_purge` | Permanently delete a trashed item (or all) |
| `assets_upload` | Upload an attachment |
| `assets_orphans` | List unreferenced attachments |
| `assets_purge` | Delete unreferenced attachments |
| `notes_reindex` | Rebuild the search index |
| `public_notes` | List public notes |
| `public_note` | Read a public note |

The surface intentionally mirrors the CLI and REST API, so an agent can perform
the same operations you can.

## OpenAPI

- Interactive docs: `/api/docs`
- Spec: `/api/v1/openapi.json`
