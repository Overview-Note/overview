package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func obj(props map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func toolDefs() []toolDef {
	return []toolDef{
		{
			Name:        "notes_list",
			Description: "List the note directory tree. Returns folders and notes with their paths and titles.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "notes_search",
			Description: "Full-text search across notes. Returns matching notes with highlighted snippets.",
			InputSchema: obj(map[string]any{
				"query": str("Search query (supports CJK substring matching)"),
				"limit": map[string]any{"type": "integer", "description": "Max results (default 20)"},
			}, "query"),
		},
		{
			Name:        "notes_read",
			Description: "Read a single note by its vault-relative path (e.g. '技术/Go/并发模型.md').",
			InputSchema: obj(map[string]any{
				"path": str("Vault-relative note path"),
			}, "path"),
		},
		{
			Name:        "notes_write",
			Description: "Create or overwrite a note with Markdown content. Use baseVersion '*' to create a new note, or omit/empty to overwrite an existing one.",
			InputSchema: obj(map[string]any{
				"path":        str("Vault-relative note path (may omit the .md extension)"),
				"body":        str("Markdown body"),
				"baseVersion": str("Optimistic concurrency token; '*' creates only if absent"),
				"public":      map[string]any{"type": "boolean", "description": "Expose the note for anonymous read-only access"},
			}, "path", "body"),
		},
		{
			Name:        "notes_delete",
			Description: "Delete a note or folder by path.",
			InputSchema: obj(map[string]any{
				"path": str("Vault-relative path"),
			}, "path"),
		},
		{
			Name:        "notes_links",
			Description: "Get the outgoing wiki-links and backlinks of a note.",
			InputSchema: obj(map[string]any{
				"path": str("Vault-relative note path"),
			}, "path"),
		},
	}
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p callParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params"}
	}
	result, err := s.runTool(ctx, p.Name, p.Arguments)
	if err != nil {
		// MCP reports tool execution failures in-band.
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": result}},
	}, nil
}

func (s *Server) runTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "notes_list":
		nodes, err := s.svc.Tree(ctx)
		if err != nil {
			return "", err
		}
		return toJSON(nodes), nil

	case "notes_search":
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		hits, err := s.svc.Search(ctx, a.Query, a.Limit, 0)
		if err != nil {
			return "", err
		}
		return toJSON(hits), nil

	case "notes_read":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		note, err := s.svc.GetNote(ctx, a.Path)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{
			"path":    note.Path,
			"title":   note.Title,
			"version": note.Version,
			"public":  note.Public,
			"body":    note.Body,
		}), nil

	case "notes_write":
		var a struct {
			Path        string `json:"path"`
			Body        string `json:"body"`
			BaseVersion string `json:"baseVersion"`
			Public      bool   `json:"public"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Path) == "" {
			return "", fmt.Errorf("path is required")
		}
		note, err := s.svc.SaveNote(ctx, a.Path, a.Body, a.BaseVersion, a.Public)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{
			"path":    note.Path,
			"title":   note.Title,
			"version": note.Version,
		}), nil

	case "notes_delete":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		if err := s.svc.Delete(ctx, a.Path); err != nil {
			return "", err
		}
		return `{"deleted":true}`, nil

	case "notes_links":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		links, err := s.svc.Links(ctx, a.Path)
		if err != nil {
			return "", err
		}
		return toJSON(links), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
