// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
	"github.com/tahabsn/cutvec/internal/doctor"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
	"github.com/tahabsn/cutvec/internal/worker"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		emitError(err)
		os.Exit(1)
	}
}

// run dispatches a subcommand. Tools read their JSON request from stdin and
// write their JSON result to stdout; both streams are passed in so the dispatch
// is testable without touching the process's real file descriptors.
func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "describe":
		return describeCmd(args[1:], stdout)
	case "schema":
		return schemaCmd(args[1:], stdout)
	case "init":
		return initCmd(args[1:], stdin, stdout)
	case "ingest":
		return ingestCmd(args[1:], stdin, stdout)
	case "status":
		return statusCmd(args[1:], stdin, stdout)
	case "transcribe":
		return transcribeCmd(args[1:], stdin, stdout)
	case "index":
		return indexCmd(args[1:], stdin, stdout)
	case "search":
		return searchCmd(args[1:], stdin, stdout)
	case "speakers":
		return speakersCmd(args[1:], stdin, stdout)
	case "shorts":
		return shortsCmd(args[1:], stdin, stdout)
	case "reframe-edit":
		return reframeCmd(args[1:], stdin, stdout)
	case "assets":
		return assetsCmd(args[1:], stdin, stdout)
	case "filters":
		return filtersCmd(args[1:], stdin, stdout)
	case "timeline":
		return timelineCmd(args[1:], stdin, stdout)
	case "render":
		return renderCmd(args[1:], stdin, stdout)
	case "doctor":
		return doctorCmd(args[1:], stdin, stdout)
	case "mcp":
		return mcpCmd(args[1:], stdin, stdout)
	case "-h", "--help", "help":
		return usage()
	default:
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("unknown command %q", args[0])).
			WithHint("run `cutvec describe` to list available tools")
	}
}

// describeCmd emits the tool catalog so an agent can discover every command and
// its typed IO. `describe` returns the full catalog object ({version,tools});
// `describe --json` returns just the tools array — each tool carrying its name,
// summary, input schema, and output schema — which is the shape an agent maps
// straight onto its tool list.
func describeCmd(args []string, stdout io.Writer) error {
	c := catalog.Default()
	for _, a := range args {
		switch a {
		case "--json":
			return emitJSON(stdout, c.Tools)
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --json (emit the tools array)")
		}
	}
	return emitJSON(stdout, c)
}

// schemaCmd emits the JSON Schema for a single command: `cutvec schema <cmd>`
// returns a draft 2020-12 schema whose `input` and `output` describe that tool's
// typed IO. An unknown command is a stable, branchable error listing the tools
// the caller can ask about.
func schemaCmd(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return axerror.New("input.empty", "no command named").
			WithHint("run `cutvec schema <cmd>`, e.g. `cutvec schema timeline`")
	}
	if len(args) > 1 {
		return axerror.New("input.invalid",
			fmt.Sprintf("schema takes one command, got %d args", len(args))).
			WithHint("name exactly one command, e.g. `cutvec schema timeline`")
	}
	c := catalog.Default()
	tool, ok := c.Lookup(args[0])
	if !ok {
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("no command named %q", args[0])).
			WithHint("known commands: " + strings.Join(c.Names(), ", "))
	}
	return emitJSON(stdout, tool.JSONSchema())
}

// timelineRequest is the JSON document a caller writes to stdin for the
// `timeline` tool: the project to write into and the clips to place on it.
type timelineRequest struct {
	Project string         `json:"project"`
	Clips   []timelineClip `json:"clips"`
	// Effects attach MLT filters to the timeline: either named presets or any
	// MLT service by name. `cutvec filters` lists what this machine's MLT has.
	Effects []effectSpec `json:"effects"`
	// Audio lays sounds under the edit as mixed MLT tracks.
	Audio []audioSpec `json:"audio"`
}

// timelineClip is one media source on the timeline. Resource is required; ID
// defaults to producerN when omitted.
type timelineClip struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
}

// timelineResult is the handle returned on success: the path to the written MLT
// document plus a small summary — a handle, never the XML blob itself.
type timelineResult struct {
	Timeline  string `json:"timeline"`
	Producers int    `json:"producers"`
	Planned   bool   `json:"planned,omitempty"`
}

// timelineCmd carries one request end-to-end: it reads a JSON clip list from
// stdin, builds the MLT XML timeline (the agent-editable artifact), writes it
// into the project directory, and returns a path handle as JSON on stdout. With
// --plan it reports what it would write without touching the filesystem.
func timelineCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	plan := false
	for _, a := range args {
		switch a {
		case "--plan":
			plan = true
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --plan (dry-run)")
		}
	}

	var req timelineRequest
	if err := json.NewDecoder(stdin).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return axerror.New("input.empty", "no JSON request on stdin").
				WithHint(`pipe a request like {"project":"proj","clips":[{"resource":"raw.mp4"}]}`)
		}
		return axerror.New("input.invalid", "request is not valid JSON: "+err.Error()).
			WithHint("stdin must be a single JSON object")
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`set "project" to the project directory path`)
	}
	if len(req.Clips) == 0 {
		return axerror.New("input.empty", "no clips provided").
			WithHint(`provide at least one clip in "clips"`)
	}

	tl := timeline.New()
	for i, c := range req.Clips {
		if c.Resource == "" {
			return axerror.New("input.invalid",
				fmt.Sprintf("clip %d has an empty resource", i)).
				WithHint("every clip needs a resource path")
		}
		id := c.ID
		if id == "" {
			id = fmt.Sprintf("producer%d", i)
		}
		tl.Producers = append(tl.Producers, timeline.Producer{ID: id, Resource: c.Resource})
	}

	// Effects are resolved before anything is written, so an unknown preset or a
	// typo'd producer id fails without leaving a half-built timeline behind.
	byProducer, err := resolveEffects(req.Effects)
	if err != nil {
		return err
	}
	if err := applyEffects(tl, byProducer); err != nil {
		return err
	}
	// Audio comes after effects so a bed is not caught by an effect meant for
	// the picture, and it builds the track structure the mix needs.
	if err := applyAudio(tl, req.Audio); err != nil {
		return err
	}

	proj := project.Open(req.Project)
	result := timelineResult{Timeline: proj.TimelinePath(), Producers: len(tl.Producers)}
	if plan {
		result.Planned = true
		return emitJSON(stdout, result)
	}

	xmlBytes, err := tl.Marshal()
	if err != nil {
		return axerror.New("timeline.marshal", "failed to render MLT XML: "+err.Error())
	}
	if err := os.MkdirAll(proj.Dir, 0o755); err != nil {
		return axerror.New("project.write", "cannot create project directory: "+err.Error()).AsRetryable()
	}
	if err := os.WriteFile(proj.TimelinePath(), xmlBytes, 0o644); err != nil {
		return axerror.New("project.write", "cannot write timeline: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, result)
}

// initRequest is the JSON document for the `init` tool: the directory to make a
// project and an optional display name (defaulting to the directory base name).
type initRequest struct {
	Project string `json:"project"`
	Name    string `json:"name"`
}

// initResult is the handle returned by `init`: where the manifest lives and
// whether this call created it (false when the directory was already a project).
type initResult struct {
	Project  string `json:"project"`
	Manifest string `json:"manifest"`
	Name     string `json:"name"`
	Created  bool   `json:"created"`
	Planned  bool   `json:"planned,omitempty"`
}

// initCmd creates a project directory with a project.json manifest — the durable
// state root every later step reads and writes. It is idempotent: re-running on
// an existing project returns the manifest with created=false. --plan reports
// what it would do without writing.
func initCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	plan, err := parsePlan(args)
	if err != nil {
		return err
	}
	var req initRequest
	if err := decodeRequest(stdin, &req); err != nil {
		return err
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`set "project" to the directory to initialise`)
	}
	proj := project.Open(req.Project)
	if plan {
		return emitJSON(stdout, initResult{
			Project:  proj.Dir,
			Manifest: proj.ManifestPath(),
			Name:     req.Name,
			Created:  !proj.Initialized(),
			Planned:  true,
		})
	}
	m, created, err := proj.Init(req.Name)
	if err != nil {
		return axerror.New("project.write", "cannot initialise project: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, initResult{
		Project:  proj.Dir,
		Manifest: proj.ManifestPath(),
		Name:     m.Name,
		Created:  created,
	})
}

// ingestRequest is the JSON document for the `ingest` tool: the project to record
// into and the source media file to ingest. Worker/WorkerArgs locate the ffmpeg
// worker that probes the source's pixel dimensions; when Worker is empty the
// CUTVEC_FFMPEG_WORKER env var is used, and when neither is set the source is
// still ingested, just without a probe.
type ingestRequest struct {
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// ingestResult is the handle returned by `ingest`: the recorded source and
// whether it was a content-hash cache hit (Cached=true means the identical bytes
// were already ingested, so the re-run was idempotent and changed nothing).
type ingestResult struct {
	Source  project.Source `json:"source"`
	Cached  bool           `json:"cached"`
	Planned bool           `json:"planned,omitempty"`
}

// ingestCmd records a source file in the project keyed by its content hash.
// Re-ingesting an unchanged file is a cache hit — deterministic, idempotent,
// resumable. --plan computes the hash and reports the cache status without
// writing.
func ingestCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	plan, err := parsePlan(args)
	if err != nil {
		return err
	}
	var req ingestRequest
	if err := decodeRequest(stdin, &req); err != nil {
		return err
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`set "project" to the project directory`)
	}
	if req.Source == "" {
		return axerror.New("input.invalid", "source path is required").
			WithHint(`set "source" to a media file to ingest`)
	}
	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	if plan {
		src, err := proj.DescribeSource(req.Source)
		if err != nil {
			return mapSourceErr(err)
		}
		return emitJSON(stdout, ingestResult{Source: src, Cached: proj.CacheHit(src.Hash), Planned: true})
	}
	res, err := proj.Ingest(req.Source, resolveProber(req))
	if err != nil {
		return mapSourceErr(err)
	}
	return emitJSON(stdout, ingestResult{Source: res.Source, Cached: res.Cached})
}

// ffmpegProber measures a source's dimensions by invoking the ffmpeg worker with
// op=probe. It adapts the worker boundary to project.Prober so Ingest can record
// width/height/duration/fps on the source without importing the worker package.
type ffmpegProber struct {
	w worker.Worker
}

// Probe invokes the ffmpeg worker and lifts its result into a project.Probe. Any
// worker failure surfaces as the *axerror.Error the worker boundary returns.
func (fp ffmpegProber) Probe(path string) (project.Probe, error) {
	resp, err := fp.w.Invoke(context.Background(), worker.Request{
		Op:     "probe",
		Params: map[string]any{"source": path},
	})
	if err != nil {
		return project.Probe{}, err // already an *axerror.Error from the worker boundary
	}
	return probeFromResult(resp.Result)
}

// resolveProber returns the ffmpeg prober ingest should use, or nil when no
// ffmpeg worker is configured. The worker comes from the request's worker field
// or, failing that, the CUTVEC_FFMPEG_WORKER env var. A nil prober means the
// source is ingested without a probe rather than failing — dimensions are a best
// effort, not a precondition for recording the source.
func resolveProber(req ingestRequest) project.Prober {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_FFMPEG_WORKER")
	}
	if path == "" {
		return nil
	}
	return ffmpegProber{w: worker.Worker{Path: path, Args: req.WorkerArgs}}
}

// probeFromResult lifts the ffmpeg worker's result map into a project.Probe. A
// result that is not shaped like a probe is a protocol violation.
func probeFromResult(result map[string]any) (project.Probe, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return project.Probe{}, axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var pr project.Probe
	if err := json.Unmarshal(b, &pr); err != nil {
		return project.Probe{}, axerror.New("worker.protocol",
			"worker result is not a probe: "+err.Error()).
			WithHint(`the ffmpeg worker must return {"width","height","duration","fps"}`)
	}
	return pr, nil
}

// statusRequest is the optional JSON document for the `status` tool: the project
// to report on, defaulting to the current directory when stdin is empty.
type statusRequest struct {
	Project string `json:"project"`
}

// statusCmd returns the whole project state graph as JSON — sources ingested and
// artifacts marked done/pending — so an agent can read the graph in one call.
// The --json flag affirms the (only) output form. The project comes from stdin
// or defaults to the current directory.
func statusCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	for _, a := range args {
		switch a {
		case "--json":
			// JSON is the only output form; the flag affirms it.
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --json")
		}
	}
	req := statusRequest{Project: "."}
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if req.Project == "" {
		req.Project = "."
	}
	proj := project.Open(req.Project)
	graph, err := proj.Status()
	if err != nil {
		if errors.Is(err, project.ErrNotInitialized) {
			return axerror.New("project.uninitialized", "project is not initialised").
				WithHint("run `cutvec init` on this directory first")
		}
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, graph)
}

// transcribeRequest is the JSON document for the `transcribe` tool. Project is
// required (and may instead be given as a positional argument). Source names the
// ingested source to transcribe — its path, id, or hash — and may be omitted when
// the project holds exactly one source. Worker/WorkerArgs locate the whisper
// worker executable; when Worker is empty the CUTVEC_WHISPER_WORKER env var is
// used.
type transcribeRequest struct {
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// transcribeResult is the handle returned by `transcribe`: the path to the cached
// word-level transcript plus a small summary. Cached=true means the transcript
// already existed for these bytes, so no worker ran — the idempotent re-run.
type transcribeResult struct {
	Transcript string `json:"transcript"`
	Source     string `json:"source"`
	Words      int    `json:"words"`
	Language   string `json:"language,omitempty"`
	Cached     bool   `json:"cached"`
	Planned    bool   `json:"planned,omitempty"`
}

// transcribeCmd produces a word-level transcript of an ingested source and caches
// it keyed by the source's content hash, so a re-run is a pure cache hit that
// invokes no worker. The transcript itself (words with start/end times) lives at
// the returned handle; the JSON result carries the handle, not the transcript
// blob. The project may be given as a positional argument (`cutvec transcribe
// <project>`) or in the JSON request. --plan reports the handle and cache status
// without running the worker.
func transcribeCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, plan, err := parseProjectArgs(args, "transcribe")
	if err != nil {
		return err
	}
	var req transcribeRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec transcribe <project>) or set "project" in the JSON request`)
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	m, err := proj.Load()
	if err != nil {
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	src, err := resolveSource(m, req.Source, "transcribe")
	if err != nil {
		return err
	}

	// Cache hit: the transcript for these bytes already exists. Return it
	// untouched, running no worker — the defining idempotent re-run.
	if proj.HasTranscript(src.Hash) {
		t, err := proj.ReadTranscript(src.Hash)
		if err != nil {
			return axerror.New("project.read", "cannot read cached transcript: "+err.Error()).AsRetryable()
		}
		return emitJSON(stdout, transcribeResult{
			Transcript: proj.TranscriptPath(src.Hash),
			Source:     src.Hash,
			Words:      len(t.Words),
			Language:   t.Language,
			Cached:     true,
		})
	}

	if plan {
		return emitJSON(stdout, transcribeResult{
			Transcript: proj.TranscriptPath(src.Hash),
			Source:     src.Hash,
			Cached:     false,
			Planned:    true,
		})
	}

	w, err := resolveWorker(req)
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op:     "transcribe",
		Params: map[string]any{"source": src.Path},
	})
	if err != nil {
		return err // already an *axerror.Error from the worker boundary
	}
	t, err := transcriptFromResult(resp.Result)
	if err != nil {
		return err
	}
	t.Source = src.Hash
	if err := proj.WriteTranscript(src.Hash, t); err != nil {
		return axerror.New("project.write", "cannot cache transcript: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, transcribeResult{
		Transcript: proj.TranscriptPath(src.Hash),
		Source:     src.Hash,
		Words:      len(t.Words),
		Language:   t.Language,
		Cached:     false,
	})
}

// parseProjectArgs reads the shared per-source flags (--plan dry-run, --json
// affirming the output form) and at most one positional project argument off
// args. cmd names the command in error messages (e.g. transcribe, index).
func parseProjectArgs(args []string, cmd string) (proj string, plan bool, err error) {
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return "", false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --plan (dry-run) and --json")
		default:
			if proj != "" {
				return "", false, axerror.New("input.invalid",
					cmd+" takes at most one project argument").
					WithHint(fmt.Sprintf("name one project, e.g. `cutvec %s ./proj`", cmd))
			}
			proj = a
		}
	}
	return proj, plan, nil
}

// resolveSource picks the ingested source a per-source step acts on. With no
// name it returns the project's sole source, erroring when the project has none
// (nothing to act on) or more than one (ambiguous). A name matches a source by
// its path, id, or hash. verb names the action in error messages.
func resolveSource(m project.Manifest, want, verb string) (project.Source, error) {
	if len(m.Sources) == 0 {
		return project.Source{}, axerror.New("input.empty",
			"project has no ingested sources to "+verb).
			WithHint("run `cutvec ingest` to add a source first")
	}
	if want == "" {
		if len(m.Sources) == 1 {
			return m.Sources[0], nil
		}
		return project.Source{}, axerror.New("input.invalid",
			fmt.Sprintf("project has %d sources; name one with \"source\"", len(m.Sources))).
			WithHint("set \"source\" to the path, id, or hash of the source to " + verb)
	}
	for _, s := range m.Sources {
		if s.Path == want || s.ID == want || s.Hash == want {
			return s, nil
		}
	}
	return project.Source{}, axerror.New("source.missing",
		"no ingested source matches "+strconv.Quote(want)).
		WithHint("ingest the source first, or use a path/id from `cutvec status`")
}

// resolveWorker builds the whisper Worker from the request, falling back to the
// CUTVEC_WHISPER_WORKER env var for the executable path. With neither set there
// is no worker to run, which is a non-retryable worker.missing.
func resolveWorker(req transcribeRequest) (worker.Worker, error) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_WHISPER_WORKER")
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no whisper worker configured").
			WithHint(`set "worker" in the request or the CUTVEC_WHISPER_WORKER env var to the whisper worker executable`)
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, nil
}

// transcriptFromResult lifts the worker's result map into a Transcript. A result
// that is not shaped like a transcript is a protocol violation.
func transcriptFromResult(result map[string]any) (project.Transcript, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return project.Transcript{}, axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var t project.Transcript
	if err := json.Unmarshal(b, &t); err != nil {
		return project.Transcript{}, axerror.New("worker.protocol",
			"worker result is not a transcript: "+err.Error()).
			WithHint(`the transcribe worker must return {"words":[{"word","start","end"}],...}`)
	}
	return t, nil
}

// indexRequest is the JSON document for the `index` tool. Project is required
// (and may instead be given as a positional argument). Source names the ingested
// source whose transcript to embed — its path, id, or hash — and may be omitted
// when the project holds exactly one source. Worker/WorkerArgs locate the local
// embedding worker executable; when Worker is empty the CUTVEC_INDEX_WORKER env
// var is used.
type indexRequest struct {
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// indexResult is the handle returned by `index`: the path to the cached index
// metadata plus the sibling LanceDB table and a small summary. Cached=true means
// the index already existed for these bytes, so no worker ran — the idempotent
// re-run.
type indexResult struct {
	Index    string `json:"index"`
	Table    string `json:"table"`
	Source   string `json:"source"`
	Segments int    `json:"segments"`
	Dim      int    `json:"dim"`
	Model    string `json:"model,omitempty"`
	Cached   bool   `json:"cached"`
	Planned  bool   `json:"planned,omitempty"`
}

// indexCmd embeds an ingested source's transcript segments into a local LanceDB
// index and caches it keyed by the source's content hash, so a re-run is a pure
// cache hit that invokes no worker. The vectors live in a .lance table the embed
// worker writes; the JSON result carries the handles (metadata + table), not the
// vectors. No network or API key is involved — embeddings are produced locally by
// the worker. The project may be given as a positional argument (`cutvec index
// <project>`) or in the JSON request. --plan reports the handles and cache status
// without running the worker.
func indexCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, plan, err := parseProjectArgs(args, "index")
	if err != nil {
		return err
	}
	var req indexRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec index <project>) or set "project" in the JSON request`)
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	m, err := proj.Load()
	if err != nil {
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	src, err := resolveSource(m, req.Source, "index")
	if err != nil {
		return err
	}

	// The index is built over transcript segments, so the source must be
	// transcribed first — a written precondition, like render's built timeline.
	if !proj.HasTranscript(src.Hash) {
		return axerror.New("transcript.missing", "source has no transcript to index").
			WithHint("run `cutvec transcribe` on this source first")
	}

	// Cache hit: the index for these bytes already exists. Return it untouched,
	// running no worker — the defining idempotent re-run.
	if proj.HasIndex(src.Hash) {
		idx, err := proj.ReadIndex(src.Hash)
		if err != nil {
			return axerror.New("project.read", "cannot read cached index: "+err.Error()).AsRetryable()
		}
		return emitJSON(stdout, indexResult{
			Index:    proj.IndexPath(src.Hash),
			Table:    idx.Table,
			Source:   src.Hash,
			Segments: idx.Segments,
			Dim:      idx.Dim,
			Model:    idx.Model,
			Cached:   true,
		})
	}

	if plan {
		return emitJSON(stdout, indexResult{
			Index:   proj.IndexPath(src.Hash),
			Table:   proj.IndexTablePath(src.Hash),
			Source:  src.Hash,
			Cached:  false,
			Planned: true,
		})
	}

	w, err := resolveIndexWorker(req)
	if err != nil {
		return err
	}
	tablePath := proj.IndexTablePath(src.Hash)
	if err := os.MkdirAll(proj.CacheDir(), 0o755); err != nil {
		return axerror.New("project.write", "cannot create cache directory: "+err.Error()).AsRetryable()
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op: "index",
		Params: map[string]any{
			"transcript": proj.TranscriptPath(src.Hash),
			"table":      tablePath,
		},
	})
	if err != nil {
		return err // already an *axerror.Error from the worker boundary
	}
	idx, err := indexFromResult(resp.Result)
	if err != nil {
		return err
	}
	// The worker must have written the LanceDB table it was asked to build; a
	// missing table is a protocol violation, not a silent success.
	if _, err := os.Stat(tablePath); err != nil {
		return axerror.New("worker.protocol", "index worker did not write the LanceDB table").
			WithHint("the index worker must write the .lance table at the params.table path")
	}
	idx.Source = src.Hash
	idx.Table = tablePath
	if err := proj.WriteIndex(src.Hash, idx); err != nil {
		return axerror.New("project.write", "cannot cache index: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, indexResult{
		Index:    proj.IndexPath(src.Hash),
		Table:    idx.Table,
		Source:   src.Hash,
		Segments: idx.Segments,
		Dim:      idx.Dim,
		Model:    idx.Model,
		Cached:   false,
	})
}

// resolveIndexWorker builds the local embedding Worker from the request, falling
// back to the CUTVEC_INDEX_WORKER env var for the executable path. With neither
// set there is no worker to run, which is a non-retryable worker.missing.
func resolveIndexWorker(req indexRequest) (worker.Worker, error) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_INDEX_WORKER")
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no index (embedding) worker configured").
			WithHint(`set "worker" in the request or the CUTVEC_INDEX_WORKER env var to the local embedding worker executable`)
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, nil
}

// indexFromResult lifts the worker's result map into an Index. A result that is
// not shaped like an index is a protocol violation. Source and Table are set by
// the core after the worker returns, so the worker need only report the summary.
func indexFromResult(result map[string]any) (project.Index, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return project.Index{}, axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var idx project.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return project.Index{}, axerror.New("worker.protocol",
			"worker result is not an index: "+err.Error()).
			WithHint(`the index worker must return {"segments":N,"dim":D,...}`)
	}
	return idx, nil
}

// searchRequest is the JSON document for the `search` tool. Project and Query are
// required (both may instead be given as positional arguments: `cutvec search
// <project> <query>`). Source names the indexed source to search — its path, id,
// or hash — and may be omitted when the project holds exactly one source. Limit
// caps the number of ranked candidates returned (0 = all). Worker/WorkerArgs
// locate the local search worker executable; when Worker is empty the
// CUTVEC_SEARCH_WORKER env var is used.
type searchRequest struct {
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Query      string   `json:"query"`
	Limit      int      `json:"limit"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// searchResult is what `search` returns: the query echoed back, the searched
// source, and the ranked timecoded candidates (each {start,end,text,score}),
// most-relevant first. It is not a handle to a cached file — search is a query,
// so the results ride in the response itself.
type searchResult struct {
	Query   string              `json:"query"`
	Source  string              `json:"source"`
	Results []project.Candidate `json:"results"`
	Count   int                 `json:"count"`
	Model   string              `json:"model,omitempty"`
	Planned bool                `json:"planned,omitempty"`
}

// searchCmd answers a semantic query against a source's LanceDB index with a
// list of ranked timecoded candidates. The vectors it searches live in the
// content-hash-cached .lance table built by `index`, so a source must be indexed
// first — a written precondition (index.missing) like index's own on transcribe.
// The worker embeds the query locally and returns scored hits; the core re-ranks
// them into a deterministic total order so a fixed query+index always yields the
// identical ranking. Unlike the build steps search caches nothing: it is a read,
// run on demand. --plan reports the query and source without running the worker.
func searchCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, posQuery, plan, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	var req searchRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if posQuery != "" {
		req.Query = posQuery
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec search <project> <query>) or set "project" in the JSON request`)
	}
	if req.Query == "" {
		return axerror.New("input.empty", "no query to search for").
			WithHint(`pass a query argument (cutvec search <project> "<query>") or set "query" in the JSON request`)
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	m, err := proj.Load()
	if err != nil {
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	src, err := resolveSource(m, req.Source, "search")
	if err != nil {
		return err
	}

	// Search reads the source's vector index, so the source must be indexed
	// first — a written precondition, like index's own transcript.missing.
	if !proj.HasIndex(src.Hash) {
		return axerror.New("index.missing", "source has no index to search").
			WithHint("run `cutvec index` on this source first")
	}

	if plan {
		return emitJSON(stdout, searchResult{
			Query:   req.Query,
			Source:  src.Hash,
			Results: []project.Candidate{},
			Planned: true,
		})
	}

	w, err := resolveSearchWorker(req)
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op: "search",
		Params: map[string]any{
			"query": req.Query,
			"table": proj.IndexTablePath(src.Hash),
			"limit": req.Limit,
		},
	})
	if err != nil {
		return err // already an *axerror.Error from the worker boundary
	}
	hits, model, err := searchFromResult(resp.Result)
	if err != nil {
		return err
	}

	// Re-rank into a deterministic total order regardless of the worker's
	// iteration order, then cap to the requested limit — the determinism the
	// acceptance criterion pins.
	ranked := project.RankCandidates(hits)
	if req.Limit > 0 && len(ranked) > req.Limit {
		ranked = ranked[:req.Limit]
	}
	return emitJSON(stdout, searchResult{
		Query:   req.Query,
		Source:  src.Hash,
		Results: ranked,
		Count:   len(ranked),
		Model:   model,
	})
}

// parseSearchArgs reads search's flags (--plan dry-run, --json affirming the
// output form) and up to two positionals off args: the project, then the query.
func parseSearchArgs(args []string) (proj, query string, plan bool, err error) {
	var positionals []string
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return "", "", false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --plan (dry-run) and --json")
		default:
			positionals = append(positionals, a)
		}
	}
	switch len(positionals) {
	case 0:
	case 1:
		proj = positionals[0]
	case 2:
		proj, query = positionals[0], positionals[1]
	default:
		return "", "", false, axerror.New("input.invalid",
			"search takes at most a project and a query argument").
			WithHint(`name one project and one query, e.g. cutvec search ./proj "the hook"`)
	}
	return proj, query, plan, nil
}

// resolveSearchWorker builds the local search Worker from the request, falling
// back to the CUTVEC_SEARCH_WORKER env var for the executable path. With neither
// set there is no worker to run, which is a non-retryable worker.missing.
func resolveSearchWorker(req searchRequest) (worker.Worker, error) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_SEARCH_WORKER")
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no search worker configured").
			WithHint(`set "worker" in the request or the CUTVEC_SEARCH_WORKER env var to the local search worker executable`)
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, nil
}

// searchFromResult lifts the worker's result map into ranked candidates plus the
// model it reports. A result not shaped like search hits is a protocol violation.
func searchFromResult(result map[string]any) ([]project.Candidate, string, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return nil, "", axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var payload struct {
		Candidates []project.Candidate `json:"candidates"`
		Model      string              `json:"model"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, "", axerror.New("worker.protocol",
			"worker result is not search hits: "+err.Error()).
			WithHint(`the search worker must return {"candidates":[{"start","end","text","score"}],...}`)
	}
	return payload.Candidates, payload.Model, nil
}

// speakersRequest is the JSON document for the `speakers` tool. Project is
// required (and may instead be given as a positional argument). Source names the
// ingested source to analyse — its path, id, or hash — and may be omitted when the
// project holds exactly one source. Worker/WorkerArgs locate the mediapipe worker
// executable; when Worker is empty the CUTVEC_MEDIAPIPE_WORKER env var is used.
// The speaker layout is a fixed two-person constant, not a request field, so the
// cache key stays the bare source hash with no layout collision.
type speakersRequest struct {
	Project    string   `json:"project"`
	Source     string   `json:"source"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// speakersResult is the handle returned by `speakers`: the path to the cached
// active-speaker turns plus a small summary. Cached=true means the turns already
// existed for these bytes, so no worker ran — the idempotent re-run.
type speakersResult struct {
	Speakers string `json:"speakers"`
	Source   string `json:"source"`
	Layout   string `json:"layout"`
	Turns    int    `json:"turns"`
	// Coverage says how much of the media was attributed to a speaker.
	// Ambiguous stretches stay unattributed and become the wide two-shot, so a
	// low fraction predicts a mostly-wide edit rather than a detection silently
	// getting it wrong.
	Coverage *project.Coverage `json:"coverage,omitempty"`
	Cached   bool              `json:"cached"`
	Planned  bool              `json:"planned,omitempty"`
}

// speakersCmd produces a per-time active-speaker track for an ingested source and
// caches it keyed by the source's content hash, so a re-run is a pure cache hit
// that invokes no worker. Detection runs against a fixed two-person layout. The
// turns themselves (speaker label with start/end times) live at the returned
// handle; the JSON result carries the handle, not the turns blob. The project may
// be given as a positional argument (`cutvec speakers <project>`) or in the JSON
// request. --plan reports the handle and cache status without running the worker.
func speakersCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, plan, err := parseSpeakersArgs(args)
	if err != nil {
		return err
	}
	var req speakersRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec speakers <project>) or set "project" in the JSON request`)
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	m, err := proj.Load()
	if err != nil {
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	src, err := resolveSource(m, req.Source, "speakers")
	if err != nil {
		return err
	}

	// Cache hit: the speaker track for these bytes already exists. Return it
	// untouched, running no worker — the defining idempotent re-run.
	if proj.HasSpeakers(src.Hash) {
		s, err := proj.ReadSpeakers(src.Hash)
		if err != nil {
			return axerror.New("project.read", "cannot read cached speakers: "+err.Error()).AsRetryable()
		}
		return emitJSON(stdout, speakersResult{
			Speakers: proj.SpeakersPath(src.Hash),
			Source:   src.Hash,
			Layout:   s.Layout,
			Turns:    len(s.Turns),
			Coverage: s.Coverage,
			Cached:   true,
		})
	}

	if plan {
		return emitJSON(stdout, speakersResult{
			Speakers: proj.SpeakersPath(src.Hash),
			Source:   src.Hash,
			Layout:   project.TwoPersonLayout,
			Cached:   false,
			Planned:  true,
		})
	}

	w, err := resolveMediapipeWorker(req)
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op:     "speakers",
		Params: map[string]any{"source": src.Path, "layout": project.TwoPersonLayout},
	})
	if err != nil {
		return err // already an *axerror.Error from the worker boundary
	}
	s, err := speakersFromResult(resp.Result)
	if err != nil {
		return err
	}
	s.Source = src.Hash
	// The layout is fixed by the core, never taken from the worker, so the cache
	// key (bare source hash) can never disagree with the recorded layout.
	s.Layout = project.TwoPersonLayout
	if err := proj.WriteSpeakers(src.Hash, s); err != nil {
		return axerror.New("project.write", "cannot cache speakers: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, speakersResult{
		Speakers: proj.SpeakersPath(src.Hash),
		Source:   src.Hash,
		Layout:   s.Layout,
		Turns:    len(s.Turns),
		Coverage: s.Coverage,
		Cached:   false,
	})
}

// parseSpeakersArgs reads speakers' flags (--plan dry-run, --json affirming the
// output form) and at most one positional project argument off args.
func parseSpeakersArgs(args []string) (proj string, plan bool, err error) {
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return "", false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --plan (dry-run) and --json")
		default:
			if proj != "" {
				return "", false, axerror.New("input.invalid",
					"speakers takes at most one project argument").
					WithHint("name one project, e.g. `cutvec speakers ./proj`")
			}
			proj = a
		}
	}
	return proj, plan, nil
}

// resolveMediapipeWorker builds the mediapipe Worker from the request, falling
// back to the CUTVEC_MEDIAPIPE_WORKER env var for the executable path. With
// neither set there is no worker to run, which is a non-retryable worker.missing.
func resolveMediapipeWorker(req speakersRequest) (worker.Worker, error) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_MEDIAPIPE_WORKER")
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no mediapipe worker configured").
			WithHint(`set "worker" in the request or the CUTVEC_MEDIAPIPE_WORKER env var to the mediapipe worker executable`)
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, nil
}

// speakersFromResult lifts the worker's result map into a Speakers track. A result
// that is not shaped like a speaker track is a protocol violation.
func speakersFromResult(result map[string]any) (project.Speakers, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return project.Speakers{}, axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var s project.Speakers
	if err := json.Unmarshal(b, &s); err != nil {
		return project.Speakers{}, axerror.New("worker.protocol",
			"worker result is not a speaker track: "+err.Error()).
			WithHint(`the speakers worker must return {"turns":[{"speaker","start","end"}],...}`)
	}
	return s, nil
}

// Render quality tiers: a fast draft ("proxy") and a full-quality final render
// ("final"). They key separate cache entries so both can coexist per timeline.
const (
	qualityProxy = "proxy"
	qualityFinal = "final"
)

// renderRequest is the JSON document for the `render` tool. Project is required
// (and may instead be given as a positional argument). Quality selects the tier
// ("proxy" or "final") and is overridden by the --proxy/--final flags; it
// defaults to proxy. Worker/WorkerArgs locate the melt render worker executable;
// when Worker is empty the CUTVEC_MELT_WORKER env var is used.
type renderRequest struct {
	Project    string   `json:"project"`
	Quality    string   `json:"quality"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// renderResult is the handle returned by `render`: the path to the rendered
// video plus a small summary. Cached=true means the render already existed for
// these timeline bytes at this quality, so no worker ran — the idempotent re-run.
type renderResult struct {
	Output   string `json:"output"`
	Timeline string `json:"timeline"`
	Source   string `json:"source"`
	Quality  string `json:"quality"`
	Cached   bool   `json:"cached"`
	Planned  bool   `json:"planned,omitempty"`
}

// renderCmd renders a project's built MLT timeline to an output video with the
// melt worker, caching the result keyed by the timeline's content hash and the
// quality tier so a re-run is a pure cache hit that invokes no worker. The
// rendered video lives at the returned handle; the JSON result carries the
// handle, not the video blob. The project may be given as a positional argument
// (`cutvec render <project>`) or in the JSON request, and the quality tier via
// --proxy/--final (default proxy) or the request's "quality". --plan reports the
// handle and cache status without running the worker.
func renderCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, flagQuality, plan, err := parseRenderArgs(args)
	if err != nil {
		return err
	}
	var req renderRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if flagQuality != "" {
		req.Quality = flagQuality
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec render <project>) or set "project" in the JSON request`)
	}
	quality, err := resolveQuality(req.Quality)
	if err != nil {
		return err
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}
	tlPath := proj.TimelinePath()
	hash, err := project.HashFile(tlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return axerror.New("timeline.missing", "project has no built timeline to render").
				WithHint("run `cutvec timeline` to build a timeline first")
		}
		return axerror.New("project.read", "cannot read timeline: "+err.Error()).AsRetryable()
	}

	// Cache hit: this timeline was already rendered at this quality. Return the
	// cached handle untouched, running no worker — the defining idempotent re-run.
	if proj.HasRender(hash, quality) {
		return emitJSON(stdout, renderResult{
			Output:   proj.RenderPath(hash, quality),
			Timeline: tlPath,
			Source:   hash,
			Quality:  quality,
			Cached:   true,
		})
	}

	if plan {
		return emitJSON(stdout, renderResult{
			Output:   proj.RenderPath(hash, quality),
			Timeline: tlPath,
			Source:   hash,
			Quality:  quality,
			Cached:   false,
			Planned:  true,
		})
	}

	w, err := resolveMeltWorker(req)
	if err != nil {
		return err
	}
	out := proj.RenderPath(hash, quality)
	if err := os.MkdirAll(proj.CacheDir(), 0o755); err != nil {
		return axerror.New("project.write", "cannot create cache directory: "+err.Error()).AsRetryable()
	}
	if _, err := w.Invoke(context.Background(), worker.Request{
		Op:     "render",
		Params: map[string]any{"timeline": tlPath, "output": out, "quality": quality},
	}); err != nil {
		return err // already an *axerror.Error from the worker boundary
	}
	// The worker's contract is to write the rendered video to the requested output
	// path; a run that returns success without it is a protocol violation.
	if _, err := os.Stat(out); err != nil {
		return axerror.New("worker.protocol", "render worker did not write the output file").
			WithHint("the melt worker must write the rendered video to the requested output path").AsRetryable()
	}
	return emitJSON(stdout, renderResult{
		Output:   out,
		Timeline: tlPath,
		Source:   hash,
		Quality:  quality,
		Cached:   false,
	})
}

// parseRenderArgs reads render's flags (--plan dry-run, --json affirming the
// output form, and the mutually exclusive --proxy/--final quality selectors) and
// at most one positional project argument off args.
func parseRenderArgs(args []string) (proj, quality string, plan bool, err error) {
	proxy, final := false, false
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case a == "--proxy":
			proxy = true
		case a == "--final":
			final = true
		case strings.HasPrefix(a, "-"):
			return "", "", false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --proxy/--final (quality), --plan (dry-run) and --json")
		default:
			if proj != "" {
				return "", "", false, axerror.New("input.invalid",
					"render takes at most one project argument").
					WithHint("name one project, e.g. `cutvec render ./proj`")
			}
			proj = a
		}
	}
	if proxy && final {
		return "", "", false, axerror.New("input.invalid",
			"choose one of --proxy or --final, not both").
			WithHint("--proxy is the fast draft render, --final the full-quality one")
	}
	switch {
	case proxy:
		quality = qualityProxy
	case final:
		quality = qualityFinal
	}
	return proj, quality, plan, nil
}

// resolveQuality validates the quality tier, defaulting to the fast proxy render
// when none is given and rejecting any tier that is neither proxy nor final.
func resolveQuality(q string) (string, error) {
	switch q {
	case "":
		return qualityProxy, nil
	case qualityProxy, qualityFinal:
		return q, nil
	default:
		return "", axerror.New("input.invalid", fmt.Sprintf("unknown quality %q", q)).
			WithHint(`quality must be "proxy" (fast draft) or "final" (full quality)`)
	}
}

// resolveMeltWorker builds the melt Worker from the request, falling back to the
// CUTVEC_MELT_WORKER env var for the executable path. With neither set there is
// no worker to run, which is a non-retryable worker.missing.
func resolveMeltWorker(req renderRequest) (worker.Worker, error) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("CUTVEC_MELT_WORKER")
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no melt worker configured").
			WithHint(`set "worker" in the request or the CUTVEC_MELT_WORKER env var to the melt render worker executable`)
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, nil
}

// doctorCmd preflights the whole runtime environment in one call: it probes
// every binary, worker, python package, and model file the pipeline needs and
// writes the ok/missing report (with an install hint per gap) to stdout. It runs
// no worker and touches no project — pure inspection — so it can gate a pipeline
// before real work starts. The report is always emitted; when a required
// dependency is missing the command additionally returns a structured error so
// the process exits non-zero, exactly the signal a caller wants before spending
// minutes on a run that would fail halfway.
func doctorCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	for _, a := range args {
		switch a {
		case "--json":
			// The report is always JSON; the flag is accepted for parity with
			// the other tools' explicit --json.
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("doctor takes no arguments; the only flag is --json")
		}
	}

	rep := doctor.Run(osProbe{python: pythonInterpreter()})
	if err := emitJSON(stdout, rep); err != nil {
		return err
	}
	if !rep.Ok {
		return axerror.New("doctor.preflight_failed",
			fmt.Sprintf("environment preflight failed: %d required dependency(ies) missing (%s)",
				rep.RequiredMissing, strings.Join(rep.MissingNames, ", "))).
			WithHint("install or configure each missing dependency; see the per-item hints in the report on stdout")
	}
	return nil
}

// osProbe is the production doctor.Probe: it inspects the real host via PATH,
// the filesystem, the environment, and a python subprocess for module checks.
type osProbe struct {
	// python is the interpreter used to test module importability; empty when
	// no python is on PATH, in which case every package check reports missing.
	python string
}

func (osProbe) LookPath(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func (osProbe) FileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (osProbe) Getenv(key string) string { return os.Getenv(key) }

func (osProbe) Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// HasModule reports whether the interpreter can import name. It uses
// importlib.util.find_spec, which locates the module without importing it, so a
// heavy package (mediapipe) is not actually loaded just to answer yes/no.
func (o osProbe) HasModule(name string) bool {
	if o.python == "" {
		return false
	}
	cmd := exec.Command(o.python, "-c",
		"import importlib.util,sys; sys.exit(0 if importlib.util.find_spec(sys.argv[1]) else 1)", name)
	return cmd.Run() == nil
}

// pythonInterpreter resolves the interpreter the module checks run under,
// preferring python3 and falling back to python; "" when neither is on PATH.
func pythonInterpreter() string {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// parsePlan reads the shared --plan dry-run flag off args, rejecting any other
// flag with a stable code.
func parsePlan(args []string) (bool, error) {
	plan := false
	for _, a := range args {
		switch a {
		case "--plan":
			plan = true
		default:
			return false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --plan (dry-run)")
		}
	}
	return plan, nil
}

// decodeRequest reads a single JSON object from stdin into v, mapping an empty
// stream to input.empty and malformed JSON to input.invalid.
func decodeRequest(stdin io.Reader, v any) error {
	if err := json.NewDecoder(stdin).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return axerror.New("input.empty", "no JSON request on stdin").
				WithHint("pipe a JSON request object to stdin")
		}
		return axerror.New("input.invalid", "request is not valid JSON: "+err.Error()).
			WithHint("stdin must be a single JSON object")
	}
	return nil
}

// decodeOptionalRequest is decodeRequest for reads with no required input: an
// empty stdin leaves v at its defaults rather than erroring.
func decodeOptionalRequest(stdin io.Reader, v any) error {
	if err := json.NewDecoder(stdin).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return axerror.New("input.invalid", "request is not valid JSON: "+err.Error()).
			WithHint("stdin must be a single JSON object")
	}
	return nil
}

// mapSourceErr turns a filesystem error from reading a source into a stable
// axerror: a missing file is a caller mistake (source.missing), any other read
// error may be transient (source.read, retryable). ErrNotInitialized is mapped
// for defence in depth, though ingestCmd checks initialisation up front.
func mapSourceErr(err error) error {
	switch {
	case errors.Is(err, project.ErrNotInitialized):
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	case errors.Is(err, os.ErrNotExist):
		return axerror.New("source.missing", "source file not found: "+err.Error()).
			WithHint("check the source path exists and is readable")
	default:
		return axerror.New("source.read", "cannot read source file: "+err.Error()).AsRetryable()
	}
}

// usage prints the catalog's tool names so a caller (human or agent) can
// discover the surface without a separate help system.
func usage() error {
	fmt.Fprintln(os.Stderr, "cutvec — agent-native video-editing CLI + MCP server")
	fmt.Fprintln(os.Stderr, "\nusage: cutvec <command> [args]\n\ncommands:")
	fmt.Fprintln(os.Stderr, "  describe   emit the tool catalog (--json for the tools array)")
	fmt.Fprintln(os.Stderr, "  schema     emit the JSON Schema for one command: schema <cmd>")
	fmt.Fprintln(os.Stderr, "  mcp        serve the catalog over MCP (not yet implemented)")
	for _, t := range catalog.Default().Tools {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", t.Name, t.Summary)
	}
	return nil
}

// emitJSON writes v as indented JSON to w.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emitError writes err as the structured axerror JSON shape to stderr, wrapping
// any non-axerror error in a generic code so every failure is machine-legible.
func emitError(err error) {
	ae, ok := err.(*axerror.Error)
	if !ok {
		ae = axerror.New("internal", err.Error())
	}
	b, _ := json.Marshal(ae)
	fmt.Fprintln(os.Stderr, string(b))
}
