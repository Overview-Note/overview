package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
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
	return generatedToolDefs()
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
	// Only tools published in the OpenAPI-derived surface may be invoked.
	if execFor(p.Name) == "" {
		return nil, &rpcError{Code: -32602, Message: "unknown tool: " + p.Name}
	}
	result, err := s.runTool(ctx, p.Name, p.Arguments)
	if err != nil {
		// MCP reports tool execution failures in-band.
		return map[string]any{
			"resultType": "complete",
			"content":    []map[string]any{{"type": "text", "text": err.Error()}},
			"isError":    true,
		}, nil
	}
	return map[string]any{
		"resultType": "complete",
		"content":    []map[string]any{{"type": "text", "text": result}},
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

	case "notes_resolve":
		var a struct {
			Target string `json:"target"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Target) == "" {
			return "", fmt.Errorf("target is required")
		}
		meta, err := s.svc.ResolveLink(ctx, a.Target)
		if err != nil {
			return "", err
		}
		return toJSON(meta), nil

	case "notes_move":
		var a struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		_ = json.Unmarshal(args, &a)
		if a.From == "" || a.To == "" {
			return "", fmt.Errorf("from and to are required")
		}
		if err := s.svc.Move(ctx, a.From, a.To); err != nil {
			return "", err
		}
		return toJSON(map[string]string{"from": a.From, "to": a.To}), nil

	case "notes_rename":
		var a struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Path) == "" || strings.TrimSpace(a.Name) == "" {
			return "", fmt.Errorf("path and name are required")
		}
		dir := ""
		if i := strings.LastIndex(a.Path, "/"); i >= 0 {
			dir = a.Path[:i+1]
		}
		to := dir + a.Name
		if err := s.svc.Move(ctx, a.Path, to); err != nil {
			return "", err
		}
		return toJSON(map[string]string{"from": a.Path, "to": to}), nil

	case "folder_create":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Path) == "" {
			return "", fmt.Errorf("path is required")
		}
		if err := s.svc.Mkdir(ctx, a.Path); err != nil {
			return "", err
		}
		return toJSON(map[string]string{"path": a.Path}), nil

	case "notes_history":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		revs, err := s.svc.Revisions(ctx, a.Path)
		if err != nil {
			return "", err
		}
		return toJSON(revs), nil

	case "notes_revision":
		var a struct {
			Path string `json:"path"`
			ID   string `json:"id"`
		}
		_ = json.Unmarshal(args, &a)
		body, err := s.svc.RevisionContent(ctx, a.Path, a.ID)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]string{"path": a.Path, "id": a.ID, "content": body}), nil

	case "notes_restore":
		var a struct {
			Path string `json:"path"`
			ID   string `json:"id"`
		}
		_ = json.Unmarshal(args, &a)
		note, err := s.svc.RestoreRevision(ctx, a.Path, a.ID)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"path": note.Path, "version": note.Version}), nil

	case "trash_list":
		entries, err := s.svc.Trash(ctx)
		if err != nil {
			return "", err
		}
		return toJSON(entries), nil

	case "trash_restore":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.ID) == "" {
			return "", fmt.Errorf("id is required")
		}
		if err := s.svc.RestoreTrash(ctx, a.ID); err != nil {
			return "", err
		}
		return `{"restored":true}`, nil

	case "trash_purge":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.ID) != "" {
			if err := s.svc.PurgeTrash(ctx, a.ID); err != nil {
				return "", err
			}
			return `{"purged":1}`, nil
		}
		entries, err := s.svc.Trash(ctx)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if err := s.svc.PurgeTrash(ctx, e.ID); err != nil {
				return "", err
			}
		}
		return toJSON(map[string]int{"purged": len(entries)}), nil

	case "assets_upload":
		var a struct {
			Name    string `json:"name"`
			Content string `json:"content"`
			Base64  bool   `json:"base64"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Name) == "" {
			return "", fmt.Errorf("name is required")
		}
		data := []byte(a.Content)
		if a.Base64 {
			decoded, err := base64.StdEncoding.DecodeString(a.Content)
			if err != nil {
				return "", fmt.Errorf("invalid base64 content: %w", err)
			}
			data = decoded
		}
		asset, err := s.svc.Upload(ctx, a.Name, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		return toJSON(asset), nil

	case "assets_orphans":
		orphans, err := s.svc.OrphanAssets(ctx)
		if err != nil {
			return "", err
		}
		return toJSON(orphans), nil

	case "assets_purge":
		n, err := s.svc.PurgeOrphanAssets(ctx)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]int{"removed": n}), nil

	case "notes_reindex":
		if err := s.svc.Reindex(ctx); err != nil {
			return "", err
		}
		return `{"reindexed":true}`, nil

	case "public_notes":
		notes, err := s.svc.PublicNotes(ctx)
		if err != nil {
			return "", err
		}
		return toJSON(notes), nil

	case "public_note":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		note, err := s.svc.PublicNote(ctx, a.Path)
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{
			"path":  note.Path,
			"title": note.Title,
			"body":  note.Body,
		}), nil

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
