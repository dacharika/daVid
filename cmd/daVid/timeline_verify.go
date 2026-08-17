package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/worker"
)

// verifyRequest is the JSON document for `timeline verify`: which project's
// timeline to preflight, and how many frames to prove it on.
type verifyRequest struct {
	Project    string   `json:"project"`
	Timeline   string   `json:"timeline"`
	Frames     int      `json:"frames"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

// verifyResult reports whether melt could load and play the timeline, and what
// it complained about when it could not.
type verifyResult struct {
	Timeline string   `json:"timeline"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems,omitempty"`
	Frames   int      `json:"frames"`
	Seconds  float64  `json:"seconds"`
}

// timelineVerifyCmd preflights a built timeline through melt itself.
//
// `--plan` and Validate() both check structure in Go, which cannot know whether
// melt will resolve every producer and filter at render time. This plays the
// real graph with no encoder attached, so a timeline that would die minutes into
// a render — or worse, halfway through a batch of shorts — fails in about a
// second instead.
//
// The failure is reported as a normal result (ok:false with problems) rather
// than an error, because "the timeline does not load" is an answer to the
// question asked, not a failure to answer it. A caller that wants it fatal can
// branch on ok.
func timelineVerifyCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) > 0 {
		return axerror.New("cli.unknown_flag", fmt.Sprintf("unexpected argument %q", args[0])).
			WithHint("`daVid timeline verify` takes its input as JSON on stdin")
	}
	var req verifyRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	path := req.Timeline
	if path == "" {
		if req.Project == "" {
			return axerror.New("input.invalid", "project directory is required").
				WithHint(`set "project", or "timeline" to verify a specific .mlt file`)
		}
		proj := project.Open(req.Project)
		if !proj.Initialized() {
			return axerror.New("project.uninitialized", "project is not initialised").
				WithHint("run `daVid init` on this directory first")
		}
		path = proj.TimelinePath()
	}

	w, err := resolveNamedWorker(req.Worker, req.WorkerArgs, meltWorkerSpec)
	if err != nil {
		return err
	}
	params := map[string]any{"timeline": path}
	if req.Frames > 0 {
		params["frames"] = req.Frames
	}
	resp, err := w.Invoke(context.Background(), worker.Request{Op: "verify", Params: params})
	if err != nil {
		return err // already an *axerror.Error from the worker boundary
	}

	out := verifyResult{Timeline: path}
	if v, ok := resp.Result["ok"].(bool); ok {
		out.OK = v
	}
	if v, ok := resp.Result["frames"].(float64); ok {
		out.Frames = int(v)
	}
	if v, ok := resp.Result["seconds"].(float64); ok {
		out.Seconds = v
	}
	if list, ok := resp.Result["problems"].([]any); ok {
		for _, raw := range list {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				out.Problems = append(out.Problems, s)
			}
		}
	}
	return emitJSON(stdout, out)
}
