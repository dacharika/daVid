// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/worker"
)

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
	return resolveNamedWorker(req.Worker, req.WorkerArgs, searchWorkerSpec)
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
