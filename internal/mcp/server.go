// Package mcp implements a minimal Model Context Protocol server that exposes
// the Overview note service as MCP tools. It speaks JSON-RPC 2.0 over HTTP
// (POST /mcp) and is authenticated with a bearer token when auth is enabled.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/overview-app/overview/internal/service"
)

const protocolVersion = "2024-11-05"

// TokenVerifier validates an MCP bearer token. It returns nil when the token is
// acceptable or an error otherwise. A nil verifier disables token checks.
type TokenVerifier func(ctx context.Context, token string) error

// Server is an MCP JSON-RPC handler.
type Server struct {
	svc    *service.Service
	verify TokenVerifier
}

// New creates an MCP server backed by the note service.
func New(svc *service.Service, verify TokenVerifier) *Server {
	return &Server{svc: svc, verify: verify}
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
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "overview",
				"version": "1.0.0",
			},
		}, nil
	case "notifications/initialized", "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		return s.callTool(ctx, req.Params)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
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
