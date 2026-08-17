// Command daVid is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `daVid mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/timeline"
)

// timelineRequest is the JSON document a caller writes to stdin for the
// `timeline` tool: the project to write into and the clips to place on it.
type timelineRequest struct {
	Project string         `json:"project"`
	Clips   []timelineClip `json:"clips"`
	// Effects attach MLT filters to the timeline: either named presets or any
	// MLT service by name. `daVid filters` lists what this machine's MLT has.
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

// timelineCmd dispatches the `timeline` tool. With no subcommand it builds a
// fresh timeline from a clip list (the historic behaviour); the `inspect` and
// `edit` subcommands read back and mutate an existing one, so an agent can adjust
// a single filter without re-running search and detection to rebuild the edit.
func timelineCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "inspect":
			return timelineInspectCmd(args[1:], stdin, stdout)
		case "edit":
			return timelineEditCmd(args[1:], stdin, stdout)
		case "verify":
			return timelineVerifyCmd(args[1:], stdin, stdout)
		}
	}
	return timelineBuildCmd(args, stdin, stdout)
}

// timelineBuildCmd carries one build request end-to-end: it reads a JSON clip
// list from stdin, builds the MLT XML timeline (the agent-editable artifact),
// writes it into the project directory, and returns a path handle as JSON on
// stdout. With --plan it reports what it would write without touching the
// filesystem.
func timelineBuildCmd(args []string, stdin io.Reader, stdout io.Writer) error {
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
	// A multi-clip build is one ordered edit, not a pile of clips: sequence the
	// producers into a playlist + tractor so every clip plays in turn. Without
	// this a no-audio build marshals bare <producer>s with no <playlist>, and
	// melt plays only the first — silently losing the rest. A lone clip needs no
	// track structure (melt plays a single producer as is). When audio is added
	// below, AddAudio reuses this tractor; its own nil-tractor guard stays as a
	// safety net for the audio-into-a-bare-timeline path.
	if len(tl.Producers) >= 2 {
		tl.Sequence("main")
	}
	// Audio comes after effects so a bed is not caught by an effect meant for
	// the picture, and it builds the track structure the mix needs.
	if err := applyAudio(tl, req.Audio); err != nil {
		return err
	}

	// Validate the composed timeline before anything is written. Every other
	// timeline-producing path already does this (reframe.go, shorts.go,
	// timeline_edit.go); build did not, so a request like two clips sharing one
	// explicit id returned success and persisted an MLT melt cannot render — the
	// failure surfaced much later as an opaque melt error, at render time, with
	// nothing pointing back at the build that caused it.
	//
	// Checked ahead of the --plan branch on purpose: a dry run whose whole job is
	// to report what would happen should not answer "I would write this" when the
	// write produces something broken.
	if problems := tl.Validate(); len(problems) > 0 {
		return axerror.New("timeline.invalid",
			fmt.Sprintf("timeline is not renderable: %v", problems)).
			WithHint("fix the clips in the request — e.g. give each clip a unique id")
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
