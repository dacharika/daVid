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
	"path/filepath"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
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
	case "timeline":
		return timelineCmd(args[1:], stdin, stdout)
	case "mcp":
		return axerror.New("mcp.unimplemented", "the MCP server is not built yet").
			WithHint("run `cutvec describe` to see the tool catalog")
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
// into and the source media file to ingest.
type ingestRequest struct {
	Project string `json:"project"`
	Source  string `json:"source"`
}

// ingestResult is the handle returned by `ingest`: the recorded source — which
// carries handles to its proxy and audio plus the probe metadata — and whether
// it was a content-hash cache hit (Cached=true means the identical bytes were
// already ingested, so the re-run was idempotent, the ffmpeg worker was not
// re-run, and nothing changed).
type ingestResult struct {
	Source  project.Source `json:"source"`
	Cached  bool           `json:"cached"`
	Planned bool           `json:"planned,omitempty"`
}

// ingestCmd records a source file in the project keyed by its content hash and,
// on a cache miss, drives the ffmpeg worker to probe it, build a fast proxy, and
// extract its audio — returning handles to all three plus the probe metadata.
// Re-ingesting an unchanged file is a cache hit that returns the recorded handles
// without touching the worker: deterministic, idempotent, resumable. --plan
// computes the hash and reports the cache status without writing or probing.
func ingestCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	plan := false
	for _, a := range args {
		switch a {
		case "--plan":
			plan = true
		case "--json":
			// JSON is the only output form; the flag affirms it.
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("flags are --plan (dry-run) and --json (affirm JSON output)")
		}
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
	res, err := proj.Ingest(req.Source, ffmpegArtifactor{ctx: context.Background()})
	if err != nil {
		return mapSourceErr(err)
	}
	return emitJSON(stdout, ingestResult{Source: res.Source, Cached: res.Cached})
}

// ffmpegOp is the single worker operation ingest drives: given a source and two
// output paths, the worker probes the media, writes a fast proxy, and extracts
// the audio track, reporting the probe metadata back in one round-trip.
const ffmpegOp = "ingest"

// ffmpegArtifactor is the project.Artifactor backed by the ffmpeg worker (task
// 009's JSON-over-stdin/stdout boundary). It writes no media itself: it hands the
// worker output paths under the project cache and trusts the worker to write
// them, then verifies both files landed before returning their handles. The
// worker is resolved lazily, only when a source is actually built (a cache miss
// on a real file), so a missing worker never masks an input error like a
// non-existent source.
type ffmpegArtifactor struct {
	ctx context.Context
}

// BuildArtifacts drives the ffmpeg worker for one source and returns handles to
// the proxy and audio it wrote plus the probe metadata it reported. Every
// failure is the structured axerror the worker boundary already produces.
func (fa ffmpegArtifactor) BuildArtifacts(src project.Source, cacheDir string) (project.Artifacts, error) {
	w, err := resolveFFmpegWorker()
	if err != nil {
		return project.Artifacts{}, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return project.Artifacts{}, axerror.New("project.write",
			"cannot create cache directory: "+err.Error()).AsRetryable()
	}
	proxyOut := filepath.Join(cacheDir, src.Hash+".proxy.mp4")
	audioOut := filepath.Join(cacheDir, src.Hash+".audio.wav")

	resp, err := w.Invoke(fa.ctx, worker.Request{
		Op: ffmpegOp,
		Params: map[string]any{
			"source": src.Path,
			"proxy":  proxyOut,
			"audio":  audioOut,
		},
	})
	if err != nil {
		return project.Artifacts{}, err // already an *axerror.Error from the worker boundary
	}

	// The worker must have written both handles; a handle that does not resolve
	// to a real file is a protocol violation, not a usable artifact.
	for _, out := range []string{proxyOut, audioOut} {
		if _, statErr := os.Stat(out); statErr != nil {
			return project.Artifacts{}, axerror.New("worker.protocol",
				"ffmpeg worker did not write "+out+": "+statErr.Error()).
				WithHint("the ingest worker must write the proxy and audio files it is given")
		}
	}
	probe, _ := resp.Result["probe"].(map[string]any)
	return project.Artifacts{Proxy: proxyOut, Audio: audioOut, Probe: probe}, nil
}

// resolveFFmpegWorker builds the ffmpeg worker handle from the environment:
// CUTVEC_FFMPEG_WORKER is the executable to run and CUTVEC_FFMPEG_WORKER_ARGS its
// fixed leading arguments (space-separated, typically the worker script path).
// With no worker configured there is nothing to probe/proxy/extract with, so
// ingest returns a non-retryable worker.missing pointing the caller at the fix.
func resolveFFmpegWorker() (worker.Worker, error) {
	path := os.Getenv("CUTVEC_FFMPEG_WORKER")
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing", "no ffmpeg worker is configured").
			WithHint("set CUTVEC_FFMPEG_WORKER to the ffmpeg worker executable (install the Python workers)")
	}
	w := worker.Worker{Path: path}
	if a := strings.Fields(os.Getenv("CUTVEC_FFMPEG_WORKER_ARGS")); len(a) > 0 {
		w.Args = a
	}
	return w, nil
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
// for defence in depth, though ingestCmd checks initialisation up front. An error
// that is already a structured axerror — the ffmpeg worker's failures reach here
// through Ingest's Artifactor — passes through unchanged so its code survives.
func mapSourceErr(err error) error {
	var ae *axerror.Error
	if errors.As(err, &ae) {
		return err
	}
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
