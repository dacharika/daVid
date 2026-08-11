package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
)

// maxFrameBytes caps a single JSON-RPC frame. Requests here carry paths and
// options, never media, so a frame this large is a runaway client rather than
// legitimate input.
const maxFrameBytes = 8 << 20

// The MCP server exposes the same catalog the CLI dispatches from, over
// JSON-RPC on stdio. tools/call routes back into run() — the identical code path
// `cutvec <cmd>` takes — so the two surfaces cannot drift: a tool added to the
// catalog appears on both, and a fix to a command fixes it for both.
//
// Results are returned as text content carrying the tool's JSON, and failures
// as isError content carrying the structured {code,message,hint,retryable}
// shape, so an agent branches on the same error codes it would from the CLI.

// mcpProtocolVersion is the MCP revision this server implements. A client that
// asks for a different one is echoed its own, since the wire shapes used here
// (initialize / tools/list / tools/call) are stable across revisions.
const mcpProtocolVersion = "2025-06-18"

// JSON-RPC 2.0 envelopes.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC reserved codes used here.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// mcpTool is a catalog tool in MCP's shape. MCP names the human-readable field
// `description` and expects the input schema inline.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema catalog.Schema `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// mcpCmd is the `cutvec mcp serve` entry point. The subcommand is required
// rather than implied so that adding another MCP mode later (an http transport,
// say) does not change what a bare `cutvec mcp` does.
func mcpCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return axerror.New("input.empty", "no mcp subcommand given").
			WithHint("run `cutvec mcp serve` to serve the tool catalog over stdio")
	}
	switch args[0] {
	case "serve":
		if len(args) > 1 {
			return axerror.New("cli.unknown_flag",
				fmt.Sprintf("unexpected argument %q", args[1])).
				WithHint("`cutvec mcp serve` takes no arguments; it speaks JSON-RPC on stdio")
		}
		return mcpServe(stdin, stdout)
	default:
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("unknown mcp subcommand %q", args[0])).
			WithHint("the only subcommand is `serve`")
	}
}

// mcpServe runs the stdio server loop until stdin closes. Notifications (a
// request with no id) are handled without a reply, as JSON-RPC requires.
func mcpServe(stdin io.Reader, stdout io.Writer) error {
	// The transport is newline-delimited JSON, and reading it a line at a time
	// is what makes a bad frame recoverable: a streaming decoder cannot advance
	// past a syntax error, so it would re-read the same bytes forever.
	scan := bufio.NewScanner(stdin)
	scan.Buffer(make([]byte, 0, 64*1024), maxFrameBytes)
	enc := json.NewEncoder(stdout)

	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Reported without an id, and the loop continues, so one bad frame
			// does not end the session.
			if err := enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: json.RawMessage("null"),
				Error: &rpcError{Code: codeParseError, Message: "invalid JSON: " + err.Error()},
			}); err != nil {
				return err
			}
			continue
		}

		result, rerr := dispatchMCP(req)
		if len(req.ID) == 0 {
			continue // notification: acknowledged by doing the work, never answered
		}
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if rerr != nil {
			resp.Error = rerr
		} else {
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	// A read failure ends the session; stdin simply closing does not.
	if err := scan.Err(); err != nil {
		return axerror.New("mcp.transport", "reading stdio failed: "+err.Error()).AsRetryable()
	}
	return nil
}

func dispatchMCP(req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		version := mcpProtocolVersion
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) > 0 && json.Unmarshal(req.Params, &p) == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "cutvec", "version": catalog.Default().Version},
		}, nil

	case "notifications/initialized", "initialized":
		return nil, nil

	case "ping":
		return map[string]any{}, nil

	case "tools/list":
		c := catalog.Default()
		tools := make([]mcpTool, 0, len(c.Tools))
		for _, t := range c.Tools {
			tools = append(tools, mcpTool{
				Name: t.Name, Description: t.Summary, InputSchema: t.InputSchema,
			})
		}
		return map[string]any{"tools": tools}, nil

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
		}
		if p.Name == "" {
			return nil, &rpcError{Code: codeInvalidParams, Message: "no tool name given"}
		}
		if _, ok := catalog.Default().Lookup(p.Name); !ok {
			return nil, &rpcError{
				Code:    codeInvalidParams,
				Message: fmt.Sprintf("no tool named %q; known tools: %s", p.Name, strings.Join(catalog.Default().Names(), ", ")),
			}
		}
		return callTool(p.Name, p.Arguments), nil

	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown method " + req.Method}
	}
}

// callTool runs a catalog tool through the CLI's own dispatch. A tool failure is
// not a protocol failure: it comes back as isError content carrying the
// structured error, which is what lets an agent read the hint and retry.
func callTool(name string, arguments json.RawMessage) mcpCallResult {
	args := []string{name}
	body := arguments
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage("{}")
	}

	// `schema` names its subject positionally rather than in a request body.
	if name == "schema" {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		for _, key := range []string{"command", "name", "tool"} {
			if v, ok := m[key].(string); ok && v != "" {
				args = append(args, v)
				break
			}
		}
	}

	// `timeline` selects its inspect/edit subcommand with an "op" field, so the
	// one MCP tool reaches the same positional subcommands the CLI dispatches on.
	if name == "timeline" {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if v, ok := m["op"].(string); ok && (v == "inspect" || v == "edit") {
			args = append(args, v)
		}
	}

	// `assets` has no default subcommand — assetsCmd rejects an empty argument
	// list outright — so without this the tool is advertised in tools/list and
	// then fails every tools/call with input.empty before reaching a worker,
	// making the whole library-retrieval surface unreachable over MCP. Same
	// "op" routing as timeline above.
	if name == "assets" {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if v, ok := m["op"].(string); ok && (v == "index" || v == "search") {
			args = append(args, v)
		}
	}

	// stdout is captured so the tool's single JSON result comes back as MCP
	// content; progress rides the server's real stderr (its log channel), never
	// the JSON-RPC stream on stdout.
	var out bytes.Buffer
	if err := run(args, bytes.NewReader(body), &out, os.Stderr); err != nil {
		ae, ok := err.(*axerror.Error)
		if !ok {
			ae = axerror.New("internal", err.Error())
		}
		b, _ := json.Marshal(ae)
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: string(b)}},
			IsError: true,
		}
	}
	return mcpCallResult{Content: []mcpContent{{Type: "text", Text: out.String()}}}
}
