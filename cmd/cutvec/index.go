// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"io"
	"os"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/worker"
)

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
	return resolveNamedWorker(req.Worker, req.WorkerArgs, indexWorkerSpec)
}

// indexFromResult lifts the worker's result map into an Index. A result that is
// not shaped like an index is a protocol violation. Source and Table are set by
// the core after the worker returns, so the worker need only report the summary.
func indexFromResult(result map[string]any) (project.Index, error) {
	return resultAs[project.Index](result, "an index", `the index worker must return {"segments":N,"dim":D,...}`)
}
