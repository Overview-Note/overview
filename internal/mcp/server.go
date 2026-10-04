// Package mcp implements a Model Context Protocol server that exposes the
// Overview note service as MCP tools. It speaks JSON-RPC 2.0 over HTTP
// (POST /mcp) and is authenticated with a bearer token when auth is enabled.
//
// It supports both protocol eras:
//
//   - Modern (2026-07-28+): stateless requests that carry the protocol version
//     in per-request _meta, server/discover, and the required resultType field.
//   - Legacy (2025-11-25 and earlier): the initialize handshake, kept so
//     existing clients keep working.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/tools"
)

// ProtocolVersion is the newest revision this server implements.
const ProtocolVersion = "2026-07-28"

// supportedVersions is advertised via server/discover and used for negotiation.
var supportedVersions = []string{
	"2026-07-28",
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

// ServerVersion is reported as serverInfo.version. The entry point sets it to
// the build version.
var ServerVersion = "1.0.0"

// TokenVerifier validates an MCP bearer token. It returns nil when the token is
// acceptable or an error otherwise. A nil verifier disables token checks.
type TokenVerifier func(ctx context.Context, token string) error

// Server is an MCP JSON-RPC handler.
type Server struct {
	set    *tools.Set
	verify TokenVerifier
}

// New creates an MCP server backed by the note service.
func New(svc *service.Service, verify TokenVerifier) *Server {
	return &Server{set: tools.NewSet(svc), verify: verify}
}

// request is a JSON-RPC 2.0 request.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// ServeHTTP handles a single JSON-RPC request. Notifications (no id) receive
// 202 with an empty body.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.verify != nil {
		token := bearerToken(r)
		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Overview MCP"`)
			writeRPC(w, http.StatusUnauthorized, response{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32001, Message: "unauthorized"},
			})
			return
		}
		if err := s.verify(r.Context(), token); err != nil {
			writeRPC(w, http.StatusUnauthorized, response{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32001, Message: "unauthorized"},
			})
			return
		}
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPC(w, http.StatusOK, response{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	// Modern requests declare their protocol version per request. Reject
	// versions we do not implement with UnsupportedProtocolVersionError so the
	// client can retry with a mutually supported one.
	if version := requestVersion(r, req.Params); version != "" && !supports(version) {
		writeRPC(w, http.StatusOK, response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32022,
				Message: "Unsupported protocol version",
				Data: map[string]any{
					"supported": supportedVersions,
					"requested": version,
				},
			},
		})
		return
	}

	result, rpcErr := s.dispatch(r.Context(), req)
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	resp := response{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	writeRPC(w, http.StatusOK, resp)
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "server/discover":
		return discoverResult(), nil
	case "initialize":
		return initializeResult(req.Params), nil
	case "notifications/initialized", "notifications/cancelled":
		return emptyResult(), nil
	case "ping":
		// Removed in 2026-07-28, retained for legacy clients.
		return emptyResult(), nil
	case "tools/list":
		return map[string]any{"resultType": "complete", "tools": tools.Defs()}, nil
	case "tools/call":
		return s.callTool(ctx, req.Params)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
}

// discoverResult implements server/discover (required by 2026-07-28).
func discoverResult() map[string]any {
	return map[string]any{
		"resultType":        "complete",
		"supportedVersions": supportedVersions,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"instructions": "Overview exposes a Markdown knowledge base: list, search, read, write, delete and inspect notes.",
		"_meta": map[string]any{
			"io.modelcontextprotocol/serverInfo": map[string]any{
				"name":    "overview",
				"version": ServerVersion,
			},
		},
	}
}

// initializeResult implements the legacy handshake (2025-11-25 and earlier).
func initializeResult(params json.RawMessage) map[string]any {
	negotiated := ProtocolVersion
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)
	if p.ProtocolVersion != "" && supports(p.ProtocolVersion) {
		negotiated = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": negotiated,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "overview",
			"version": ServerVersion,
		},
	}
}

func emptyResult() map[string]any {
	return map[string]any{"resultType": "complete"}
}

func supports(version string) bool {
	for _, v := range supportedVersions {
		if v == version {
			return true
		}
	}
	return false
}

// requestVersion extracts the protocol version a request declares, first from
// per-request _meta (modern) and then from the MCP-Protocol-Version header.
func requestVersion(r *http.Request, params json.RawMessage) string {
	if v := metaProtocolVersion(params); v != "" {
		return v
	}
	return strings.TrimSpace(r.Header.Get("MCP-Protocol-Version"))
}

const metaProtocolVersionKey = "io.modelcontextprotocol/protocolVersion"

func metaProtocolVersion(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var env struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &env); err != nil {
		return ""
	}
	raw, ok := env.Meta[metaProtocolVersionKey]
	if !ok {
		return ""
	}
	var v string
	_ = json.Unmarshal(raw, &v)
	return strings.TrimSpace(v)
}

func writeRPC(w http.ResponseWriter, status int, resp response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}
