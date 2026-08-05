package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/axerror"
)

// TestRunDispatch exercises the CLI entry point end-to-end: run() is the single
// dispatcher every human or agent call passes through, so asserting on its
// behaviour is what makes `go test ./...` mean the surface actually works.
func TestRunDispatch(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// code is the expected axerror code, or "" when run should succeed.
		code string
	}{
		{name: "describe succeeds", args: []string{"describe"}, code: ""},
		{name: "no args prints usage", args: nil, code: ""},
		{name: "help succeeds", args: []string{"--help"}, code: ""},
		{name: "mcp is unimplemented", args: []string{"mcp"}, code: "mcp.unimplemented"},
		{name: "unknown command", args: []string{"bogus"}, code: "cli.unknown_command"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, strings.NewReader(""), io.Discard)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("run(%v) = %v, want nil", tc.args, err)
				}
				return
			}
			var ae *axerror.Error
			if !errors.As(err, &ae) {
				t.Fatalf("run(%v) = %v, want *axerror.Error", tc.args, err)
			}
			if ae.Code != tc.code {
				t.Fatalf("run(%v) code = %q, want %q", tc.args, ae.Code, tc.code)
			}
		})
	}
}

// TestDescribeJSONReturnsToolArray is the AC1 proof: `describe --json` emits a
// JSON array of tools, each carrying a name, summary, and both schemas — the
// catalog an agent maps onto its tool list.
func TestDescribeJSONReturnsToolArray(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"describe", "--json"}, strings.NewReader(""), &out); err != nil {
		t.Fatalf("describe --json = %v, want nil", err)
	}
	var tools []struct {
		Name         string         `json:"name"`
		Summary      string         `json:"summary"`
		InputSchema  map[string]any `json:"inputSchema"`
		OutputSchema map[string]any `json:"outputSchema"`
	}
	if err := json.Unmarshal(out.Bytes(), &tools); err != nil {
		t.Fatalf("output is not a JSON array of tools: %v\n%s", err, out.String())
	}
	if len(tools) == 0 {
		t.Fatal("describe --json returned no tools")
	}
	for _, tool := range tools {
		if tool.Name == "" || tool.Summary == "" ||
			len(tool.InputSchema) == 0 || len(tool.OutputSchema) == 0 {
			t.Fatalf("tool missing name/summary/schema: %+v", tool)
		}
	}
}

// TestSchemaReturnsCommandSchema is the AC2 proof: `schema <cmd>` returns the
// JSON Schema for that one command, declaring the draft dialect and nesting the
// command's input and output schemas.
func TestSchemaReturnsCommandSchema(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"schema", "timeline"}, strings.NewReader(""), &out); err != nil {
		t.Fatalf("schema timeline = %v, want nil", err)
	}
	var s map[string]any
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, out.String())
	}
	if s["title"] != "timeline" {
		t.Errorf("title = %v, want timeline", s["title"])
	}
	if s["$schema"] == nil {
		t.Error("schema does not declare a $schema dialect")
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is %T, want object", s["properties"])
	}
	if props["input"] == nil || props["output"] == nil {
		t.Errorf("schema missing input/output: %v", props)
	}
}

// TestSchemaFailureRules pins schema's behaviour on empty and unknown input to
// stable, branchable axerror codes.
func TestSchemaFailureRules(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code string
	}{
		{name: "no command", args: []string{"schema"}, code: "input.empty"},
		{name: "unknown command", args: []string{"schema", "nope"}, code: "cli.unknown_command"},
		{name: "too many args", args: []string{"schema", "a", "b"}, code: "input.invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, strings.NewReader(""), io.Discard)
			var ae *axerror.Error
			if !errors.As(err, &ae) {
				t.Fatalf("err = %v, want *axerror.Error", err)
			}
			if ae.Code != tc.code {
				t.Fatalf("code = %q, want %q", ae.Code, tc.code)
			}
		})
	}
}

// TestDescribeUnknownFlag pins describe's rejection of an unknown flag.
func TestDescribeUnknownFlag(t *testing.T) {
	err := run([]string{"describe", "--bogus"}, strings.NewReader(""), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "cli.unknown_flag" {
		t.Fatalf("code = %q, want cli.unknown_flag", ae.Code)
	}
}

// TestTimelineSlice is the end-to-end proof of the first feature slice: a JSON
// request enters at the CLI entry point, is carried through catalog/timeline/
// project, and returns a handle to an MLT document that was really written to
// disk. It fails if any link in that path breaks (no result, wrong producer
// count, or the handle points at a file that was never written).
func TestTimelineSlice(t *testing.T) {
	dir := t.TempDir()
	reqBytes, err := json.Marshal(map[string]any{
		"project": dir,
		"clips": []map[string]any{
			{"id": "cam1", "resource": "raw.mp4"},
			{"resource": "broll.mp4"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := run([]string{"timeline"}, bytes.NewReader(reqBytes), &out); err != nil {
		t.Fatalf("run timeline = %v, want nil", err)
	}

	var res struct {
		Timeline  string `json:"timeline"`
		Producers int    `json:"producers"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.Producers != 2 {
		t.Fatalf("producers = %d, want 2", res.Producers)
	}

	// The returned handle must resolve to a real MLT document containing the
	// clips we asked for — proving the request reached storage and back.
	got, err := os.ReadFile(res.Timeline)
	if err != nil {
		t.Fatalf("timeline handle %q was not written: %v", res.Timeline, err)
	}
	xml := string(got)
	for _, want := range []string{"<mlt", "raw.mp4", "broll.mp4", `id="cam1"`, `id="producer1"`} {
		if !strings.Contains(xml, want) {
			t.Fatalf("timeline XML missing %q:\n%s", want, xml)
		}
	}
}

// TestTimelinePlanWritesNothing asserts the --plan dry-run returns a result but
// leaves the filesystem untouched, per the AX contract's plan/dry-run rule.
func TestTimelinePlanWritesNothing(t *testing.T) {
	dir := t.TempDir()
	reqBytes, _ := json.Marshal(map[string]any{
		"project": dir,
		"clips":   []map[string]any{{"resource": "raw.mp4"}},
	})

	var out bytes.Buffer
	if err := run([]string{"timeline", "--plan"}, bytes.NewReader(reqBytes), &out); err != nil {
		t.Fatalf("run timeline --plan = %v, want nil", err)
	}
	var res struct {
		Planned bool `json:"planned"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil || !res.Planned {
		t.Fatalf("plan result = %s, want planned:true (err %v)", out.String(), err)
	}
	if _, err := os.Stat(filepath.Join(dir, "timeline.mlt")); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote a file (stat err = %v), want it absent", err)
	}
}

// TestTimelineFailureRules pins the written behaviour for empty and malformed
// input: each maps to a stable, machine-branchable axerror code.
func TestTimelineFailureRules(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
		code  string
	}{
		{name: "empty stdin", stdin: "", code: "input.empty"},
		{name: "no clips", stdin: `{"project":"p","clips":[]}`, code: "input.empty"},
		{name: "no project", stdin: `{"clips":[{"resource":"a.mp4"}]}`, code: "input.invalid"},
		{name: "empty resource", stdin: `{"project":"p","clips":[{"id":"a"}]}`, code: "input.invalid"},
		{name: "malformed json", stdin: `{not json`, code: "input.invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run([]string{"timeline"}, strings.NewReader(tc.stdin), io.Discard)
			var ae *axerror.Error
			if !errors.As(err, &ae) {
				t.Fatalf("err = %v, want *axerror.Error", err)
			}
			if ae.Code != tc.code {
				t.Fatalf("code = %q, want %q", ae.Code, tc.code)
			}
		})
	}
}
