// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"io"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/progress"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/worker"
)

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
func transcribeCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	var resp *worker.Response
	if err := progress.While(stderr, "transcribe", progressInterval(), func() error {
		var e error
		resp, e = w.Invoke(context.Background(), worker.Request{
			Op:     "transcribe",
			Params: map[string]any{"source": src.Path},
		})
		return e
	}); err != nil {
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

// resolveWorker builds the whisper Worker from the request, falling back to the
// CUTVEC_WHISPER_WORKER env var for the executable path. With neither set there
// is no worker to run, which is a non-retryable worker.missing.
func resolveWorker(req transcribeRequest) (worker.Worker, error) {
	return resolveNamedWorker(req.Worker, req.WorkerArgs, whisperWorkerSpec)
}

// transcriptFromResult lifts the worker's result map into a Transcript. A result
// that is not shaped like a transcript is a protocol violation.
func transcriptFromResult(result map[string]any) (project.Transcript, error) {
	return resultAs[project.Transcript](result, "a transcript", `the transcribe worker must return {"words":[{"word","start","end"}],...}`)
}
