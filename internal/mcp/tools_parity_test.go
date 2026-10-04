package mcp_test

import (
	"testing"

	"github.com/Overview-Note/overview/internal/tools"
)

// TestToolsListMatchesSharedSurface guards that the MCP tools/list surface is
// exactly the shared internal/tools definition.
func TestToolsListMatchesSharedSurface(t *testing.T) {
	ts := newMCPServer(t, nil)
	out := rpc(t, ts, "", "tools/list", map[string]any{})
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list failed: %v", out)
	}
	list, _ := result["tools"].([]any)
	if len(list) != len(tools.Defs()) {
		t.Fatalf("mcp tools = %d, shared defs = %d", len(list), len(tools.Defs()))
	}
	mcpNames := map[string]bool{}
	for _, entry := range list {
		m, _ := entry.(map[string]any)
		name, _ := m["name"].(string)
		mcpNames[name] = true
	}
	for _, d := range tools.Defs() {
		if !mcpNames[d.Name] {
			t.Errorf("shared tool %s missing from MCP surface", d.Name)
		}
	}
}
