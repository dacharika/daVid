// Command daVid is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `daVid mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"io"
	"os"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/worker"
)

// ingestRequest is the JSON document for the `ingest` tool: the project to record
// into and the source media file to ingest. Worker/WorkerArgs locate the ffmpeg
// worker that probes the source's pixel dimensions; when Worker is empty the
// DAVID_FFMPEG_WORKER env var is used, and when neither is set the source is
// still ingested, just without a probe.
type ingestRequest struct {
	// Rehash forces the full content hash to be recomputed instead of trusting
	// the recorded fingerprint. Correctness escape hatch, not a normal path.
	Rehash     bool     `json:"rehash"`
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
	// Drop the recorded fingerprint first, so the content hash is recomputed
	// from every byte. A vanished or unreadable source is left to the ingest
	// below to report — it already has the good error for that.
	if req.Rehash {
		_ = proj.ForgetFingerprint(req.Source)
	}
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `daVid init` on this directory first")
	}
	if plan {
		src, err := proj.DescribeSource(req.Source)
		if err != nil {
			return mapSourceErr(err)
		}
		return emitJSON(stdout, ingestResult{Source: src, Cached: proj.CacheHit(src.Hash), Planned: true})
	}
	res, err := proj.Ingest(req.Source, resolveProber(req), resolveDeriver(req))
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
// or, failing that, the DAVID_FFMPEG_WORKER env var. A nil prober means the
// source is ingested without a probe rather than failing — dimensions are a best
// effort, not a precondition for recording the source.
func resolveProber(req ingestRequest) project.Prober {
	w, ok := ffmpegWorkerFor(req)
	if !ok {
		return nil
	}
	return ffmpegProber{w: w}
}

// ffmpegDeriver builds a source's derived media through the same ffmpeg worker
// the prober uses — one executable to install and configure, three ops.
type ffmpegDeriver struct {
	w worker.Worker
}

// Proxy builds the small, fast-seeking copy the expensive steps read instead of
// the master.
func (fd ffmpegDeriver) Proxy(src, dest string) (int, int, error) {
	resp, err := fd.w.Invoke(context.Background(), worker.Request{
		Op:     "proxy",
		Params: map[string]any{"source": src, "dest": dest},
	})
	if err != nil {
		return 0, 0, err // already an *axerror.Error from the worker boundary
	}
	// Dimensions are what the worker actually wrote. A worker that does not
	// report them yields zeroes, which downstream reads as "unknown" and falls
	// back to the master — better than guessing at a scale rule.
	w, _ := resp.Result["width"].(float64)
	h, _ := resp.Result["height"].(float64)
	return int(w), int(h), nil
}

// Audio extracts the source's audio track on its own.
func (fd ffmpegDeriver) Audio(src, dest string) error {
	_, err := fd.w.Invoke(context.Background(), worker.Request{
		Op:     "audio",
		Params: map[string]any{"source": src, "dest": dest},
	})
	return err
}

// resolveDeriver returns the ffmpeg deriver ingest should use, or nil when no
// ffmpeg worker is configured. Resolved the same way as the prober and from the
// same worker, so configuring one configures both; a nil deriver means the
// source is ingested without derived media rather than failing.
func resolveDeriver(req ingestRequest) project.Deriver {
	w, ok := ffmpegWorkerFor(req)
	if !ok {
		return nil
	}
	return ffmpegDeriver{w: w}
}

// ffmpegWorkerFor locates the one ffmpeg worker both the prober and the deriver
// use, so configuring it once configures both. Unlike the steps that refuse
// without a worker, ingest treats its absence as "skip the optional extras": a
// source must stay recordable on a machine with no ffmpeg.
func ffmpegWorkerFor(req ingestRequest) (worker.Worker, bool) {
	path := req.Worker
	if path == "" {
		path = os.Getenv("DAVID_FFMPEG_WORKER")
	}
	if path == "" {
		return worker.Worker{}, false
	}
	return worker.Worker{Path: path, Args: req.WorkerArgs}, true
}

// probeFromResult lifts the ffmpeg worker's result map into a project.Probe. A
// result that is not shaped like a probe is a protocol violation.
func probeFromResult(result map[string]any) (project.Probe, error) {
	return resultAs[project.Probe](result, "a probe", `the ffmpeg worker must return {"width","height","duration","fps"}`)
}
