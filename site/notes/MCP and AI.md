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

Methods: `initialize`, `tools/list`, `tools/call`.

Tools:

| Tool | Description |
| --- | --- |
| `notes_list` | List the note tree |
| `notes_search` | Full-text search |
| `notes_read` | Read a note by path |
| `notes_write` | Create or update a note |
| `notes_delete` | Delete a note or folder |
| `notes_links` | Outgoing links and backlinks |

## OpenAPI

- Interactive docs: `/api/docs`
- Spec: `/api/v1/openapi.json`
