package mcp

import (
	"context"
	"encoding/json"

	"github.com/Overview-Note/overview/internal/tools"
)

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
	if !tools.Known(p.Name) {
		return nil, &rpcError{Code: -32602, Message: "unknown tool: " + p.Name}
	}
	result, err := s.set.Exec(ctx, p.Name, p.Arguments)
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
