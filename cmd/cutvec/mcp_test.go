package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/catalog"
)

// serveMCP runs the stdio loop over the given JSON-RPC lines and returns the
// decoded responses, exactly as a client would see them.
func serveMCP(t *testing.T, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := mcpServe(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("mcpServe: %v", err)
	}
	var got []map[string]any
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		got = append(got, m)
	}
	return got
}

func TestMCPInitializeAnnouncesToolsCapability(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	res, _ := got[0]["result"].(map[string]any)
	if res == nil {
		t.Fatalf("initialize returned no result: %v", got[0])
	}
	if _, ok := res["capabilities"].(map[string]any)["tools"]; !ok {
		t.Fatalf("initialize does not advertise the tools capability: %v", res)
	}
	if res["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want the client's own version echoed back", res["protocolVersion"])
	}
	if info, _ := res["serverInfo"].(map[string]any); info["name"] != "cutvec" {
		t.Errorf("serverInfo = %v, want name cutvec", info)
	}
}

// The whole point of one catalog backing both surfaces is that they cannot
// drift: every CLI tool must appear over MCP, carrying its schema.
func TestMCPToolsListMirrorsTheCatalog(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	res, _ := got[0]["result"].(map[string]any)
	tools, _ := res["tools"].([]any)

	want := catalog.Default().Names()
	if len(tools) != len(want) {
		t.Fatalf("MCP exposes %d tools, catalog has %d", len(tools), len(want))
	}
	seen := map[string]bool{}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		name, _ := tool["name"].(string)
		seen[name] = true
		if tool["description"] == "" || tool["description"] == nil {
			t.Errorf("tool %q has no description for the agent to read", name)
		}
		if _, ok := tool["inputSchema"].(map[string]any); !ok {
			t.Errorf("tool %q has no inputSchema, so an agent cannot call it correctly", name)
		}
	}
	for _, name := range want {
		if !seen[name] {
			t.Errorf("catalog tool %q is missing from tools/list", name)
		}
	}
}

// tools/call must route into the same dispatch the CLI uses, so a tool's real
// behaviour — here, describe's own catalog — comes back as text content.
func TestMCPToolsCallRunsTheTool(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"describe","arguments":{}}}`)
	res, _ := got[0]["result"].(map[string]any)
	if res["isError"] == true {
		t.Fatalf("describe reported an error: %v", res)
	}
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatal("tools/call returned no content")
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("tool output is not JSON: %v\n%s", err, text)
	}
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("describe did not return the catalog: %s", text)
	}
}

// A tool that fails is not a protocol failure: the structured error has to reach
// the agent as content, or it loses the code and hint it needs to recover.
func TestMCPToolFailureCarriesTheStructuredError(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"transcribe","arguments":{"project":"/nonexistent-project"}}}`)
	if _, isProtocolError := got[0]["error"]; isProtocolError {
		t.Fatalf("a tool failure became a protocol error: %v", got[0])
	}
	res, _ := got[0]["result"].(map[string]any)
	if res["isError"] != true {
		t.Fatalf("failure not flagged with isError: %v", res)
	}
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	var ae struct {
		Code      string `json:"code"`
		Hint      string `json:"hint"`
		Retryable bool   `json:"retryable"`
	}
	if err := json.Unmarshal([]byte(text), &ae); err != nil {
		t.Fatalf("error content is not the structured shape: %v\n%s", err, text)
	}
	if ae.Code == "" || ae.Hint == "" {
		t.Fatalf("error lost its code/hint: %+v", ae)
	}
}

// Calling something that does not exist is a protocol-level error, and it names
// the tools that do exist so an agent can correct itself.
func TestMCPUnknownToolIsAProtocolErrorListingKnownTools(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	rpcErr, ok := got[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown tool did not produce a protocol error: %v", got[0])
	}
	msg, _ := rpcErr["message"].(string)
	if !strings.Contains(msg, "describe") {
		t.Errorf("error does not list known tools: %q", msg)
	}
}

// JSON-RPC forbids replying to a notification; a stray reply desynchronises a
// strict client.
func TestMCPNotificationsGetNoReply(t *testing.T) {
	got := serveMCP(t,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	)
	if len(got) != 1 {
		t.Fatalf("got %d responses, want only the ping's", len(got))
	}
	if id, _ := got[0]["id"].(float64); int(id) != 7 {
		t.Fatalf("responded to the notification instead of the ping: %v", got[0])
	}
}

// One malformed frame must not kill the session, or a client hiccup ends the
// whole run.
func TestMCPMalformedFrameDoesNotEndTheSession(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0",,,}`, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(got) < 2 {
		t.Fatalf("session stopped after a bad frame: %v", got)
	}
	if _, ok := got[0]["error"]; !ok {
		t.Errorf("bad frame did not produce a parse error: %v", got[0])
	}
	if id, _ := got[len(got)-1]["id"].(float64); int(id) != 2 {
		t.Errorf("ping after the bad frame was not answered: %v", got)
	}
}

func TestMCPUnknownMethodIsMethodNotFound(t *testing.T) {
	got := serveMCP(t, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	rpcErr, ok := got[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown method did not error: %v", got[0])
	}
	if code, _ := rpcErr["code"].(float64); int(code) != codeMethodNotFound {
		t.Errorf("code = %v, want %d (method not found)", rpcErr["code"], codeMethodNotFound)
	}
}

// The stdio loop once spun forever on a malformed frame, because a streaming
// decoder cannot advance past a syntax error. That bug was a hang, not a crash,
// so it survived every example-based test until one happened to include a bad
// frame. Fuzzing the frame reader is the check that generalises: whatever the
// input, the loop must terminate and emit only well-formed JSON-RPC.
func FuzzMCPFrames(f *testing.F) {
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	f.Add(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	f.Add(`{"jsonrpc":"2.0",,,}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"describe"}}`)
	f.Add("")
	f.Add("\n\n\n")
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":""}}`)

	f.Fuzz(func(t *testing.T, frame string) {
		var out bytes.Buffer
		// A frame is one line by definition; embedded newlines would just be
		// separate frames and are not what this is exercising.
		frame = strings.ReplaceAll(frame, "\n", " ")
		if err := mcpServe(strings.NewReader(frame+"\n"), &out); err != nil {
			t.Fatalf("serve returned an error for input %q: %v", frame, err)
		}
		// Whatever came back must be decodable JSON-RPC: a client that cannot
		// parse our reply is as stuck as one we never replied to.
		dec := json.NewDecoder(&out)
		for dec.More() {
			var msg map[string]any
			if err := dec.Decode(&msg); err != nil {
				t.Fatalf("emitted invalid JSON for input %q: %v", frame, err)
			}
			if msg["jsonrpc"] != "2.0" {
				t.Fatalf("reply is not JSON-RPC 2.0 for input %q: %v", frame, msg)
			}
			_, hasResult := msg["result"]
			_, hasError := msg["error"]
			if hasResult == hasError {
				t.Fatalf("reply must carry exactly one of result/error, got %v", msg)
			}
		}
	})
}
