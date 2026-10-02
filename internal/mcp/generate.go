package mcp

import (
	"github.com/Overview-Note/overview/internal/openapi"
)

// toolBinding maps a REST operation in the OpenAPI document to the MCP tool
// exposed for it. The tool input schema is generated from the spec so it stays
// in sync with the published API; Exec selects the executor in runTool.
type toolBinding struct {
	Method string
	Path   string
	Tool   string
	Exec   string
	Desc   string
}

// toolBindings is the curated surface of the API exposed over MCP.
var toolBindings = []toolBinding{
	{Method: "get", Path: "/tree", Tool: "notes_list", Exec: "list",
		Desc: "List the note directory tree. Returns folders and notes with their paths and titles."},
	{Method: "get", Path: "/search", Tool: "notes_search", Exec: "search",
		Desc: "Full-text search across notes. Returns matching notes with highlighted snippets."},
	{Method: "get", Path: "/note", Tool: "notes_read", Exec: "read",
		Desc: "Read a single note by its vault-relative path (e.g. '技术/Go/并发模型.md')."},
	{Method: "put", Path: "/note", Tool: "notes_write", Exec: "write",
		Desc: "Create or overwrite a note with Markdown content. Use baseVersion '*' to create a new note, or omit/empty to overwrite an existing one."},
	{Method: "delete", Path: "/note", Tool: "notes_delete", Exec: "delete",
		Desc: "Delete a note or folder by path."},
	{Method: "get", Path: "/links", Tool: "notes_links", Exec: "links",
		Desc: "Get the outgoing wiki-links and backlinks of a note."},
}

// generatedToolDefs builds the MCP tool list from the OpenAPI document, falling
// back to the curated schemas if the spec cannot be parsed.
func generatedToolDefs() []toolDef {
	spec, err := openapi.Document()
	if err != nil {
		return fallbackToolDefs()
	}
	defs := make([]toolDef, 0, len(toolBindings))
	for _, b := range toolBindings {
		op, ok := spec.Paths[b.Path][b.Method]
		if !ok {
			continue
		}
		desc := b.Desc
		if desc == "" {
			desc = op.Summary
		}
		defs = append(defs, toolDef{
			Name:        b.Tool,
			Description: desc,
			InputSchema: schemaFor(op),
		})
	}
	if len(defs) == 0 {
		return fallbackToolDefs()
	}
	return defs
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

// fallbackToolDefs is used when the OpenAPI document is unavailable.
func fallbackToolDefs() []toolDef {
	return []toolDef{
		{Name: "notes_list", Description: "List the note directory tree.",
			InputSchema: obj(map[string]any{})},
		{Name: "notes_search", Description: "Full-text search across notes.",
			InputSchema: obj(map[string]any{"query": str("Search query")}, "query")},
		{Name: "notes_read", Description: "Read a note by path.",
			InputSchema: obj(map[string]any{"path": str("Vault-relative note path")}, "path")},
		{Name: "notes_write", Description: "Create or overwrite a note.",
			InputSchema: obj(map[string]any{
				"path":        str("Vault-relative note path"),
				"body":        str("Markdown body"),
				"baseVersion": str("Optimistic concurrency token"),
				"public":      map[string]any{"type": "boolean"},
			}, "path", "body")},
		{Name: "notes_delete", Description: "Delete a note or folder.",
			InputSchema: obj(map[string]any{"path": str("Vault-relative path")}, "path")},
		{Name: "notes_links", Description: "Outgoing links and backlinks.",
			InputSchema: obj(map[string]any{"path": str("Vault-relative note path")}, "path")},
	}
}
