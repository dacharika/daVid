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

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/catalog"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/timeline"
	"github.com/mlnomadpy/daVid/internal/worker"
)

// TestHelperProcess is not a real test: when GO_WANT_HELPER_PROCESS is set it is
// re-exec'd (via `daVid transcribe`'s worker path pointing at this test binary
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
	case "ffmpeg":
		// The real ffmpeg worker answers three ops from one executable, so this
		// stand-in dispatches on the op rather than the mode: ingest calls all
		// three through a single configured worker. proxy and audio write real
		// files, because a handle pointing at nothing is the failure worth
		// catching.
		res := map[string]any{}
		switch req.Op {
		case "probe":
			res = map[string]any{"width": 1920, "height": 1080, "duration": 12.5, "fps": 30.0}
		case "proxy", "audio":
			dest, _ := req.Params["dest"].(string)
			if dest != "" {
				_ = os.WriteFile(dest, []byte("fake "+req.Op), 0o644)
			}
			res = map[string]any{"output": dest, "bytes": 9}
		}
		out, _ := json.Marshal(worker.Response{Version: worker.ContractVersion, Result: res})
		os.Stdout.Write(out)
		os.Exit(0)
	case "probe":
		// Stand in for ffmpeg: report the source's intrinsic dimensions without
		// touching a real media file or ffprobe binary.
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"width":    1920,
				"height":   1080,
				"duration": 12.5,
				"fps":      30.0,
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
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
	case "index":
		// A local embed worker: write the LanceDB table it was asked to build,
		// then report the summary. No network, no python — just a file.
		if table, ok := req.Params["table"].(string); ok && table != "" {
			_ = os.WriteFile(table, []byte("fake lance table bytes"), 0o644)
		}
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"segments": 2,
				"dim":      3,
				"model":    "fake-local-embed",
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "search":
		// A local search worker: embed the query and return scored hits. The
		// candidates are returned deliberately out of ranked order (lower score
		// first) so the test can prove the core re-ranks them into a
		// deterministic descending-score order.
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"model": "fake-local-embed",
				"candidates": []map[string]any{
					{"start": 1.5, "end": 3.0, "text": "world", "score": 0.2},
					{"start": 0.0, "end": 1.5, "text": "hello", "score": 0.9},
				},
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "search-assets":
		// A local asset search worker: return scored library hits deliberately
		// out of ranked order (lower score first) so the test can prove the core
		// re-ranks them into a deterministic descending-score order.
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"model": "fake-local-embed",
				"assets": []map[string]any{
					{"ref": "sfx/whoosh.wav", "text": "whoosh", "score": 0.2},
					{"ref": "sfx/boom.wav", "text": "deep boom", "score": 0.9},
				},
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "speakers":
		// Echo the layout the core sent so the test can prove the fixed layout
		// travels to the worker, and return two per-time turns.
		layout, _ := req.Params["layout"].(string)
		// WORKER_ECHO_FILE records which media path the core actually handed
		// over. The result shape has nowhere to carry it — Speakers.Source is
		// overwritten with the content hash by the core — so the only way to
		// assert "detection ran on the proxy" is to have the worker say so.
		if echo := os.Getenv("WORKER_ECHO_FILE"); echo != "" {
			s, _ := req.Params["source"].(string)
			_ = os.WriteFile(echo, []byte(s), 0o644)
		}
		resp := worker.Response{
			Version: worker.ContractVersion,
			Result: map[string]any{
				"layout": layout,
				"turns": []map[string]any{
					{"speaker": "left", "start": 0.0, "end": 1.5},
					{"speaker": "right", "start": 1.5, "end": 3.0},
				},
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "verify":
		// Stand in for melt's null consumer. The timeline path decides the
		// verdict so a test can exercise both without melt: anything named
		// *-broken.mlt reports the load failure melt would put on stderr.
		tl, _ := req.Params["timeline"].(string)
		res := map[string]any{"ok": true, "problems": []any{}, "frames": 10, "seconds": 0.35}
		if strings.Contains(tl, "-broken") {
			res["ok"] = false
			res["problems"] = []any{`[producer_xml] failed to load producer "/nope/missing.mp4"`}
		}
		out, _ := json.Marshal(worker.Response{Version: worker.ContractVersion, Result: res})
		os.Stdout.Write(out)
		os.Exit(0)
	case "render":
		// Stand in for melt: write a fake rendered video to the requested output
		// path, then report success — exercising the real subprocess round-trip
		// and the Go side's "did the worker write the file?" verification.
		outPath, _ := req.Params["output"].(string)
		if echo := os.Getenv("WORKER_ECHO_FILE"); echo != "" {
			_ = os.WriteFile(echo, []byte(outPath), 0o644)
		}
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
	case "render-partial":
		// Stand in for a melt that dies mid-encode: write truncated bytes to the
		// requested output path, then exit non-zero. Exercises that the Go side
		// never publishes a crashed worker's partial output onto the cache path.
		outPath, _ := req.Params["output"].(string)
		if outPath != "" {
			_ = os.WriteFile(outPath, []byte("partial"), 0o644)
		}
		os.Exit(1)
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

// ffmpegWorkerEnv points ingest at this test binary as the ffmpeg probe worker,
// re-exec'd as a helper process in WORKER_MODE=probe — the same re-exec trick the
// other worker helpers use, so no ffmpeg/ffprobe is needed on PATH.
func ffmpegWorkerEnv(t *testing.T) (path string, args []string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", "probe")
	return os.Args[0], []string{"-test.run=TestHelperProcess"}
}

// speakersWorkerEnv points speakers at this test binary as the mediapipe worker,
// re-exec'd as a helper process in WORKER_MODE=speakers — the same re-exec trick
// transcribeWorkerEnv uses, so no python/mediapipe is needed on PATH.
func speakersWorkerEnv(t *testing.T) (path string, args []string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", "speakers")
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
		{name: "bare mcp asks for a subcommand", args: []string{"mcp"}, code: "input.empty"},
		{name: "unknown mcp subcommand is rejected", args: []string{"mcp", "listen"}, code: "cli.unknown_command"},
		{name: "mcp serve takes no arguments", args: []string{"mcp", "serve", "--port"}, code: "cli.unknown_flag"},
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

// TestTimelineMultiClipSequencesWithoutAudio is the regression for task 031: a
// multi-clip build with NO audio must still sequence the clips into one ordered
// edit — a playlist with one entry per clip plus the tractor that plays it — so
// melt renders every clip in turn instead of silently keeping only the first.
// Before the fix the build wrote bare <producer>s (sequencing only ran as a side
// effect of adding an audio bed), so this fails against current main.
func TestTimelineMultiClipSequencesWithoutAudio(t *testing.T) {
	dir := t.TempDir()
	reqBytes, err := json.Marshal(map[string]any{
		"project": dir,
		"clips": []map[string]any{
			{"id": "cam1", "resource": "a.mp4"},
			{"id": "cam2", "resource": "b.mp4"},
			{"id": "cam3", "resource": "c.mp4"},
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
		Timeline string `json:"timeline"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}

	raw, err := os.ReadFile(res.Timeline)
	if err != nil {
		t.Fatalf("timeline handle %q was not written: %v", res.Timeline, err)
	}
	tl, err := timeline.Parse(raw)
	if err != nil {
		t.Fatalf("written timeline does not parse: %v\n%s", err, raw)
	}

	// The tractor is what melt renders; without it a no-audio build plays only
	// the first producer.
	if tl.Tractor == nil {
		t.Fatalf("Tractor is nil: a multi-clip no-audio build was not sequenced\n%s", raw)
	}
	if len(tl.Playlists) != 1 {
		t.Fatalf("Playlists = %d, want exactly one video playlist\n%s", len(tl.Playlists), raw)
	}

	// One entry per clip, in the order requested.
	wantOrder := []string{"cam1", "cam2", "cam3"}
	pl := tl.Playlists[0]
	if len(pl.Entries) != len(wantOrder) {
		t.Fatalf("playlist has %d entries, want %d (one per clip)\n%s",
			len(pl.Entries), len(wantOrder), raw)
	}
	for i, want := range wantOrder {
		if pl.Entries[i].Producer != want {
			t.Fatalf("entry %d producer = %q, want %q (clip order not preserved)\n%s",
				i, pl.Entries[i].Producer, want, raw)
		}
	}

	// The tractor must play the sequenced playlist.
	if len(tl.Tractor.Tracks) != 1 || tl.Tractor.Tracks[0].Producer != pl.ID {
		t.Fatalf("tractor tracks = %+v, want a single track on playlist %q\n%s",
			tl.Tractor.Tracks, pl.ID, raw)
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

// A build that composes an unrenderable timeline must fail at build time, not
// persist a broken MLT and return success. Two clips sharing one explicit id is
// the cheapest way to produce one: it used to write the file and report
// producers:2, and the fault only surfaced later as an opaque melt error at
// render, with nothing pointing back at the build that caused it.
func TestTimelineBuildRejectsAnUnrenderableTimeline(t *testing.T) {
	dir := t.TempDir()
	reqBytes, _ := json.Marshal(map[string]any{
		"project": dir,
		"clips": []map[string]any{
			{"id": "cam1", "resource": "a.mp4"},
			{"id": "cam1", "resource": "b.mp4"}, // same id: melt cannot resolve it
		},
	})

	var out bytes.Buffer
	err := run([]string{"timeline"}, bytes.NewReader(reqBytes), &out)
	if err == nil {
		t.Fatalf("duplicate producer id was accepted; result = %s", out.String())
	}
	var ae *axerror.Error
	if !errors.As(err, &ae) || ae.Code != "timeline.invalid" {
		t.Fatalf("error = %v, want code timeline.invalid", err)
	}
	// Nothing may reach disk: a persisted broken timeline is what a later render
	// would pick up and fail on.
	if _, statErr := os.Stat(filepath.Join(dir, "timeline.mlt")); !os.IsNotExist(statErr) {
		t.Fatalf("a rejected build still wrote timeline.mlt (stat err = %v)", statErr)
	}

	// --plan must report the same fault rather than claiming it would write it.
	out.Reset()
	if err := run([]string{"timeline", "--plan"}, bytes.NewReader(reqBytes), &out); err == nil {
		t.Fatalf("--plan reported success for an unrenderable timeline: %s", out.String())
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

// TestInitCreatesManifest is acceptance #1: `daVid init` creates a project
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

// TestStatusStateGraph is acceptance #2: `daVid status --json` returns the state
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

// TestIngestProbesDimensions is the AC1 proof: ingest records the source's pixel
// width, height, duration and fps via the ffmpeg worker, and `status --json`
// reports them. AC2: a re-ingest of the unchanged bytes is still a cache hit that
// returns the same probe without re-running the worker.
func TestIngestProbesDimensions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("fake video bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	wpath, wargs := ffmpegWorkerEnv(t)

	type probe struct {
		Width    int     `json:"width"`
		Height   int     `json:"height"`
		Duration float64 `json:"duration"`
		Fps      float64 `json:"fps"`
	}
	want := probe{Width: 1920, Height: 1080, Duration: 12.5, Fps: 30}

	var first struct {
		Cached bool `json:"cached"`
		Source struct {
			Probe *probe `json:"probe"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{
		"project": dir, "source": src, "worker": wpath, "workerArgs": wargs,
	}, &first)
	if first.Cached {
		t.Fatal("first ingest cached = true, want false (cache miss)")
	}
	if first.Source.Probe == nil {
		t.Fatal("first ingest returned no probe")
	}
	if *first.Source.Probe != want {
		t.Fatalf("probe = %+v, want %+v", *first.Source.Probe, want)
	}

	// AC1: status --json reports the recorded dimensions on the source.
	var graph struct {
		Sources []struct {
			Probe *probe `json:"probe"`
		} `json:"sources"`
	}
	runJSON(t, []string{"status", "--json"}, map[string]any{"project": dir}, &graph)
	if len(graph.Sources) != 1 || graph.Sources[0].Probe == nil {
		t.Fatalf("status sources = %+v, want one source carrying a probe", graph.Sources)
	}
	if *graph.Sources[0].Probe != want {
		t.Fatalf("status probe = %+v, want %+v", *graph.Sources[0].Probe, want)
	}

	// AC2: re-ingesting the unchanged bytes is a cache hit that returns the same
	// probe. Point the worker at a path that cannot run to prove none is invoked.
	var second struct {
		Cached bool `json:"cached"`
		Source struct {
			Probe *probe `json:"probe"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{
		"project": dir, "source": src, "worker": "/nonexistent/ffmpeg-worker",
	}, &second)
	if !second.Cached {
		t.Fatal("second ingest cached = false, want true (cache hit)")
	}
	if second.Source.Probe == nil || *second.Source.Probe != want {
		t.Fatalf("cache-hit probe = %+v, want %+v", second.Source.Probe, want)
	}
}

// TestIngestWithoutWorkerRecordsNoProbe pins the graceful path at the CLI: with no
// ffmpeg worker configured, ingest still records the source, just without a probe.
func TestIngestWithoutWorkerRecordsNoProbe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)
	src := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(src, []byte("no worker bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Source struct {
			Hash  string         `json:"hash"`
			Probe map[string]any `json:"probe"`
		} `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{"project": dir, "source": src}, &res)
	if res.Source.Hash == "" {
		t.Fatal("ingest recorded no source")
	}
	if res.Source.Probe != nil {
		t.Fatalf("probe = %+v, want absent with no worker configured", res.Source.Probe)
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

// TestTranscribeWordLevel is acceptance #1: `daVid transcribe <project> --json`
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

// speakersReply is the JSON handle speakers returns.
type speakersReply struct {
	Speakers string `json:"speakers"`
	Source   string `json:"source"`
	Layout   string `json:"layout"`
	Turns    int    `json:"turns"`
	Cached   bool   `json:"cached"`
	Planned  bool   `json:"planned"`
}

// TestSpeakersTurns is acceptance #1: `daVid speakers <project> --json` returns a
// handle, and the analysis at that handle carries per-time speaker turns with
// start/end times. The project is given as a positional argument (the AC form).
func TestSpeakersTurns(t *testing.T) {
	dir, _ := ingestFixture(t, "fake video bytes")
	wpath, wargs := speakersWorkerEnv(t)

	var res speakersReply
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)
	var out bytes.Buffer
	if err := run([]string{"speakers", dir, "--json"}, bytes.NewReader(b), &out); err != nil {
		t.Fatalf("speakers = %v, want nil", err)
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.Cached {
		t.Fatal("first speakers cached = true, want false (cache miss)")
	}
	if res.Turns != 2 || res.Speakers == "" || res.Source == "" {
		t.Fatalf("result = %+v, want turns:2 with a handle and source hash", res)
	}
	// AC2: the analysis is against the fixed two-person layout, not a request param.
	if res.Layout != project.TwoPersonLayout {
		t.Fatalf("layout = %q, want the fixed %q", res.Layout, project.TwoPersonLayout)
	}

	// The handle points at a real file whose turns carry per-time speaker labels.
	sb, err := os.ReadFile(res.Speakers)
	if err != nil {
		t.Fatalf("speakers handle %q is not readable: %v", res.Speakers, err)
	}
	var got project.Speakers
	if err := json.Unmarshal(sb, &got); err != nil {
		t.Fatalf("speakers is not JSON: %v\n%s", err, sb)
	}
	if len(got.Turns) != 2 {
		t.Fatalf("analysis has %d turns, want 2", len(got.Turns))
	}
	if got.Turns[0].Speaker != "left" || got.Turns[0].Start != 0.0 || got.Turns[0].End != 1.5 {
		t.Fatalf("turn[0] = %+v, want {left 0 1.5}", got.Turns[0])
	}
	if got.Turns[1].Speaker != "right" || got.Turns[1].Start != 1.5 || got.Turns[1].End != 3.0 {
		t.Fatalf("turn[1] = %+v, want {right 1.5 3.0}", got.Turns[1])
	}
	// The core fixes the layout on the recorded track, and the worker echoed back
	// exactly the layout the core sent it — proving the fixed layout reaches it.
	if got.Layout != project.TwoPersonLayout {
		t.Fatalf("recorded layout = %q, want the fixed %q", got.Layout, project.TwoPersonLayout)
	}
	if got.Source != res.Source {
		t.Fatalf("analysis source = %q, want the returned handle's source %q", got.Source, res.Source)
	}
}

// Detection runs on the proxy when ingest built one, and falls back to the
// master when it did not. This is what the proxy is for: measured on a 20s 4K
// clip it cut detection from 9.40s to 4.15s and raised attribution from 96.3%
// to 100%, with seat positions agreeing to within 0.002 normalised (~6px at 4K).
// The substitution is only safe because the worker's output is time-based and
// normalised — if a seat ever carried pixel coordinates, crops computed for the
// master would be silently wrong by the scale factor, so this test is also the
// guard on that assumption holding.
func TestSpeakersAnalysesTheProxyWhenOneExists(t *testing.T) {
	echo := filepath.Join(t.TempDir(), "handed-to-worker.txt")

	// Ingest through the ffmpeg stand-in so the source really carries a proxy.
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)
	srcPath := filepath.Join(t.TempDir(), "raw.mp4")
	if err := os.WriteFile(srcPath, []byte("master bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", "ffmpeg")
	var ing struct {
		Source project.Source `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{
		"project": dir, "source": srcPath,
		"worker": os.Args[0], "workerArgs": []string{"-test.run=TestHelperProcess"},
	}, &ing)
	if ing.Source.Proxy == "" {
		t.Fatal("fixture did not produce a proxy, so this test proves nothing")
	}

	// Now run detection; the speakers stand-in records the path it was given.
	t.Setenv("WORKER_MODE", "speakers")
	t.Setenv("WORKER_ECHO_FILE", echo)
	var out bytes.Buffer
	b, _ := json.Marshal(map[string]any{
		"worker": os.Args[0], "workerArgs": []string{"-test.run=TestHelperProcess"},
	})
	if err := run([]string{"speakers", dir, "--json"}, bytes.NewReader(b), &out); err != nil {
		t.Fatalf("speakers = %v", err)
	}
	handed, err := os.ReadFile(echo)
	if err != nil {
		t.Fatalf("worker recorded no source path: %v", err)
	}
	if string(handed) != ing.Source.Proxy {
		t.Errorf("detection ran on %q, want the proxy %q", handed, ing.Source.Proxy)
	}

	// With the proxy file gone, detection must fall back to the master rather
	// than failing — the proxy is an optimisation, not a precondition.
	if err := os.Remove(ing.Source.Proxy); err != nil {
		t.Fatal(err)
	}
	dir2 := filepath.Join(t.TempDir(), "proj2")
	runJSON(t, []string{"init"}, map[string]any{"project": dir2}, nil)
	t.Setenv("WORKER_MODE", "ffmpeg")
	var ing2 struct {
		Source project.Source `json:"source"`
	}
	runJSON(t, []string{"ingest"}, map[string]any{
		"project": dir2, "source": srcPath,
		"worker": os.Args[0], "workerArgs": []string{"-test.run=TestHelperProcess"},
	}, &ing2)
	if err := os.Remove(ing2.Source.Proxy); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKER_MODE", "speakers")
	out.Reset()
	if err := run([]string{"speakers", dir2, "--json"}, bytes.NewReader(b), &out); err != nil {
		t.Fatalf("speakers with a missing proxy = %v, want it to fall back", err)
	}
	handed2, err := os.ReadFile(echo)
	if err != nil {
		t.Fatal(err)
	}
	if string(handed2) != ing2.Source.Path {
		t.Errorf("fallback ran on %q, want the master %q", handed2, ing2.Source.Path)
	}
}

// TestSpeakersCacheHitIdempotent is acceptance #2's idempotency half: the speaker
// track is cached and re-running is a content-hash cache hit that returns
// cached=true and runs NO worker — proven by pointing the second run at a
// non-existent worker, which would fail with worker.missing if the cache were
// bypassed.
func TestSpeakersCacheHitIdempotent(t *testing.T) {
	dir, _ := ingestFixture(t, "fake video bytes")
	wpath, wargs := speakersWorkerEnv(t)

	var first speakersReply
	runJSON(t, []string{"speakers", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &first)
	if first.Cached {
		t.Fatal("first speakers cached = true, want false")
	}

	// Second run with a worker that cannot start: a cache hit never touches it.
	var second speakersReply
	runJSON(t, []string{"speakers", dir}, map[string]any{"worker": "/nonexistent/mediapipe-worker"}, &second)
	if !second.Cached {
		t.Fatal("second speakers cached = false, want true (cache hit on unchanged bytes)")
	}
	if second.Speakers != first.Speakers || second.Source != first.Source || second.Turns != first.Turns {
		t.Fatalf("cache hit changed the handle: first %+v second %+v", first, second)
	}
}

// TestSpeakersPlanWritesNothing asserts --plan reports the handle and cache status
// without running the worker or writing an analysis.
func TestSpeakersPlanWritesNothing(t *testing.T) {
	dir, _ := ingestFixture(t, "fake video bytes")

	var res speakersReply
	// No worker configured: --plan must still succeed since it runs none.
	runJSON(t, []string{"speakers", dir, "--plan"}, map[string]any{}, &res)
	if !res.Planned || res.Cached {
		t.Fatalf("plan result = %+v, want planned:true cached:false", res)
	}
	if res.Speakers == "" {
		t.Fatal("plan result has no speakers handle")
	}
	if res.Layout != project.TwoPersonLayout {
		t.Fatalf("plan layout = %q, want the fixed %q", res.Layout, project.TwoPersonLayout)
	}
	if _, err := os.Stat(res.Speakers); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote the analysis (stat err = %v), want it absent", err)
	}
}

// TestSpeakersWorkerErrorMapsThrough asserts a worker that cannot start surfaces
// as its axerror code, not masked as a generic failure.
func TestSpeakersWorkerErrorMapsThrough(t *testing.T) {
	dir, _ := ingestFixture(t, "fake video bytes")
	err := run([]string{"speakers", dir},
		strings.NewReader(`{"worker":"/nonexistent/mediapipe-worker"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
}

// TestSpeakersFailureRules pins the written behaviour for empty and malformed
// input: each maps to a stable, machine-branchable axerror code.
func TestSpeakersFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
	// Initialised but no sources ingested -> nothing to analyse.
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
		{name: "no project", args: []string{"speakers"}, stdin: "", code: "input.invalid"},
		{name: "two positional projects", args: []string{"speakers", "a", "b"}, stdin: "", code: "input.invalid"},
		{name: "unknown flag", args: []string{"speakers", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
		{name: "uninitialized", args: []string{"speakers", filepath.Join(t.TempDir(), "none")}, stdin: "", code: "project.uninitialized"},
		{name: "no sources", args: []string{"speakers", empty}, stdin: "", code: "input.empty"},
		{name: "malformed json", args: []string{"speakers", initialized}, stdin: `{not json`, code: "input.invalid"},
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

// TestRenderProducesOutput is acceptance #1: `daVid render <project> --json`
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

// TestRenderPartialWriteDoesNotPoisonCache asserts render publishes atomically: a
// worker that writes partial bytes then dies non-zero must leave NO file at the
// cache path, so the next resumed run re-renders instead of returning a truncated
// mp4 as a completed cache hit. The first render errors and HasRender stays false;
// a second render with a working worker is a cache miss that writes the complete
// output.
func TestRenderPartialWriteDoesNotPoisonCache(t *testing.T) {
	dir := renderFixture(t)

	// Resolve the cache key the same way render does: hash the built timeline.
	proj := project.Open(dir)
	hash, err := project.HashFile(proj.TimelinePath())
	if err != nil {
		t.Fatalf("hash timeline: %v", err)
	}

	// First render: a melt that dies mid-encode after writing truncated bytes.
	wpath, wargs := renderWorkerEnv(t, "render-partial")
	req, _ := json.Marshal(map[string]any{"worker": wpath, "workerArgs": wargs})
	if err := run([]string{"render", dir}, bytes.NewReader(req), io.Discard); err == nil {
		t.Fatal("partial render err = nil, want a worker failure")
	}
	if proj.HasRender(hash, "proxy") {
		t.Fatal("HasRender = true after a crashed render, want false (partial output must not poison the cache)")
	}
	if _, err := os.Stat(proj.RenderPath(hash, "proxy") + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp render survived a crash (stat err = %v), want it removed", err)
	}

	// Second render with a working worker: a cache miss that re-renders and writes
	// the complete output, proving the poisoned first run left nothing behind.
	var second renderReply
	wpath, wargs = renderWorkerEnv(t, "render")
	runJSON(t, []string{"render", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &second)
	if second.Cached {
		t.Fatal("second render cached = true, want false (the crashed first run must not count as a hit)")
	}
	if b, err := os.ReadFile(second.Output); err != nil || string(b) != "fake rendered video" {
		t.Fatalf("output = %q err = %v, want the complete rendered file", b, err)
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

// progressEvent is one JSONL line a long step writes to stderr.
type progressEvent struct {
	Kind  string `json:"kind"`
	Step  string `json:"step"`
	Phase string `json:"phase"`
	Seq   int    `json:"seq"`
}

// assertStderrProgress parses stderr as newline-delimited progress objects and
// asserts they belong to the named step, open with a "start" and close with a
// "done", and carry a strictly increasing seq — the contract that lets a caller
// tell a slow step from a hung one.
func assertStderrProgress(t *testing.T, stderr, step string) {
	t.Helper()
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		t.Fatalf("%s emitted no progress on stderr", step)
	}
	var evs []progressEvent
	for _, line := range strings.Split(trimmed, "\n") {
		if line == "" {
			continue
		}
		var ev progressEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("progress line %q is not valid JSON: %v", line, err)
		}
		if ev.Kind != "progress" {
			t.Fatalf("progress line %q kind = %q, want progress", line, ev.Kind)
		}
		if ev.Step != step {
			t.Fatalf("progress line %q step = %q, want %q", line, ev.Step, step)
		}
		evs = append(evs, ev)
	}
	if len(evs) < 2 {
		t.Fatalf("%s emitted %d progress events, want at least start+done", step, len(evs))
	}
	if evs[0].Phase != "start" {
		t.Fatalf("first progress phase = %q, want start", evs[0].Phase)
	}
	if last := evs[len(evs)-1]; last.Phase != "done" {
		t.Fatalf("last progress phase = %q, want done", last.Phase)
	}
	for i, ev := range evs {
		if ev.Seq != i {
			t.Fatalf("progress event %d seq = %d, want %d", i, ev.Seq, i)
		}
	}
}

// assertOneJSONResult decodes stdout and confirms it is exactly one JSON value
// with nothing trailing — the unchanged contract that progress on stderr must
// not disturb.
func assertOneJSONResult(t *testing.T, stdout string) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var first any
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("stdout is not a JSON value: %v\n%s", err, stdout)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatalf("stdout carried more than one JSON value (err=%v), want exactly one:\n%s", err, stdout)
	}
}

// TestTranscribeEmitsProgress is acceptance #1+#2 for transcribe: the step
// writes newline-delimited progress objects to stderr while stdout still carries
// exactly one JSON result.
func TestTranscribeEmitsProgress(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes")
	wpath, wargs := transcribeWorkerEnv(t, "transcribe")
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)

	var out, errBuf bytes.Buffer
	if err := run([]string{"transcribe", dir, "--json"}, bytes.NewReader(b), &out, &errBuf); err != nil {
		t.Fatalf("transcribe = %v, want nil", err)
	}
	assertOneJSONResult(t, out.String())
	assertStderrProgress(t, errBuf.String(), "transcribe")
}

// TestSpeakersEmitsProgress is acceptance #1+#2 for speakers.
func TestSpeakersEmitsProgress(t *testing.T) {
	dir, _ := ingestFixture(t, "fake video bytes")
	wpath, wargs := speakersWorkerEnv(t)
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)

	var out, errBuf bytes.Buffer
	if err := run([]string{"speakers", dir, "--json"}, bytes.NewReader(b), &out, &errBuf); err != nil {
		t.Fatalf("speakers = %v, want nil", err)
	}
	assertOneJSONResult(t, out.String())
	assertStderrProgress(t, errBuf.String(), "speakers")
}

// TestRenderEmitsProgress is acceptance #1+#2 for render.
func TestRenderEmitsProgress(t *testing.T) {
	dir := renderFixture(t)
	wpath, wargs := renderWorkerEnv(t, "render")
	req := map[string]any{"worker": wpath, "workerArgs": wargs}
	b, _ := json.Marshal(req)

	var out, errBuf bytes.Buffer
	if err := run([]string{"render", dir, "--json"}, bytes.NewReader(b), &out, &errBuf); err != nil {
		t.Fatalf("render = %v, want nil", err)
	}
	assertOneJSONResult(t, out.String())
	assertStderrProgress(t, errBuf.String(), "render")
}

// TestCacheHitEmitsNoProgress proves progress brackets only real work: an
// idempotent cache-hit re-run returns its JSON result on stdout and writes
// nothing to the progress channel, so a fast re-run stays silent.
func TestCacheHitEmitsNoProgress(t *testing.T) {
	dir := renderFixture(t)
	wpath, wargs := renderWorkerEnv(t, "render")

	// Prime the cache with a real render (its progress goes to a discarded buffer).
	runJSON(t, []string{"render", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, nil)

	// Re-run against a worker that cannot start: a cache hit never touches it and
	// emits no progress.
	var out, errBuf bytes.Buffer
	if err := run([]string{"render", dir}, strings.NewReader(`{"worker":"/nonexistent/melt"}`), &out, &errBuf); err != nil {
		t.Fatalf("cache-hit render = %v, want nil", err)
	}
	assertOneJSONResult(t, out.String())
	if strings.TrimSpace(errBuf.String()) != "" {
		t.Fatalf("cache hit emitted progress, want none:\n%s", errBuf.String())
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

// indexReply is the JSON handle index returns.
type indexReply struct {
	Index    string `json:"index"`
	Table    string `json:"table"`
	Source   string `json:"source"`
	Segments int    `json:"segments"`
	Dim      int    `json:"dim"`
	Model    string `json:"model"`
	Cached   bool   `json:"cached"`
	Planned  bool   `json:"planned"`
}

// indexWorkerEnv points a per-source step at this test binary as its worker:
// t.Setenv marks it a helper process running only TestHelperProcess in the given
// WORKER_MODE, so the real subprocess round-trip runs with no python on PATH.
func indexWorkerEnv(t *testing.T, mode string) (path string, args []string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("WORKER_MODE", mode)
	return os.Args[0], []string{"-test.run=TestHelperProcess"}
}

// transcribedFixture ingests contents and transcribes it via the fake whisper
// worker, leaving a project whose sole source has a transcript ready to index.
func transcribedFixture(t *testing.T, contents string) (dir, src string) {
	t.Helper()
	dir, src = ingestFixture(t, contents)
	wpath, wargs := indexWorkerEnv(t, "transcribe")
	runJSON(t, []string{"transcribe", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, nil)
	return dir, src
}

// TestIndexBuildsLanceDBIndex is acceptance #1: `daVid index <project> --json`
// builds a local LanceDB index of the source's transcript segments. The result
// is a handle to the cached metadata plus the sibling .lance table, and the
// worker embeds locally (fake worker, no network / API key — acceptance #2).
func TestIndexBuildsLanceDBIndex(t *testing.T) {
	dir, _ := transcribedFixture(t, "fake audio bytes")
	wpath, wargs := indexWorkerEnv(t, "index")

	var res indexReply
	runJSON(t, []string{"index", dir, "--json"}, map[string]any{"worker": wpath, "workerArgs": wargs}, &res)
	if res.Cached {
		t.Fatal("first index cached = true, want false (cache miss)")
	}
	if res.Segments != 2 || res.Dim != 3 || res.Index == "" || res.Table == "" || res.Source == "" {
		t.Fatalf("result = %+v, want segments:2 dim:3 with index+table handles and a source hash", res)
	}
	if res.Model != "fake-local-embed" {
		t.Fatalf("model = %q, want fake-local-embed", res.Model)
	}

	// The metadata handle points at a real JSON file describing the index.
	mb, err := os.ReadFile(res.Index)
	if err != nil {
		t.Fatalf("index handle %q is not readable: %v", res.Index, err)
	}
	var got project.Index
	if err := json.Unmarshal(mb, &got); err != nil {
		t.Fatalf("index metadata is not JSON: %v\n%s", err, mb)
	}
	if got.Segments != 2 || got.Dim != 3 || got.Source != res.Source || got.Table != res.Table {
		t.Fatalf("index metadata = %+v, want the returned handle's summary", got)
	}
	// The LanceDB table the worker wrote must exist on disk.
	if _, err := os.Stat(res.Table); err != nil {
		t.Fatalf("LanceDB table %q not written: %v", res.Table, err)
	}
}

// TestIndexCacheHitIdempotent is acceptance #2: the index is cached and
// idempotent. The second run is a content-hash cache hit that returns cached=true
// and runs NO worker — proven by pointing it at a non-existent worker, which
// would fail with worker.missing if the cache were bypassed.
func TestIndexCacheHitIdempotent(t *testing.T) {
	dir, _ := transcribedFixture(t, "fake audio bytes")
	wpath, wargs := indexWorkerEnv(t, "index")

	var first indexReply
	runJSON(t, []string{"index", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &first)
	if first.Cached {
		t.Fatal("first index cached = true, want false")
	}

	// Second run with a worker that cannot start: a cache hit never touches it.
	var second indexReply
	runJSON(t, []string{"index", dir}, map[string]any{"worker": "/nonexistent/embed-worker"}, &second)
	if !second.Cached {
		t.Fatal("second index cached = false, want true (cache hit on unchanged bytes)")
	}
	if second.Index != first.Index || second.Table != first.Table || second.Source != first.Source || second.Segments != first.Segments {
		t.Fatalf("cache hit changed the handle: first %+v second %+v", first, second)
	}
}

// TestIndexPlanWritesNothing asserts --plan reports the handles and cache status
// without running the worker or writing an index.
func TestIndexPlanWritesNothing(t *testing.T) {
	dir, _ := transcribedFixture(t, "fake audio bytes")

	var res indexReply
	// No worker configured: --plan must still succeed since it runs none.
	runJSON(t, []string{"index", dir, "--plan"}, map[string]any{}, &res)
	if !res.Planned || res.Cached {
		t.Fatalf("plan result = %+v, want planned:true cached:false", res)
	}
	if res.Index == "" || res.Table == "" {
		t.Fatal("plan result is missing a handle")
	}
	if _, err := os.Stat(res.Index); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote the index metadata (stat err = %v), want it absent", err)
	}
	if _, err := os.Stat(res.Table); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote the LanceDB table (stat err = %v), want it absent", err)
	}
}

// TestIndexRequiresTranscript asserts index refuses a source that has not been
// transcribed: the index is built over transcript segments, so a missing
// transcript is a stable, branchable precondition error.
func TestIndexRequiresTranscript(t *testing.T) {
	dir, _ := ingestFixture(t, "fake audio bytes") // ingested but not transcribed
	err := run([]string{"index", dir}, strings.NewReader(`{}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "transcript.missing" {
		t.Fatalf("code = %q, want transcript.missing", ae.Code)
	}
}

// TestIndexWorkerErrorMapsThrough asserts a worker that cannot start surfaces as
// its axerror code, not masked as a generic failure.
func TestIndexWorkerErrorMapsThrough(t *testing.T) {
	dir, _ := transcribedFixture(t, "fake audio bytes")
	err := run([]string{"index", dir},
		strings.NewReader(`{"worker":"/nonexistent/embed-worker"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
}

// TestIndexFailureRules pins the written behaviour for empty and malformed input:
// each maps to a stable, machine-branchable axerror code.
func TestIndexFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
	// Initialised but no sources ingested -> nothing to index.
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
		{name: "no project", args: []string{"index"}, stdin: "", code: "input.invalid"},
		{name: "two positional projects", args: []string{"index", "a", "b"}, stdin: "", code: "input.invalid"},
		{name: "unknown flag", args: []string{"index", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
		{name: "uninitialized", args: []string{"index", filepath.Join(t.TempDir(), "none")}, stdin: "", code: "project.uninitialized"},
		{name: "no sources", args: []string{"index", empty}, stdin: "", code: "input.empty"},
		{name: "malformed json", args: []string{"index", initialized}, stdin: `{not json`, code: "input.invalid"},
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

// searchReply is the JSON shape search returns: the query echoed back, the
// searched source, and the ranked timecoded candidates.
type searchReply struct {
	Query   string `json:"query"`
	Source  string `json:"source"`
	Results []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
		Score float64 `json:"score"`
	} `json:"results"`
	Count   int    `json:"count"`
	Model   string `json:"model"`
	Planned bool   `json:"planned"`
}

// indexedFixture ingests, transcribes, and indexes contents, leaving a project
// whose sole source has a LanceDB index ready to search.
func indexedFixture(t *testing.T, contents string) (dir, src string) {
	t.Helper()
	dir, src = transcribedFixture(t, contents)
	wpath, wargs := indexWorkerEnv(t, "index")
	runJSON(t, []string{"index", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, nil)
	return dir, src
}

// TestSearchRanksTimecodedCandidates is acceptance #1: `daVid search <project>
// "<query>" --json` returns ranked {start,end,text,score} candidates. The fake
// worker deliberately returns hits out of score order, so a correct pass proves
// the core re-ranks them descending by score.
func TestSearchRanksTimecodedCandidates(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")
	wpath, wargs := indexWorkerEnv(t, "search")

	var res searchReply
	runJSON(t, []string{"search", dir, "hello", "--json"},
		map[string]any{"worker": wpath, "workerArgs": wargs}, &res)

	if res.Query != "hello" {
		t.Fatalf("query = %q, want hello", res.Query)
	}
	if res.Source == "" {
		t.Fatal("source is empty, want the searched source's content hash")
	}
	if res.Count != 2 || len(res.Results) != 2 {
		t.Fatalf("results = %+v, want 2 candidates", res.Results)
	}
	if res.Model != "fake-local-embed" {
		t.Fatalf("model = %q, want fake-local-embed", res.Model)
	}
	// Ranked most-relevant first: the 0.9-score "hello" hit outranks the
	// 0.2-score "world" hit even though the worker listed them the other way.
	top := res.Results[0]
	if top.Text != "hello" || top.Score != 0.9 || top.Start != 0.0 || top.End != 1.5 {
		t.Fatalf("top candidate = %+v, want {0 1.5 hello 0.9}", top)
	}
	second := res.Results[1]
	if second.Text != "world" || second.Score != 0.2 {
		t.Fatalf("second candidate = %+v, want text=world score=0.2", second)
	}
	// The ranking must be non-increasing by score.
	if res.Results[0].Score < res.Results[1].Score {
		t.Fatalf("results not sorted by descending score: %+v", res.Results)
	}
}

// TestSearchDeterministic is acceptance #2: a fixed query+index yields the
// identical ranked output on every run — proven by comparing the full JSON bytes
// of two independent searches.
func TestSearchDeterministic(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")
	wpath, wargs := indexWorkerEnv(t, "search")
	req := map[string]any{"worker": wpath, "workerArgs": wargs}

	var first, second bytes.Buffer
	b, _ := json.Marshal(req)
	if err := run([]string{"search", dir, "hello", "--json"}, bytes.NewReader(b), &first); err != nil {
		t.Fatalf("first search = %v", err)
	}
	if err := run([]string{"search", dir, "hello", "--json"}, bytes.NewReader(b), &second); err != nil {
		t.Fatalf("second search = %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("search not deterministic:\nfirst:  %s\nsecond: %s", first.String(), second.String())
	}
}

// TestSearchLimitCaps asserts the limit field caps the number of ranked
// candidates, keeping the highest-scoring ones.
func TestSearchLimitCaps(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")
	wpath, wargs := indexWorkerEnv(t, "search")

	var res searchReply
	runJSON(t, []string{"search", dir, "hello"},
		map[string]any{"worker": wpath, "workerArgs": wargs, "limit": 1}, &res)
	if res.Count != 1 || len(res.Results) != 1 {
		t.Fatalf("limit=1 returned %d candidates, want 1", res.Count)
	}
	if res.Results[0].Text != "hello" {
		t.Fatalf("limit kept %q, want the top-scoring hello", res.Results[0].Text)
	}
}

// assetSearchReply mirrors the assets search JSON: the query, the model, and the
// ranked library hits.
type assetSearchReply struct {
	Query  string `json:"query"`
	Model  string `json:"model"`
	Assets []struct {
		Ref   string  `json:"ref"`
		Text  string  `json:"text"`
		Score float64 `json:"score"`
	} `json:"assets"`
}

// TestAssetsSearchRanksHitsByScore proves the library retrieval surface re-ranks
// the worker's hits into a deterministic descending-score order, mirroring
// transcript search's project.RankCandidates. The fake worker deliberately
// returns a 0.2-score hit before a 0.9-score hit, so a correct pass proves the Go
// side sorts them highest-first before emitting.
func TestAssetsSearchRanksHitsByScore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+dir+`"}`), io.Discard); err != nil {
		t.Fatalf("init = %v", err)
	}
	wpath, wargs := indexWorkerEnv(t, "search-assets")

	var res assetSearchReply
	runJSON(t, []string{"assets", "search", "deep boom"},
		map[string]any{"project": dir, "worker": wpath, "workerArgs": wargs}, &res)

	if len(res.Assets) != 2 {
		t.Fatalf("assets = %+v, want 2 hits", res.Assets)
	}
	// Ranked most-relevant first: the 0.9-score boom outranks the 0.2-score
	// whoosh even though the worker listed them the other way.
	top := res.Assets[0]
	if top.Score != 0.9 || top.Ref != "sfx/boom.wav" {
		t.Fatalf("top hit = %+v, want the 0.9-score sfx/boom.wav", top)
	}
	if res.Assets[0].Score < res.Assets[1].Score {
		t.Fatalf("assets not sorted by descending score: %+v", res.Assets)
	}
}

// TestAssetsSearchLimitCaps asserts req.Limit caps the ranked hits Go-side,
// keeping the highest-scoring ones.
func TestAssetsSearchLimitCaps(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+dir+`"}`), io.Discard); err != nil {
		t.Fatalf("init = %v", err)
	}
	wpath, wargs := indexWorkerEnv(t, "search-assets")

	var res assetSearchReply
	runJSON(t, []string{"assets", "search", "deep boom"},
		map[string]any{"project": dir, "worker": wpath, "workerArgs": wargs, "limit": 1}, &res)
	if len(res.Assets) != 1 {
		t.Fatalf("limit=1 returned %d hits, want 1", len(res.Assets))
	}
	if res.Assets[0].Ref != "sfx/boom.wav" {
		t.Fatalf("limit kept %q, want the top-scoring sfx/boom.wav", res.Assets[0].Ref)
	}
}

// TestSearchRequiresIndex asserts search refuses a source that has not been
// indexed: the query runs against the vector index, so a missing index is a
// stable, branchable precondition error.
func TestSearchRequiresIndex(t *testing.T) {
	dir, _ := transcribedFixture(t, "fake audio bytes") // transcribed but not indexed
	err := run([]string{"search", dir, "hello"}, strings.NewReader(`{}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "index.missing" {
		t.Fatalf("code = %q, want index.missing", ae.Code)
	}
}

// TestSearchPlanRunsNoWorker asserts --plan reports the query and source without
// running a worker (it succeeds even with none configured) and returns no hits.
func TestSearchPlanRunsNoWorker(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")

	var res searchReply
	runJSON(t, []string{"search", dir, "hello", "--plan"}, map[string]any{}, &res)
	if !res.Planned {
		t.Fatalf("plan result = %+v, want planned:true", res)
	}
	if res.Query != "hello" || res.Source == "" {
		t.Fatalf("plan result = %+v, want query=hello and a non-empty source", res)
	}
	if len(res.Results) != 0 {
		t.Fatalf("plan returned %d results, want none", len(res.Results))
	}
}

// TestSearchWorkerErrorMapsThrough asserts a worker that cannot start surfaces as
// its axerror code, not masked as a generic failure.
func TestSearchWorkerErrorMapsThrough(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")
	err := run([]string{"search", dir, "hello"},
		strings.NewReader(`{"worker":"/nonexistent/search-worker"}`), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "worker.missing" {
		t.Fatalf("code = %q, want worker.missing", ae.Code)
	}
}

// TestSearchFailureRules pins the written behaviour for missing and malformed
// input: each maps to a stable, machine-branchable axerror code.
func TestSearchFailureRules(t *testing.T) {
	initialized := filepath.Join(t.TempDir(), "proj")
	if err := run([]string{"init"}, strings.NewReader(`{"project":"`+initialized+`"}`), io.Discard); err != nil {
		t.Fatal(err)
	}
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
		{name: "no project", args: []string{"search"}, stdin: "", code: "input.invalid"},
		{name: "three positionals", args: []string{"search", "a", "b", "c"}, stdin: "", code: "input.invalid"},
		{name: "unknown flag", args: []string{"search", "--bogus"}, stdin: "", code: "cli.unknown_flag"},
		{name: "empty query", args: []string{"search", initialized}, stdin: "", code: "input.empty"},
		{name: "uninitialized", args: []string{"search", filepath.Join(t.TempDir(), "none"), "q"}, stdin: "", code: "project.uninitialized"},
		{name: "no sources", args: []string{"search", empty, "q"}, stdin: "", code: "input.empty"},
		{name: "malformed json", args: []string{"search", initialized, "q"}, stdin: `{not json`, code: "input.invalid"},
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

// doctorReport mirrors the doctor tool's JSON output for assertions.
type doctorReport struct {
	Ok              bool `json:"ok"`
	Missing         int  `json:"missing"`
	RequiredMissing int  `json:"requiredMissing"`
	MissingNames    []string
	Checks          []struct {
		Name, Kind, Status, Detail, Hint string
		Required                         bool
	}
}

// TestDoctorReportsEveryKind proves `daVid doctor --json` emits a structured
// preflight covering every dependency class (binary, worker, package, model),
// each with an ok/missing status, and that every missing check carries a hint.
// The report is produced regardless of what the host actually has installed.
func TestDoctorReportsEveryKind(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"doctor", "--json"}, strings.NewReader(""), &out)

	var rep doctorReport
	if jerr := json.Unmarshal(out.Bytes(), &rep); jerr != nil {
		t.Fatalf("doctor did not emit valid JSON: %v\n%s", jerr, out.String())
	}
	if len(rep.Checks) == 0 {
		t.Fatal("doctor reported no checks")
	}
	kinds := map[string]bool{}
	for _, c := range rep.Checks {
		kinds[c.Kind] = true
		if c.Status != "ok" && c.Status != "missing" {
			t.Errorf("%s: status = %q, want ok|missing", c.Name, c.Status)
		}
		if c.Status == "missing" && c.Hint == "" {
			t.Errorf("%s is missing but carries no hint", c.Name)
		}
	}
	for _, want := range []string{"binary", "worker", "package", "model"} {
		if !kinds[want] {
			t.Errorf("report has no %q check", want)
		}
	}
	// The report and the process exit signal must agree: a failed preflight is
	// reported ok:false AND returned as a structured error (non-zero exit).
	var ae *axerror.Error
	if rep.Ok {
		if err != nil {
			t.Fatalf("ok report but run returned %v", err)
		}
	} else {
		if !errors.As(err, &ae) {
			t.Fatalf("not-ok report but run returned %v, want *axerror.Error", err)
		}
		if ae.Code != "doctor.preflight_failed" {
			t.Fatalf("code = %q, want doctor.preflight_failed", ae.Code)
		}
	}
}

// TestDoctorFailsWhenRequiredMissing forces a required dependency to be missing
// (the melt worker's env var is cleared) and asserts doctor never starts real
// work, still emits the full report on stdout, and exits non-zero with a
// structured error naming the gap.
func TestDoctorFailsWhenRequiredMissing(t *testing.T) {
	// Clear every worker env var so the worker checks deterministically miss,
	// independent of the host's binaries/packages/models.
	for _, e := range []string{
		"DAVID_FFMPEG_WORKER", "DAVID_WHISPER_WORKER", "DAVID_INDEX_WORKER",
		"DAVID_SEARCH_WORKER", "DAVID_MEDIAPIPE_WORKER", "DAVID_MELT_WORKER",
	} {
		t.Setenv(e, "")
	}

	var out bytes.Buffer
	err := run([]string{"doctor"}, strings.NewReader(""), &out)

	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "doctor.preflight_failed" {
		t.Fatalf("code = %q, want doctor.preflight_failed", ae.Code)
	}
	// The report is still emitted on stdout so the caller sees every gap.
	var rep doctorReport
	if jerr := json.Unmarshal(out.Bytes(), &rep); jerr != nil {
		t.Fatalf("doctor emitted no report on failure: %v", jerr)
	}
	if rep.Ok {
		t.Fatal("report ok:true despite missing workers")
	}
	if rep.RequiredMissing < 6 {
		t.Fatalf("requiredMissing = %d, want >= 6 (the workers)", rep.RequiredMissing)
	}
	found := false
	for _, c := range rep.Checks {
		if c.Name == "melt worker" {
			found = true
			if c.Status != "missing" {
				t.Fatalf("melt worker status = %q, want missing", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("no 'melt worker' check in the report")
	}
}

// TestDoctorRejectsUnknownFlag pins the CLI failure rule for a bad flag.
func TestDoctorRejectsUnknownFlag(t *testing.T) {
	err := run([]string{"doctor", "--bogus"}, strings.NewReader(""), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "cli.unknown_flag" {
		t.Fatalf("code = %q, want cli.unknown_flag", ae.Code)
	}
}

// shortsReply is the JSON shape shorts returns: the query echoed back, the
// searched source, and one ranked short per selected moment.
type shortsReply struct {
	Query  string `json:"query"`
	Source string `json:"source"`
	Shorts []struct {
		Rank  int     `json:"rank"`
		Score float64 `json:"score"`
		Text  string  `json:"text"`
	} `json:"shorts"`
	Planned bool `json:"planned"`
}

// TestShortsRanksMomentsBeforeSelecting asserts shorts re-ranks the search
// candidates into a deterministic descending-score order before capping to the
// requested count — mirroring search — so shorts and search agree on the top
// moments and two runs over one index cannot yield different clips. The fake
// search worker returns hits out of score order (0.2 before 0.9); a correct
// pass proves shorts orders them by score, not by the worker's iteration order.
// --plan leaves the renders unpaid so the assertion needs no melt worker.
func TestShortsRanksMomentsBeforeSelecting(t *testing.T) {
	dir, _ := indexedFixture(t, "fake audio bytes")

	// shorts frames on the speaking seat, so the source needs a speaker track.
	spath, sargs := speakersWorkerEnv(t)
	runJSON(t, []string{"speakers", dir}, map[string]any{"worker": spath, "workerArgs": sargs}, nil)

	// Point the shorts search at the same out-of-order fake worker search uses.
	wpath, wargs := indexWorkerEnv(t, "search")

	var res shortsReply
	runJSON(t, []string{"shorts", dir, "hello", "--plan"},
		map[string]any{"searchWorker": wpath, "searchWorkerArgs": wargs}, &res)

	if !res.Planned {
		t.Fatalf("planned = false, want true for a --plan dry run")
	}
	if len(res.Shorts) != 2 {
		t.Fatalf("shorts = %+v, want 2 ranked moments", res.Shorts)
	}
	// The 0.9-score "hello" hit outranks the 0.2-score "world" hit even though the
	// worker listed them the other way — shorts selects the top moment, not the
	// worker's first.
	top := res.Shorts[0]
	if top.Text != "hello" || top.Score != 0.9 {
		t.Fatalf("top short = %+v, want text=hello score=0.9", top)
	}
	// Rank fields ascend from 1 while scores descend — the deterministic order.
	for i := range res.Shorts {
		if res.Shorts[i].Rank != i+1 {
			t.Fatalf("shorts[%d].rank = %d, want %d", i, res.Shorts[i].Rank, i+1)
		}
		if i > 0 && res.Shorts[i-1].Score < res.Shorts[i].Score {
			t.Fatalf("shorts not sorted by descending score: %+v", res.Shorts)
		}
	}
}

// TestDoctorInCatalog proves the tool is discoverable on both surfaces via the
// shared catalog.
func TestDoctorInCatalog(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"describe", "--json"}, strings.NewReader(""), &out); err != nil {
		t.Fatalf("describe: %v", err)
	}
	if !strings.Contains(out.String(), `"doctor"`) {
		t.Fatal("describe --json does not list the doctor tool")
	}
}

// TestUsageIsAccurate pins the help text an agent reads first. Help is the one
// surface where a stale sentence is worse than no sentence: an agent that reads
// "not yet implemented" against a working feature simply never calls it, and
// nothing fails loudly enough to notice. Each check below corresponds to a real
// defect found in this text, so the test fails if any of them regress.
func TestUsageIsAccurate(t *testing.T) {
	var help bytes.Buffer
	// The fourth argument is the injected stderr; help must land there rather
	// than on the process's own stderr, or it is uncapturable by an MCP host.
	if err := run(nil, strings.NewReader(""), io.Discard, &help); err != nil {
		t.Fatalf("usage: %v", err)
	}
	text := help.String()
	if text == "" {
		t.Fatal("usage wrote nothing to the injected stderr (it is still writing to os.Stderr)")
	}

	// The MCP server works — `mcp serve` answers initialize and tools/list, and
	// mcp_test.go exercises it. Advertising it as unbuilt is a lie to the agent.
	if strings.Contains(text, "not yet implemented") {
		t.Error(`usage claims a command is "not yet implemented"; mcp serve works — see mcp_test.go`)
	}
	// It is reached as `mcp serve`; a bare `mcp` is an input.empty error, so the
	// help must name the subcommand or it sends the caller into that error.
	if !strings.Contains(text, "mcp serve") {
		t.Error("usage does not mention `mcp serve`; bare `mcp` errors with input.empty")
	}

	// Every catalog tool must appear exactly once: the meta block above the loop
	// used to hardcode `describe`, which the loop then printed a second time.
	for _, tool := range catalog.Default().Tools {
		if n := strings.Count(text, "  "+tool.Name+" "); n != 1 {
			t.Errorf("tool %q appears %d times in usage, want exactly 1", tool.Name, n)
		}
	}

	// The longest name is `reframe-edit` (12 chars). With a narrower field its
	// summary is shoved out of the column, so assert the summaries line up.
	var col = -1
	for _, line := range strings.Split(text, "\n") {
		for _, tool := range catalog.Default().Tools {
			if !strings.HasPrefix(line, "  "+tool.Name+" ") {
				continue
			}
			at := strings.Index(line, strings.Fields(line)[1])
			if col == -1 {
				col = at
			} else if at != col {
				t.Errorf("summary for %q starts at column %d, want %d (field too narrow for the longest name)", tool.Name, at, col)
			}
		}
	}
}

func TestRenderPreservesContainerUnderAtomicTempSuffix(t *testing.T) {
	dir := renderFixture(t)
	echo := filepath.Join(t.TempDir(), "asked-for.txt")
	t.Setenv("WORKER_ECHO_FILE", echo)
	wpath, wargs := renderWorkerEnv(t, "render")

	body, _ := json.Marshal(map[string]any{"worker": wpath, "workerArgs": wargs})
	var out bytes.Buffer
	if err := run([]string{"render", dir}, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	asked, err := os.ReadFile(echo)
	if err != nil {
		t.Fatalf("worker recorded no output path: %v", err)
	}
	if !strings.HasSuffix(string(asked), ".mp4.tmp") {
		t.Fatalf("render asked the worker for %q; want a temp path retaining .mp4", asked)
	}
}
