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
(legacy). The tool list is generated from the OpenAPI document so it stays in
sync with the REST API.

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
