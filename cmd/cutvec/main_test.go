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
	useFakeFFmpegWorker(t)
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
	useFakeFFmpegWorker(t)
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("identical bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	type ingestSource struct {
		Hash  string         `json:"hash"`
		Proxy string         `json:"proxy"`
		Audio string         `json:"audio"`
		Probe map[string]any `json:"probe"`
	}
	var first struct {
		Cached bool         `json:"cached"`
		Source ingestSource `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, &first)
	if first.Cached {
		t.Fatalf("first ingest cached = true, want false (cache miss)")
	}

	var second struct {
		Cached bool         `json:"cached"`
		Source ingestSource `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, &second)
	if !second.Cached {
		t.Fatalf("second ingest cached = false, want true (cache hit on unchanged bytes)")
	}
	if first.Source.Hash != second.Source.Hash {
		t.Fatalf("hash changed across runs: %q then %q", first.Source.Hash, second.Source.Hash)
	}
	// The cache hit must return the SAME artifact handles the miss built, without
	// re-running the worker — the no-op re-ingest returns the recorded proxy/audio.
	if second.Source.Proxy != first.Source.Proxy || second.Source.Audio != first.Source.Audio {
		t.Fatalf("cached handles drifted: %+v then %+v", first.Source, second.Source)
	}
	if first.Source.Proxy == "" || first.Source.Audio == "" {
		t.Fatalf("first ingest returned no proxy/audio handles: %+v", first.Source)
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

// useFakeFFmpegWorker points ingest's ffmpeg worker at this test binary re-exec'd
// as TestFFmpegHelperProcess, so ingest drives a real subprocess over real
// stdin/stdout with no ffmpeg or python on PATH — the task 009 fake-worker
// pattern, exercised through the CLI. It sets the env for the current test only.
func useFakeFFmpegWorker(t *testing.T) {
	t.Helper()
	t.Setenv("GO_WANT_FFMPEG_HELPER", "1")
	t.Setenv("CUTVEC_FFMPEG_WORKER", os.Args[0])
	t.Setenv("CUTVEC_FFMPEG_WORKER_ARGS", "-test.run=TestFFmpegHelperProcess")
}

// TestFFmpegHelperProcess is not a real test: when GO_WANT_FFMPEG_HELPER is set
// the test binary re-execs it as a stand-in ffmpeg worker. It reads the JSON
// worker Request from stdin, writes the requested proxy and audio files (a real
// worker would write actual media), and prints a JSON Response carrying probe
// metadata plus the two handles — the versioned stdin/stdout contract task 009
// defined, with zero external toolchain.
func TestFFmpegHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FFMPEG_HELPER") != "1" {
		return
	}
	in, _ := io.ReadAll(os.Stdin)
	var req struct {
		Version int            `json:"version"`
		Op      string         `json:"op"`
		Params  map[string]any `json:"params"`
	}
	_ = json.Unmarshal(in, &req)
	proxy, _ := req.Params["proxy"].(string)
	audio, _ := req.Params["audio"].(string)
	_ = os.WriteFile(proxy, []byte("proxy media"), 0o644)
	_ = os.WriteFile(audio, []byte("audio track"), 0o644)
	resp := map[string]any{
		"version": req.Version,
		"result": map[string]any{
			"probe": map[string]any{"duration": 12.5, "width": 1920, "height": 1080},
			"proxy": proxy,
			"audio": audio,
		},
	}
	out, _ := json.Marshal(resp)
	os.Stdout.Write(out)
	os.Exit(0)
}

// TestIngestBuildsProxyAudioProbe is acceptance #1: `cutvec ingest <video>`
// returns handles for a proxy and audio plus probe metadata. The handles must
// resolve to real files the worker wrote — proving the request reached the
// ffmpeg worker over stdin/stdout and back.
func TestIngestBuildsProxyAudioProbe(t *testing.T) {
	useFakeFFmpegWorker(t)
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("raw video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	var res struct {
		Cached bool `json:"cached"`
		Source struct {
			Proxy string         `json:"proxy"`
			Audio string         `json:"audio"`
			Probe map[string]any `json:"probe"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest", "--json"}, map[string]any{"project": dir, "source": src}, &res)

	if res.Cached {
		t.Fatalf("first ingest cached = true, want false")
	}
	if res.Source.Proxy == "" || res.Source.Audio == "" {
		t.Fatalf("ingest returned no proxy/audio handles: %+v", res.Source)
	}
	for _, h := range []string{res.Source.Proxy, res.Source.Audio} {
		if _, err := os.Stat(h); err != nil {
			t.Fatalf("handle %q does not resolve to a file the worker wrote: %v", h, err)
		}
	}
	if res.Source.Probe == nil || res.Source.Probe["duration"] == nil {
		t.Fatalf("ingest returned no probe metadata: %+v", res.Source.Probe)
	}
}

// TestIngestWithoutWorkerIsMissing pins the failure rule: ingesting a real file
// with no ffmpeg worker configured returns a non-retryable worker.missing — the
// honest failure when the probe/proxy/audio step cannot run.
func TestIngestWithoutWorkerIsMissing(t *testing.T) {
	t.Setenv("CUTVEC_FFMPEG_WORKER", "")
	t.Setenv("CUTVEC_FFMPEG_WORKER_ARGS", "")
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("raw video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"ingest"}, strings.NewReader(`{"project":"`+dir+`","source":"`+src+`"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
	if ae.Retryable {
		t.Fatal("a missing worker is not retryable")
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
