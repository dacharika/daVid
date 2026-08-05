// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
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
		return emitJSON(stdout, catalog.Default())
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

// usage prints the catalog's tool names so a caller (human or agent) can
// discover the surface without a separate help system.
func usage() error {
	fmt.Fprintln(os.Stderr, "cutvec — agent-native video-editing CLI + MCP server")
	fmt.Fprintln(os.Stderr, "\nusage: cutvec <command> [args]\n\ncommands:")
	fmt.Fprintln(os.Stderr, "  describe   emit the tool catalog and JSON Schemas")
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
