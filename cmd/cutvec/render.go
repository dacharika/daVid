// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/progress"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/worker"
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
func renderCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	// Render to a temp path and publish atomically: a melt that dies mid-encode
	// leaves only the discarded .tmp, never a truncated mp4 at the cache path that
	// the next resumed run would return as a completed cache hit. We os.Rename onto
	// the cache path only after the worker fully succeeds AND wrote its output.
	tmp := out + ".tmp"
	_ = os.Remove(tmp) // clear any stale temp from a previously interrupted run
	if err := progress.While(stderr, "render", progressInterval(), func() error {
		_, e := w.Invoke(context.Background(), worker.Request{
			Op:     "render",
			Params: map[string]any{"timeline": tlPath, "output": tmp, "quality": quality},
		})
		return e
	}); err != nil {
		os.Remove(tmp) // a crashed worker's partial bytes must not survive
		return err     // already an *axerror.Error from the worker boundary
	}
	// The worker's contract is to write the rendered video to the requested output
	// path; a run that returns success without it is a protocol violation.
	if _, err := os.Stat(tmp); err != nil {
		os.Remove(tmp)
		return axerror.New("worker.protocol", "render worker did not write the output file").
			WithHint("the melt worker must write the rendered video to the requested output path").AsRetryable()
	}
	if err := os.Rename(tmp, out); err != nil {
		os.Remove(tmp)
		return axerror.New("project.write", "cannot publish rendered output: "+err.Error()).AsRetryable()
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
	return resolveNamedWorker(req.Worker, req.WorkerArgs, meltWorkerSpec)
}
