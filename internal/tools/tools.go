// Package tools is the single source of truth for the application's agent/MCP
// capability surface. Each tool maps a REST operation in the OpenAPI document
// to an executable in-process action, and carries a risk classification used to
// decide whether a role may run it and whether the user must confirm first.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/openapi"
	"github.com/Overview-Note/overview/internal/service"
)

// ToolDef is a single tool definition in MCP shape.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// toolBinding maps a REST operation in the OpenAPI document to the tool exposed
// for it. The tool input schema is generated from the spec so it stays in sync
// with the published API; Schema overrides that when the operation does not
// describe the tool's arguments directly (e.g. rename, multipart upload).
type toolBinding struct {
	Method string
	Path   string
	Tool   string
	Desc   string
	Schema map[string]any
}

// toolBindings is the curated surface of the API exposed to agents and MCP. It
// mirrors the CLI/REST capabilities as closely as practical.
var toolBindings = []toolBinding{
	// Notes
	{Method: "get", Path: "/tree", Tool: "notes_list",
		Desc:   "List the note directory tree. Returns folders and notes with their paths and titles.",
		Schema: emptyObj()},
	{Method: "get", Path: "/search", Tool: "notes_search",
		Desc: "Full-text search across notes. Returns matching notes with highlighted snippets."},
	{Method: "get", Path: "/note", Tool: "notes_read",
		Desc: "Read a single note by its vault-relative path (e.g. '技术/Go/并发模型.md')."},
	{Method: "put", Path: "/note", Tool: "notes_write",
		Desc: "Create or overwrite a note with Markdown content. Use baseVersion '*' to create a new note."},
	{Method: "delete", Path: "/note", Tool: "notes_delete",
		Desc: "Delete a note or folder by path (moved to trash)."},
	{Method: "get", Path: "/links", Tool: "notes_links",
		Desc: "Get the outgoing wiki-links and backlinks of a note."},
	{Method: "get", Path: "/resolve", Tool: "notes_resolve",
		Desc: "Resolve a [[wiki]] target (path, title or file name) to a note path."},
	{Method: "post", Path: "/rename", Tool: "notes_move",
		Desc: "Move or rename a note or folder from one vault path to another."},
	{Method: "post", Path: "/rename", Tool: "notes_rename",
		Desc: "Rename a note or folder within its current parent folder.",
		Schema: obj(map[string]any{
			"path": str("Current vault-relative path"),
			"name": str("New name (with or without the .md extension)"),
		}, "path", "name")},
	{Method: "post", Path: "/folder", Tool: "folder_create",
		Desc: "Create a folder at the given vault-relative path."},

	// History
	{Method: "get", Path: "/history", Tool: "notes_history",
		Desc: "List the saved revisions of a note."},
	{Method: "get", Path: "/history/revision", Tool: "notes_revision",
		Desc: "Return the raw Markdown body of a specific note revision."},
	{Method: "post", Path: "/history/restore", Tool: "notes_restore",
		Desc: "Restore a note from a saved revision."},

	// Trash
	{Method: "get", Path: "/trash", Tool: "trash_list",
		Desc: "List soft-deleted notes that can still be restored."},
	{Method: "post", Path: "/trash/restore", Tool: "trash_restore",
		Desc: "Restore a trashed item by its id."},
	{Method: "delete", Path: "/trash", Tool: "trash_purge",
		Desc: "Permanently delete a trashed item by id, or every trashed item when id is omitted.",
		Schema: obj(map[string]any{
			"id": str("Trash entry id; omit to purge everything"),
		})},

	// Assets
	{Method: "post", Path: "/assets", Tool: "assets_upload",
		Desc: "Upload an attachment. Provide its name and text content, or base64-encoded bytes.",
		Schema: obj(map[string]any{
			"name":    str("Stored file name, e.g. diagram.png"),
			"content": str("File content (text, or base64 when base64 is true)"),
			"base64":  map[string]any{"type": "boolean", "description": "Set true when content is base64-encoded"},
		}, "name", "content")},
	{Method: "get", Path: "/assets/orphans", Tool: "assets_orphans",
		Desc:   "List stored attachments that no note references.",
		Schema: emptyObj()},
	{Method: "post", Path: "/assets/orphans/purge", Tool: "assets_purge",
		Desc: "Delete every unreferenced attachment."},

	// Index / public
	{Method: "post", Path: "/reindex", Tool: "notes_reindex",
		Desc:   "Rebuild the full-text search index from the files on disk.",
		Schema: emptyObj()},
	{Method: "get", Path: "/public/notes", Tool: "public_notes",
		Desc:   "List notes marked public.",
		Schema: emptyObj()},
	{Method: "get", Path: "/public/note", Tool: "public_note",
		Desc: "Read a public note by path."},
}

// Risk levels.
const (
	RiskRead      = "read"
	RiskWrite     = "write"
	RiskDangerous = "dangerous"
)

// RoleOwner is the synthetic role used when authentication is disabled. It is
// treated as administrator-equivalent.
const RoleOwner = "owner"

// riskByTool classifies every tool. Read tools need no confirmation, write
// tools may require confirmation depending on policy, and dangerous tools
// always require an explicit confirmation. Tools absent from this map are
// treated as dangerous.
var riskByTool = map[string]string{
	// read
	"notes_list":      RiskRead,
	"notes_search":    RiskRead,
	"notes_read":      RiskRead,
	"notes_links":     RiskRead,
	"notes_resolve":   RiskRead,
	"notes_history":   RiskRead,
	"notes_revision":  RiskRead,
	"trash_list":      RiskRead,
	"assets_orphans":  RiskRead,
	"public_notes":    RiskRead,
	"public_note":     RiskRead,
	"capture_preview": RiskRead,
	// write
	"notes_write":   RiskWrite,
	"folder_create": RiskWrite,
	"notes_move":    RiskWrite,
	"notes_rename":  RiskWrite,
	"notes_restore": RiskWrite,
	"trash_restore": RiskWrite,
	"notes_reindex": RiskWrite,
	"assets_upload": RiskWrite,
	// dangerous
	"notes_delete": RiskDangerous,
	"trash_purge":  RiskDangerous,
	"assets_purge": RiskDangerous,
}

// Defs builds the tool list from the OpenAPI document, using each binding's
// explicit schema where provided and falling back to curated schemas if the
// spec cannot be parsed.
func Defs() []ToolDef {
	spec, err := openapi.Document()
	if err != nil {
		return fallbackDefs()
	}
	defs := make([]ToolDef, 0, len(toolBindings))
	for _, b := range toolBindings {
		defs = append(defs, ToolDef{
			Name:        b.Tool,
			Description: b.Desc,
			InputSchema: bindingSchema(spec, b),
		})
	}
	if len(defs) == 0 {
		return fallbackDefs()
	}
	return defs
}

// DefsOpenAI converts the (optionally filtered) tool list to the OpenAI
// chat-completions tools[].function shape.
func DefsOpenAI(allow func(name string) bool) []any {
	defs := Defs()
	out := make([]any, 0, len(defs))
	for _, d := range defs {
		if allow != nil && !allow(d.Name) {
			continue
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        d.Name,
				"description": d.Description,
				"parameters":  d.InputSchema,
			},
		})
	}
	return out
}

// Known reports whether name is part of the published tool surface.
func Known(name string) bool {
	for _, b := range toolBindings {
		if b.Tool == name {
			return true
		}
	}
	return false
}

// Risk returns the risk level of a tool. Unknown tools are dangerous.
func Risk(name string) string {
	if r, ok := riskByTool[name]; ok {
		return r
	}
	return RiskDangerous
}

// Allows reports whether role may run name. Members may run read and write
// tools; only administrators (or the no-auth owner) may run dangerous tools.
func Allows(role, name string) bool {
	privileged := role == core.RoleAdmin || role == RoleOwner
	switch Risk(name) {
	case RiskRead, RiskWrite:
		return privileged || role == core.RoleMember
	default:
		return privileged
	}
}

func bindingSchema(spec openapi.Spec, b toolBinding) map[string]any {
	if b.Schema != nil {
		return b.Schema
	}
	if op, ok := spec.Paths[b.Path][b.Method]; ok {
		return schemaFor(op)
	}
	return obj(map[string]any{})
}

// schemaFor synthesizes a JSON-schema object from an OpenAPI operation's query
// parameters and JSON request body.
func schemaFor(op openapi.Op) map[string]any {
	props := map[string]any{}
	required := []string{}
	for _, p := range op.Parameters {
		if p.In != "query" && p.In != "path" {
			continue
		}
		schema := p.Schema
		if len(schema) == 0 {
			schema = map[string]any{"type": "string"}
		}
		props[p.Name] = schema
		if p.Required {
			required = append(required, p.Name)
		}
	}
	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok {
			if p, ok := mt.Schema["properties"].(map[string]any); ok {
				for k, v := range p {
					props[k] = v
				}
			}
			if req, ok := mt.Schema["required"].([]any); ok {
				for _, r := range req {
					if name, ok := r.(string); ok {
						required = append(required, name)
					}
				}
			}
		}
	}
	return obj(props, required...)
}

// fallbackDefs is used when the OpenAPI document is unavailable. It mirrors the
// core note tools with hand-written schemas.
func fallbackDefs() []ToolDef {
	defs := make([]ToolDef, 0, len(toolBindings))
	for _, b := range toolBindings {
		schema := b.Schema
		if schema == nil {
			schema = obj(map[string]any{})
		}
		defs = append(defs, ToolDef{Name: b.Tool, Description: b.Desc, InputSchema: schema})
	}
	return defs
}

func obj(props map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func emptyObj() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// Set is a service-backed tool executor. It holds the only dependency tools
// need to run in-process: the application service.
type Set struct {
	svc *service.Service
}

// NewSet constructs a tool set backed by svc.
func NewSet(svc *service.Service) *Set { return &Set{svc: svc} }

// Preview renders a short, human-readable description of a pending call so a
// user can confirm it. Read tools need no preview and return "".
func (s *Set) Preview(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "notes_delete":
		var a struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.Path) == "" {
			return "", core.Invalidf("path is required")
		}
		note, err := s.svc.GetNote(ctx, a.Path)
		if err != nil {
			return fmt.Sprintf("Delete %s", a.Path), nil
		}
		return fmt.Sprintf("Delete note %q (%s, %d bytes)", note.Title, note.Path, note.Size), nil

	case "notes_move":
		var a struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		_ = json.Unmarshal(args, &a)
		return fmt.Sprintf("Move %s → %s", a.From, a.To), nil

	case "notes_rename":
		var a struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(args, &a)
		dir := ""
		if i := strings.LastIndex(a.Path, "/"); i >= 0 {
			dir = a.Path[:i+1]
		}
		return fmt.Sprintf("Rename %s → %s", a.Path, dir+a.Name), nil

	case "trash_purge":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(args, &a)
		if strings.TrimSpace(a.ID) != "" {
			return fmt.Sprintf("Permanently delete 1 trashed item (%s)", a.ID), nil
		}
		entries, err := s.svc.Trash(ctx)
		if err != nil {
			return "Permanently delete all trashed items", nil
		}
		return fmt.Sprintf("Permanently delete %d trashed items", len(entries)), nil

	case "assets_purge":
		orphans, err := s.svc.OrphanAssets(ctx)
		if err != nil {
			return "Permanently delete all orphaned attachments", nil
		}
		return fmt.Sprintf("Permanently delete %d orphaned attachments", len(orphans)), nil
	}

	if Risk(name) == RiskRead {
		return "", nil
	}
	return fmt.Sprintf("%s %s", name, truncate(string(args), 300)), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
