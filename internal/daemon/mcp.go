package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/BenyD/haypile/internal/query"
)

// MCP (Model Context Protocol) support: the daemon speaks the Streamable
// HTTP transport at /mcp, so Claude Code, Cursor, or any MCP client can
// use the user's documents as a knowledge source with one line of config.
// The server is stateless and answers plain JSON — the spec's SSE stream
// is optional and nothing here needs server push.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func rpcResult(id json.RawMessage, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func rpcFail(id json.RawMessage, code int, msg string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": rpcError{Code: code, Message: msg}}
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, rpcFail(nil, -32700, "parse error: "+err.Error()))
		return
	}

	// Notifications have no id and get no body back.
	if req.ID == nil || string(req.ID) == "null" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &params)
		version := params.ProtocolVersion
		if version == "" {
			version = "2025-06-18"
		}
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "haypile", "version": s.version},
		}))
	case "ping":
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]any{}))
	case "tools/list":
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]any{"tools": mcpTools}))
	case "tools/call":
		s.handleMCPToolCall(r.Context(), w, req)
	default:
		writeJSON(w, http.StatusOK, rpcFail(req.ID, -32601, "method not found: "+req.Method))
	}
}

var mcpTools = []map[string]any{
	{
		"name": "search_documents",
		"description": "Search the user's local document index (contracts, notes, papers, PDFs) " +
			"with hybrid keyword + semantic search. Returns the most relevant passages with " +
			"file-and-page citations. Use this whenever a question might be answered by the " +
			"user's own files.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "What to search for; plain language or exact identifiers both work"},
				"tag":   map[string]any{"type": "string", "description": "Optional: restrict to folders indexed with this tag"},
				"limit": map[string]any{"type": "integer", "description": "Max passages to return (default 8)"},
			},
			"required": []string{"query"},
		},
	},
	{
		"name":        "list_sources",
		"description": "List the folders currently indexed in the user's local document index, with file and chunk counts.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
}

func (s *Server) handleMCPToolCall(ctx context.Context, w http.ResponseWriter, req rpcRequest) {
	var params struct {
		Name      string `json:"name"`
		Arguments struct {
			Query string `json:"query"`
			Tag   string `json:"tag"`
			Limit int    `json:"limit"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSON(w, http.StatusOK, rpcFail(req.ID, -32602, "bad params: "+err.Error()))
		return
	}

	text, err := s.runMCPTool(ctx, params.Name, params.Arguments.Query, params.Arguments.Tag, params.Arguments.Limit)
	if err != nil {
		// Tool-level failures are results with isError, not protocol errors,
		// so the model can read and react to them.
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}))
		return
	}
	writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}))
}

func (s *Server) runMCPTool(ctx context.Context, name, q, tag string, limit int) (string, error) {
	switch name {
	case "search_documents":
		if q == "" {
			return "", fmt.Errorf("query is required")
		}
		if limit <= 0 || limit > 50 {
			limit = 8
		}
		results, err := query.Hybrid(ctx, s.st, s.emb, q, tag, limit)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "No matching passages in the indexed documents.", nil
		}
		var b strings.Builder
		for i, r := range results {
			cite := fmt.Sprintf("%s (chunk %d)", r.Path, r.Seq+1)
			if r.Page > 0 {
				cite = fmt.Sprintf("%s (page %d)", r.Path, r.Page)
			}
			// Agents get the full passage: they reason over what they
			// receive, and a truncated snippet reads as the document
			// not containing the answer.
			passage := r.Text
			if passage == "" {
				passage = r.Snippet
			}
			fmt.Fprintf(&b, "[%d] %s\n%s\n\n", i+1, cite, passage)
		}
		return strings.TrimSpace(b.String()), nil
	case "list_sources":
		sources, err := s.st.Sources()
		if err != nil {
			return "", err
		}
		if len(sources) == 0 {
			return "Nothing indexed yet. The user can add folders with: hay add <folder>", nil
		}
		var b strings.Builder
		for _, src := range sources {
			fmt.Fprintf(&b, "%s — %d files, %d chunks", src.Path, src.Files, src.Chunks)
			if src.Tag != "" {
				fmt.Fprintf(&b, " (tag: %s)", src.Tag)
			}
			b.WriteByte('\n')
		}
		return strings.TrimSpace(b.String()), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
