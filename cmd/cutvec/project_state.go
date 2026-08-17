// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
)

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
