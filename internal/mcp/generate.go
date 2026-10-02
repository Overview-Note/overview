package mcp

import (
	"github.com/Overview-Note/overview/internal/openapi"
)

// toolBinding maps a REST operation in the OpenAPI document to the MCP tool
// exposed for it. The tool input schema is generated from the spec so it stays
// in sync with the published API; Schema overrides that when the operation does
// not describe the tool's arguments directly (e.g. rename, multipart upload).
// Exec is the allow-list key checked before dispatch.
type toolBinding struct {
	Method string
	Path   string
	Tool   string
	Exec   string
	Desc   string
	Schema map[string]any
}

// toolBindings is the curated surface of the API exposed over MCP. It mirrors
// the CLI/REST capabilities as closely as practical.
var toolBindings = []toolBinding{
	// Notes
	{Method: "get", Path: "/tree", Tool: "notes_list", Exec: "list",
		Desc: "List the note directory tree. Returns folders and notes with their paths and titles."},
	{Method: "get", Path: "/search", Tool: "notes_search", Exec: "search",
		Desc: "Full-text search across notes. Returns matching notes with highlighted snippets."},
	{Method: "get", Path: "/note", Tool: "notes_read", Exec: "read",
		Desc: "Read a single note by its vault-relative path (e.g. '技术/Go/并发模型.md')."},
	{Method: "put", Path: "/note", Tool: "notes_write", Exec: "write",
		Desc: "Create or overwrite a note with Markdown content. Use baseVersion '*' to create a new note."},
	{Method: "delete", Path: "/note", Tool: "notes_delete", Exec: "delete",
		Desc: "Delete a note or folder by path (moved to trash)."},
	{Method: "get", Path: "/links", Tool: "notes_links", Exec: "links",
		Desc: "Get the outgoing wiki-links and backlinks of a note."},
	{Method: "get", Path: "/resolve", Tool: "notes_resolve", Exec: "resolve",
		Desc: "Resolve a [[wiki]] target (path, title or file name) to a note path."},
	{Method: "post", Path: "/rename", Tool: "notes_move", Exec: "move",
		Desc: "Move or rename a note or folder from one vault path to another."},
	{Method: "post", Path: "/rename", Tool: "notes_rename", Exec: "rename",
		Desc: "Rename a note or folder within its current parent folder.",
		Schema: obj(map[string]any{
			"path": str("Current vault-relative path"),
			"name": str("New name (with or without the .md extension)"),
		}, "path", "name")},
	{Method: "post", Path: "/folder", Tool: "folder_create", Exec: "mkdir",
		Desc: "Create a folder at the given vault-relative path."},

	// History
	{Method: "get", Path: "/history", Tool: "notes_history", Exec: "history",
		Desc: "List the saved revisions of a note."},
	{Method: "get", Path: "/history/revision", Tool: "notes_revision", Exec: "revision",
		Desc: "Return the raw Markdown body of a specific note revision."},
	{Method: "post", Path: "/history/restore", Tool: "notes_restore", Exec: "restore",
		Desc: "Restore a note from a saved revision."},

	// Trash
	{Method: "get", Path: "/trash", Tool: "trash_list", Exec: "trash-list",
		Desc: "List soft-deleted notes that can still be restored."},
	{Method: "post", Path: "/trash/restore", Tool: "trash_restore", Exec: "trash-restore",
		Desc: "Restore a trashed item by its id."},
	{Method: "delete", Path: "/trash", Tool: "trash_purge", Exec: "trash-purge",
		Desc: "Permanently delete a trashed item by id, or every trashed item when id is omitted.",
		Schema: obj(map[string]any{
			"id": str("Trash entry id; omit to purge everything"),
		})},

	// Assets
	{Method: "post", Path: "/assets", Tool: "assets_upload", Exec: "asset-upload",
		Desc: "Upload an attachment. Provide its name and text content, or base64-encoded bytes.",
		Schema: obj(map[string]any{
			"name":    str("Stored file name, e.g. diagram.png"),
			"content": str("File content (text, or base64 when base64 is true)"),
			"base64":  map[string]any{"type": "boolean", "description": "Set true when content is base64-encoded"},
		}, "name", "content")},
	{Method: "get", Path: "/assets/orphans", Tool: "assets_orphans", Exec: "assets-orphans",
		Desc: "List stored attachments that no note references."},
	{Method: "post", Path: "/assets/orphans/purge", Tool: "assets_purge", Exec: "assets-purge",
		Desc: "Delete every unreferenced attachment."},

	// Index / public
	{Method: "post", Path: "/reindex", Tool: "notes_reindex", Exec: "reindex",
		Desc: "Rebuild the full-text search index from the files on disk."},
	{Method: "get", Path: "/public/notes", Tool: "public_notes", Exec: "public-notes",
		Desc: "List notes marked public."},
	{Method: "get", Path: "/public/note", Tool: "public_note", Exec: "public-note",
		Desc: "Read a public note by path."},
}

// generatedToolDefs builds the MCP tool list from the OpenAPI document, using
// each binding's explicit schema where provided and falling back to curated
// schemas if the spec cannot be parsed.
func generatedToolDefs() []toolDef {
	spec, err := openapi.Document()
	if err != nil {
		return fallbackToolDefs()
	}
	defs := make([]toolDef, 0, len(toolBindings))
	for _, b := range toolBindings {
		defs = append(defs, toolDef{
			Name:        b.Tool,
			Description: b.Desc,
			InputSchema: bindingSchema(spec, b),
		})
	}
	if len(defs) == 0 {
		return fallbackToolDefs()
	}
	return defs
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

// execFor returns the executor key for a tool name, or "" if unknown.
func execFor(tool string) string {
	for _, b := range toolBindings {
		if b.Tool == tool {
			return b.Exec
		}
	}
	return ""
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

// fallbackToolDefs is used when the OpenAPI document is unavailable. It mirrors
// the core note tools with hand-written schemas.
func fallbackToolDefs() []toolDef {
	defs := make([]toolDef, 0, len(toolBindings))
	for _, b := range toolBindings {
		schema := b.Schema
		if schema == nil {
			schema = obj(map[string]any{})
		}
		defs = append(defs, toolDef{Name: b.Tool, Description: b.Desc, InputSchema: schema})
	}
	return defs
}
