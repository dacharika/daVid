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
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/worker"
)

// TestHelperProcess is not a real test: when GO_WANT_HELPER_PROCESS is set it is
// re-exec'd (via `cutvec transcribe`'s worker path pointing at this test binary
// with -test.run=TestHelperProcess) as a stand-in whisper worker. It reads the
// JSON worker Request from stdin and prints a JSON Response to stdout, exercising
// the real subprocess round-trip without a python interpreter or whisper model.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	in, _ := io.ReadAll(os.Stdin)
	var req worker.Request
	_ = json.Unmarshal(in, &req)

	switch os.Getenv("WORKER_MODE") {
	case "transcribe":
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"language": "en",
				"text":     "hello world",
				"words": []map[string]any{
					{"word": "hello", "start": 0.0, "end": 0.5},
					{"word": "world", "start": 0.5, "end": 1.0},
				},
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "render":
		// Stand in for melt: write a fake rendered video to the requested output
		// path, then report success — exercising the real subprocess round-trip
		// and the Go side's "did the worker write the file?" verification.
		outPath, _ := req.Params["output"].(string)
		if outPath != "" {
			_ = os.WriteFile(outPath, []byte("fake rendered video"), 0o644)
		}
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result:  map[string]any{"output": outPath},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	}
	os.Exit(0)
}

// transcribeWorkerEnv points transcribe at this test binary as the whisper
// worker: t.Setenv marks it a helper process running only TestHelperProcess in
// the given WORKER_MODE. The worker inherits these vars (Invoke leaves the child
// env alone unless Worker.Env is set), and the request carries the path + args.
func transcribeWorkerEnv(t *testing.T, mode string) (path string, args []string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", mode)
	return os.Args[0], []string{"-test.run=TestHelperProcess"}
}

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

// ingestFixture inits a project and ingests a source with the given bytes,
// returning the project dir and the source path — the setup every transcribe
// test starts from.
func ingestFixture(t *testing.T, contents string) (dir, src string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)
	src = filepath.Join(t.TempDir(), "raw.wav")
	if err := os.WriteFile(src, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, nil)
	return dir, src
}

// transcribeReply is the JSON handle transcribe returns.
type transcribeReply struct {
	Transcript string `json:"transcript"`
	Source     string `json:"source"`
	Words      int    `json:"words"`
	Language   string `json:"language"`
	Cached     bool   `json:"cached"`
	Planned    bool   `json:"planned"`
}

// TestTranscribeWordLevel is acceptance #1: `cutvec transcribe <project> --json`
// returns a handle, and the transcript at that handle carries word-level
// start/end times. The project is given as a positional argument (the AC form).
func TestTranscribeWordLevel(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes")
	wpath, wargs := transcribeWorkerEnv(t, "transcribe")

	var res transcribeReply
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)
	var out bytes.Buffer
	if err := run([]string{"transcribe", dir, "--json"}, bytes.NewReader(b), &out); err != nil {
		t.Fatalf("transcribe = %v, want nil", err)
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.Cached {
		t.Fatal("first transcribe cached = true, want false (cache miss)")
	}
	if res.Words != 2 || res.Language != "en" || res.Transcript == "" || res.Source == "" {
		t.Fatalf("result = %+v, want words:2 language:en with a handle and source hash", res)
	}

	// The handle points at a real file whose transcript has word-level timing.
	tb, err := os.ReadFile(res.Transcript)
	if err != nil {
		t.Fatalf("transcript handle %q is not readable: %v", res.Transcript, err)
	}
	var got project.Transcript
	if err := json.Unmarshal(tb, &got); err != nil {
		t.Fatalf("transcript is not JSON: %v\n%s", err, tb)
	}
	if len(got.Words) != 2 {
		t.Fatalf("transcript has %d words, want 2", len(got.Words))
	}
	if got.Words[0].Word != "hello" || got.Words[0].Start != 0.0 || got.Words[0].End != 0.5 {
		t.Fatalf("word[0] = %+v, want {hello 0 0.5}", got.Words[0])
	}
	if got.Words[1].Start != 0.5 || got.Words[1].End != 1.0 {
		t.Fatalf("word[1] timing = %+v, want start 0.5 end 1.0", got.Words[1])
	}
	if got.Source != res.Source {
		t.Fatalf("transcript source = %q, want the returned handle's source %q", got.Source, res.Source)
	}
}

// TestTranscribeCacheHitIdempotent is acceptance #2: the transcript is cached and
// idempotent. The second run is a content-hash cache hit that returns cached=true
// and runs NO worker — proven by pointing it at a non-existent worker, which
// would fail with worker.missing if the cache were bypassed.
func TestTranscribeCacheHitIdempotent(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes")
	wpath, wargs := transcribeWorkerEnv(t, "transcribe")

	var first transcribeReply
	runJSON(t, []string{"transcribe", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &first)
	if first.Cached {
		t.Fatal("first transcribe cached = true, want false")
	}

	// Second run with a worker that cannot start: a cache hit never touches it.
	var second transcribeReply
	runJSON(t, []string{"transcribe", dir}, map[string]any{"worker": "/nonexistent/whisper-worker"}, &second)
	if !second.Cached {
		t.Fatal("second transcribe cached = false, want true (cache hit on unchanged bytes)")
	}
	if second.Transcript != first.Transcript || second.Source != first.Source || second.Words != first.Words {
		t.Fatalf("cache hit changed the handle: first %+v second %+v", first, second)
	}
}

// TestTranscribePlanWritesNothing asserts --plan reports the handle and cache
// status without running the worker or writing a transcript.
func TestTranscribePlanWritesNothing(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes")

	var res transcribeReply
	// No worker configured: --plan must still succeed since it runs none.
	runJSON(t, []string{"transcribe", dir, "--plan"}, map[string]any{}, &res)
	if !res.Planned || res.Cached {
		t.Fatalf("plan result = %+v, want planned:true cached:false", res)
	}
	if res.Transcript == "" {
		t.Fatal("plan result has no transcript handle")
	}
	if _, err := os.Stat(res.Transcript); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote the transcript (stat err = %v), want it absent", err)
	}
}

// TestTranscribeWorkerErrorMapsThrough asserts a structured error the worker
// reports is surfaced as its axerror code, not masked as a generic failure.
func TestTranscribeWorkerErrorMapsThrough(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes")
	// A worker that exits non-zero with no structured error is a retryable crash.
	err := run([]string{"transcribe", dir},
		strings.NewReader(`{"worker":"/nonexistent/whisper-worker"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
}

// TestTranscribeFailureRules pins the written behaviour for empty and malformed
// input: each maps to a stable, machine-branchable axerror code.
func TestTranscribeFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
	// Initialised but no sources ingested -> nothing to transcribe.
	empty := filepath.Join(t.TempDir(), "empty")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+empty+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		args  []string
		stdin string
		code  string
	}{
		{name: "no project", args: []string{"transcribe"}, stdin: "", code: "input.invalid"},
		{name: "two positional projects", args: []string{"transcribe", "a", "b"}, stdin: "", code: "input.invalid"},
		{name: "unknown flag", args: []string{"transcribe", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
		{name: "uninitialized", args: []string{"transcribe", filepath.Join(t.TempDir(), "none")}, stdin: "", code: "project.uninitialized"},
		{name: "no sources", args: []string{"transcribe", empty}, stdin: "", code: "input.empty"},
		{name: "malformed json", args: []string{"transcribe", initialized}, stdin: `{not json`, code: "input.invalid"},
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

// renderWorkerEnv points render at this test binary as the melt worker: t.Setenv
// marks it a helper process running only TestHelperProcess in the given
// WORKER_MODE. The worker inherits these vars (Invoke leaves the child env alone
// unless Worker.Env is set), and the request carries the path + args.
func renderWorkerEnv(t *testing.T, mode string) (path string, args []string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", mode)
	return os.Args[0], []string{"-test.run=TestHelperProcess"}
}

// renderFixture inits a project and builds a one-clip timeline into it, returning
// the project dir — the setup every render test starts from (render needs a built
// timeline to hash and render).
func renderFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)
	runJSON(t, []string{"timeline"}, map[string]any{
		"project": dir,
		"clips":   []map[string]any{{"resource": "raw.mp4"}},
	}, nil)
	return dir
}

// renderReply is the JSON handle render returns.
type renderReply struct {
	Output   string `json:"output"`
	Timeline string `json:"timeline"`
	Source   string `json:"source"`
	Quality  string `json:"quality"`
	Cached   bool   `json:"cached"`
	Planned  bool   `json:"planned"`
}

// TestRenderProducesOutput is acceptance #1: `cutvec render <project> --json`
// returns an output handle that resolves to a video the worker really wrote. The
// project is given as a positional argument (the AC form) and no quality flag is
// passed, so it defaults to the proxy tier.
func TestRenderProducesOutput(t *testing.T) {
	dir := renderFixture(t)
	wpath, wargs := renderWorkerEnv(t, "render")

	var res renderReply
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)
	var out bytes.Buffer
	if err := run([]string{"render", dir, "--json"}, bytes.NewReader(b), &out); err != nil {
		t.Fatalf("render = %v, want nil", err)
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.Cached {
		t.Fatal("first render cached = true, want false (cache miss)")
	}
	if res.Output == "" || res.Source == "" {
		t.Fatalf("result = %+v, want an output handle and source hash", res)
	}
	if res.Quality != "proxy" {
		t.Fatalf("quality = %q, want proxy (the default)", res.Quality)
	}
	// The handle points at a real file the worker wrote — proving the request
	// reached the worker and back.
	if _, err := os.ReadFile(res.Output); err != nil {
		t.Fatalf("output handle %q was not written: %v", res.Output, err)
	}
}

// TestRenderProxyAndFinalQuality is acceptance #2: render supports both the
// --proxy and --final quality tiers, each producing its own distinct output so
// the two renders never clobber each other.
func TestRenderProxyAndFinalQuality(t *testing.T) {
	dir := renderFixture(t)
	wpath, wargs := renderWorkerEnv(t, "render")
	workerReq := map[string]any{"worker": wpath, "workerArgs": wargs}

	var proxy renderReply
	runJSON(t, []string{"render", dir, "--proxy"}, workerReq, &proxy)
	if proxy.Quality != "proxy" || proxy.Cached {
		t.Fatalf("proxy render = %+v, want quality proxy cached false", proxy)
	}

	var final renderReply
	runJSON(t, []string{"render", dir, "--final"}, workerReq, &final)
	if final.Quality != "final" || final.Cached {
		t.Fatalf("final render = %+v, want quality final cached false", final)
	}

	if proxy.Output == final.Output {
		t.Fatalf("proxy and final share an output path %q, want distinct handles", proxy.Output)
	}
	for _, r := range []renderReply{proxy, final} {
		if _, err := os.Stat(r.Output); err != nil {
			t.Fatalf("%s output %q not written: %v", r.Quality, r.Output, err)
		}
	}
}

// TestRenderCacheHitIdempotent asserts the render is cached and idempotent: the
// second run is a content-hash cache hit that returns cached=true and runs NO
// worker — proven by pointing it at a non-existent worker, which would fail with
// worker.missing if the cache were bypassed.
func TestRenderCacheHitIdempotent(t *testing.T) {
	dir := renderFixture(t)
	wpath, wargs := renderWorkerEnv(t, "render")

	var first renderReply
	runJSON(t, []string{"render", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &first)
	if first.Cached {
		t.Fatal("first render cached = true, want false")
	}

	// Second run with a worker that cannot start: a cache hit never touches it.
	var second renderReply
	runJSON(t, []string{"render", dir}, map[string]any{"worker": "/nonexistent/melt-worker"}, &second)
	if !second.Cached {
		t.Fatal("second render cached = false, want true (cache hit on unchanged timeline)")
	}
	if second.Output != first.Output || second.Source != first.Source {
		t.Fatalf("cache hit changed the handle: first %+v second %+v", first, second)
	}
}

// TestRenderPlanWritesNothing asserts --plan reports the output handle and cache
// status without running the worker or writing a video.
func TestRenderPlanWritesNothing(t *testing.T) {
	dir := renderFixture(t)

	var res renderReply
	// No worker configured: --plan must still succeed since it runs none.
	runJSON(t, []string{"render", dir, "--plan"}, map[string]any{}, &res)
	if !res.Planned || res.Cached {
		t.Fatalf("plan result = %+v, want planned:true cached:false", res)
	}
	if res.Output == "" {
		t.Fatal("plan result has no output handle")
	}
	if _, err := os.Stat(res.Output); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote the output (stat err = %v), want it absent", err)
	}
}

// TestRenderWorkerErrorMapsThrough asserts a worker that cannot start surfaces as
// its axerror code, not masked as a generic failure.
func TestRenderWorkerErrorMapsThrough(t *testing.T) {
	dir := renderFixture(t)
	err := run([]string{"render", dir},
		strings.NewReader(`{"worker":"/nonexistent/melt-worker"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
}

// TestRenderStatusArtifacts asserts the state graph reports per-quality render
// artifacts, flipping from pending to done once a tier is rendered.
func TestRenderStatusArtifacts(t *testing.T) {
	dir := renderFixture(t)

	var graph struct {
		Artifacts []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"artifacts"`
	}
	runJSON(t, []string{"status", "--json"}, map[string]any{"project": dir}, &graph)
	if a := findArtifact(graph.Artifacts, "render:proxy"); a == nil || a.Status != "pending" {
		t.Fatalf("render:proxy = %+v, want status pending before it is rendered", a)
	}

	wpath, wargs := renderWorkerEnv(t, "render")
	runJSON(t, []string{"render", dir, "--proxy"}, map[string]any{"worker": wpath, "workerArgs": wargs}, nil)

	graph.Artifacts = nil
	runJSON(t, []string{"status", "--json"}, map[string]any{"project": dir}, &graph)
	if a := findArtifact(graph.Artifacts, "render:proxy"); a == nil || a.Status != "done" {
		t.Fatalf("render:proxy = %+v, want status done after it is rendered", a)
	}
}

// TestRenderFailureRules pins the written behaviour for empty, malformed, and
// out-of-order input: each maps to a stable, machine-branchable axerror code.
func TestRenderFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
	built := renderFixture(t)

	cases := []struct {
		name  string
		args  []string
		stdin string
		code  string
	}{
		{name: "no project", args: []string{"render"}, stdin: "", code: "input.invalid"},
		{name: "two positional projects", args: []string{"render", "a", "b"}, stdin: "", code: "input.invalid"},
		{name: "unknown flag", args: []string{"render", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
		{name: "both quality flags", args: []string{"render", built, "--proxy", "--final"}, stdin: "", code: "input.invalid"},
		{name: "unknown quality", args: []string{"render", built}, stdin: `{"quality":"ultra"}`, code: "input.invalid"},
		{name: "uninitialized", args: []string{"render", filepath.Join(t.TempDir(), "none")}, stdin: "", code: "project.uninitialized"},
		{name: "no timeline built", args: []string{"render", initialized}, stdin: "", code: "timeline.missing"},
		{name: "malformed json", args: []string{"render", built}, stdin: `{not json`, code: "input.invalid"},
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
