// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/progress"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/worker"
)

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
func speakersCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	// Detect on the proxy when ingest built one. This is what the proxy is for,
	// and the substitution is safe because the worker's output is time-based
	// (turns) and normalised (seat cx/cy in 0..1), so nothing carries the
	// proxy's pixel dimensions into the crop windows shorts and reframe compute.
	//
	// Measured on a 20s 4K clip, proxy (960x540) against master (3840x2160):
	//
	//   detection time   9.40s -> 4.15s
	//   attributed       96.3% -> 100%
	//   seat positions   agree to within 0.002 normalised (~6px at 4K)
	//
	// Faster and better attributed, because scaling down feeds the detector a
	// frame closer to the size it expects rather than a 4K one it must fit.
	analyse := src.Path
	if src.Proxy != "" {
		if _, statErr := os.Stat(src.Proxy); statErr == nil {
			analyse = src.Proxy
		}
		// A recorded handle whose file has since been deleted falls back to the
		// master rather than failing: the proxy is an optimisation, not a
		// precondition.
	}

	var resp *worker.Response
	if err := progress.While(stderr, "speakers", progressInterval(), func() error {
		var e error
		resp, e = w.Invoke(context.Background(), worker.Request{
			Op:     "speakers",
			Params: map[string]any{"source": analyse, "layout": project.TwoPersonLayout},
		})
		return e
	}); err != nil {
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
	return resolveNamedWorker(req.Worker, req.WorkerArgs, mediapipeWorkerSpec)
}

// speakersFromResult lifts the worker's result map into a Speakers track. A result
// that is not shaped like a speaker track is a protocol violation.
func speakersFromResult(result map[string]any) (project.Speakers, error) {
	return resultAs[project.Speakers](result, "a speaker track", `the speakers worker must return {"turns":[{"speaker","start","end"}],...}`)
}
