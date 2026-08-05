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

// runJSON drives run() with a JSON request on stdin and unmarshals the JSON
// result into out. It is the shared harness for the state-model tools, exercising
// the real CLI entry point end to end.
func runJSON(t *testing.T, args []string, req any, out any) {
	t.Helper()
	var stdin io.Reader = strings.NewReader("")
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		stdin = bytes.NewReader(b)
	}
	var buf bytes.Buffer
	if err := run(args, stdin, &buf); err != nil {
		t.Fatalf("run(%v) = %v, want nil", args, err)
	}
	if out != nil {
		if err := json.Unmarshal(buf.Bytes(), out); err != nil {
			t.Fatalf("result is not JSON: %v\n%s", err, buf.String())
		}
	}
}

// TestInitCreatesManifest is acceptance #1: `cutvec init` creates a project
// directory holding a project.json manifest with the expected shape.
func TestInitCreatesManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	var res struct {
		Manifest string `json:"manifest"`
		Name     string `json:"name"`
		Created  bool   `json:"created"`
	}
	runJSON(t, []string{"init"}, map[string]any{"project": dir, "name": "demo"}, &res)

	if !res.Created {
		t.Fatalf("created = false, want true on first init")
	}
	manifest := filepath.Join(dir, "project.json")
	if res.Manifest != manifest {
		t.Fatalf("manifest handle = %q, want %q", res.Manifest, manifest)
	}
	b, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("manifest was not written: %v", err)
	}
	var m struct {
		Version string `json:"version"`
		Name    string `json:"name"`
		Sources []any  `json:"sources"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("manifest is not JSON: %v\n%s", err, b)
	}
	if m.Version == "" || m.Name != "demo" {
		t.Fatalf("manifest = %+v, want version set and name=demo", m)
	}
}

// TestInitIdempotent asserts a second init on an existing project reports
// created=false and leaves the manifest untouched — resumable, never clobbering.
func TestInitIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir, "name": "demo"}, nil)
	before, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}

	var res struct {
		Created bool `json:"created"`
	}
	runJSON(t, []string{"init"}, map[string]any{"project": dir, "name": "ignored"}, &res)
	if res.Created {
		t.Fatalf("created = true on re-init, want false")
	}
	after, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("re-init changed the manifest:\nbefore %s\nafter  %s", before, after)
	}
}

// TestStatusStateGraph is acceptance #2: `cutvec status --json` returns the state
// graph — the sources ingested and the artifacts with their done/pending status.
func TestStatusStateGraph(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("fake video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, nil)

	var graph struct {
		Sources []struct {
			Path string `json:"path"`
			Hash string `json:"hash"`
		} `json:"sources"`
		Artifacts []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"artifacts"`
	}
	runJSON(t, []string{"status", "--json"}, map[string]any{"project": dir}, &graph)

	if len(graph.Sources) != 1 || graph.Sources[0].Path != src || graph.Sources[0].Hash == "" {
		t.Fatalf("sources = %+v, want one entry for %q with a hash", graph.Sources, src)
	}
	tl := findArtifact(graph.Artifacts, "timeline")
	if tl == nil || tl.Status != "pending" {
		t.Fatalf("timeline artifact = %+v, want status pending before it is built", tl)
	}

	// Build the timeline, then the same artifact must flip to done in the graph.
	runJSON(t, []string{"timeline"}, map[string]any{
		"project": dir,
		"clips":   []map[string]any{{"resource": "raw.mp4"}},
	}, nil)
	graph.Artifacts = nil
	runJSON(t, []string{"status", "--json"}, map[string]any{"project": dir}, &graph)
	tl = findArtifact(graph.Artifacts, "timeline")
	if tl == nil || tl.Status != "done" {
		t.Fatalf("timeline artifact = %+v, want status done after it is built", tl)
	}
}

// findArtifact returns the named artifact from a status graph, or nil.
func findArtifact(as []struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}, name string) *struct {
	Name   string `json:"name"`
	Status string `json:"status"`
} {
	for i := range as {
		if as[i].Name == name {
			return &as[i]
		}
	}
	return nil
}

// TestIngestCacheHitIdempotent is acceptance #3: re-running a completed step is a
// content-hash cache hit. Ingesting the same file twice returns cached=false then
// cached=true, and the manifest holds exactly one source — the step changed
// nothing on the second run.
func TestIngestCacheHitIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("identical bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	var first struct {
		Cached bool `json:"cached"`
		Source struct {
			Hash string `json:"hash"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, &first)
	if first.Cached {
		t.Fatalf("first ingest cached = true, want false (cache miss)")
	}

	var second struct {
		Cached bool `json:"cached"`
		Source struct {
			Hash string `json:"hash"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, &second)
	if !second.Cached {
		t.Fatalf("second ingest cached = false, want true (cache hit on unchanged bytes)")
	}
	if first.Source.Hash != second.Source.Hash {
		t.Fatalf("hash changed across runs: %q then %q", first.Source.Hash, second.Source.Hash)
	}

	// Idempotency: the completed step added exactly one source, not two.
	b, err := os.ReadFile(filepath.Join(dir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Sources []any `json:"sources"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("manifest has %d sources after re-ingest, want 1 (idempotent)", len(m.Sources))
	}
}

// TestIngestPlanWritesNothing asserts --plan reports the source and its cache
// status without recording anything: no manifest source, no cache entry.
func TestIngestPlanWritesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Planned bool `json:"planned"`
		Cached  bool `json:"cached"`
	}
	runJSON(t, []string{"ingest", "--plan"}, map[string]any{"project": dir, "source": src}, &res)
	if !res.Planned || res.Cached {
		t.Fatalf("plan result = %+v, want planned:true cached:false", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache")); !os.IsNotExist(err) {
		t.Fatalf("--plan created the cache dir (stat err = %v), want it absent", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "project.json"))
	var m struct {
		Sources []any `json:"sources"`
	}
	_ = json.Unmarshal(b, &m)
	if len(m.Sources) != 0 {
		t.Fatalf("--plan recorded %d sources, want 0", len(m.Sources))
	}
}

// TestStateModelFailureRules pins the written behaviour for the new tools: each
// bad input maps to a stable, machine-branchable axerror code.
func TestStateModelFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
	missingSrc := filepath.Join(t.TempDir(), "nope.mp4")

	cases := []struct {
		name  string
		args  []string
		stdin string
		code  string
	}{
		{name: "init empty stdin", args: []string{"init"}, stdin: "", code: "input.empty"},
		{name: "init no project", args: []string{"init"}, stdin: `{}`, code: "input.invalid"},
		{name: "ingest no project", args: []string{"ingest"}, stdin: `{"source":"a.mp4"}`, code: "input.invalid"},
		{name: "ingest no source", args: []string{"ingest"}, stdin: `{"project":"p"}`, code: "input.invalid"},
		{name: "ingest uninitialized", args: []string{"ingest"}, stdin: `{"project":"` + filepath.Join(t.TempDir(), "none") + `","source":"a.mp4"}`, code: "project.uninitialized"},
		{name: "ingest missing file", args: []string{"ingest"}, stdin: `{"project":"` + initialized + `","source":"` + missingSrc + `"}`, code: "source.missing"},
		{name: "status uninitialized", args: []string{"status", "--json"}, stdin: `{"project":"` + filepath.Join(t.TempDir(), "none") + `"}`, code: "project.uninitialized"},
		{name: "status unknown flag", args: []string{"status", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, strings.NewReader(tc.stdin), io.Discard)
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
