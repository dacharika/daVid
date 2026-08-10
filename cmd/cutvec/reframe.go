package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
	"github.com/tahabsn/cutvec/internal/worker"
)

// reframe-edit is the recipe that turns one locked-off wide shot into an edit
// that looks multi-camera. A high-resolution frame holds far more detail than
// the delivery format needs, so a punch-in on whoever is speaking is a real cut,
// not an upscale. The speaker track supplies the cuts; the timeline expresses
// them as a playlist of trimmed producers, each with its own framing, so the
// whole edit stays inspectable and re-renderable.

const (
	// keepSingle is the fraction of frame width a single keeps: tight enough to
	// read as a punch-in, wide enough to survive a subject who leans or gestures.
	keepSingle = 0.58
	// faceY is where faces sit vertically in a seated two-person interview.
	faceY = 0.42
	// minShotSec stops the edit ping-ponging on brief interjections.
	minShotSec = 1.8
)

type reframeRequest struct {
	Project string `json:"project"`
	Source  string `json:"source"`
	Quality string `json:"quality"`

	Keep        float64 `json:"keep"`
	MinShot     float64 `json:"minShot"`
	LeftCentre  float64 `json:"leftCentre"`
	RightCentre float64 `json:"rightCentre"`

	RenderWorker     string   `json:"renderWorker"`
	RenderWorkerArgs []string `json:"renderWorkerArgs"`
}

type reframeShot struct {
	Shot  string  `json:"shot"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type reframeResult struct {
	Source   string        `json:"source"`
	Timeline string        `json:"timeline"`
	Shots    []reframeShot `json:"shots"`
	Duration float64       `json:"duration"`
	Output   string        `json:"output,omitempty"`
	Planned  bool          `json:"planned,omitempty"`
}

func reframeCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, _, plan, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	var req reframeRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec reframe-edit <project>) or set "project" in the JSON request`)
	}
	if req.Keep <= 0 {
		req.Keep = keepSingle
	}
	if req.MinShot <= 0 {
		req.MinShot = minShotSec
	}
	if req.Quality == "" {
		req.Quality = "final"
	}
	if req.LeftCentre == 0 {
		req.LeftCentre = defaultLeftCentre
	}
	if req.RightCentre == 0 {
		req.RightCentre = defaultRightCentre
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
	src, err := resolveSource(m, req.Source, "reframe-edit")
	if err != nil {
		return err
	}
	if !proj.HasSpeakers(src.Hash) {
		return axerror.New("speakers.missing", "source has no speaker track to cut on").
			WithHint("run `cutvec speakers` on this source first")
	}
	speakers, err := proj.ReadSpeakers(src.Hash)
	if err != nil {
		return axerror.New("project.read", "cannot read speaker track: "+err.Error()).AsRetryable()
	}
	// Prefer where detection actually found each speaker over the default
	// seating plan, so the punch-ins land on this set rather than an assumed one.
	req.LeftCentre = seatCentre(speakers, "left", req.LeftCentre)
	req.RightCentre = seatCentre(speakers, "right", req.RightCentre)

	// The edit must cover the media, not just the detected turns, so trailing
	// silence still renders. The transcript's last word extends the span when
	// someone speaks after the final attributed turn.
	duration := lastTurnEnd(speakers)
	if proj.HasTranscript(src.Hash) {
		if t, err := proj.ReadTranscript(src.Hash); err == nil && len(t.Words) > 0 {
			if end := t.Words[len(t.Words)-1].End; end > duration {
				duration = end
			}
		}
	}
	if duration <= 0 {
		return axerror.New("speakers.empty", "the speaker track has no turns to cut on").
			WithHint("re-run `cutvec speakers`: an edit needs at least one attributed turn")
	}

	shots := planShots(speakers, duration, req.MinShot)
	profile := timeline.Landscape(30)
	tl := timeline.New()
	tl.Profile = profile
	for i, s := range shots {
		cx, keep := 0.5, 1.0 // the wide two-shot: centred, no punch-in
		switch s.Shot {
		case "left":
			cx, keep = req.LeftCentre, req.Keep
		case "right":
			cx, keep = req.RightCentre, req.Keep
		}
		tl.Producers = append(tl.Producers, timeline.Producer{
			ID:       fmt.Sprintf("shot%d", i),
			Resource: src.Path,
			In:       timeline.Clock(s.Start),
			Out:      timeline.Clock(s.End),
			Filters:  []timeline.Filter{timeline.PunchIn(profile, keep, cx, faceY)},
		})
	}
	tl.Sequence("edit")
	if problems := tl.Validate(); len(problems) > 0 {
		return axerror.New("timeline.invalid",
			fmt.Sprintf("composed edit is not renderable: %v", problems)).
			WithHint("this is a cutvec bug — the recipe built an incomplete timeline")
	}
	xmlBytes, err := tl.Marshal()
	if err != nil {
		return axerror.New("timeline.encode", "cannot encode timeline: "+err.Error())
	}
	tlPath := filepath.Join(proj.CacheDir(), "reframe-"+src.Hash+".mlt")
	if err := os.WriteFile(tlPath, xmlBytes, 0o644); err != nil {
		return axerror.New("project.write", "cannot write timeline: "+err.Error()).AsRetryable()
	}

	out := reframeResult{
		Source: src.Hash, Timeline: tlPath, Shots: shots,
		Duration: round3(duration), Planned: plan,
	}
	if !plan {
		rw, err := resolveMeltWorker(renderRequest{
			Worker: req.RenderWorker, WorkerArgs: req.RenderWorkerArgs,
		})
		if err != nil {
			return err
		}
		output := filepath.Join(proj.CacheDir(), "reframe-"+src.Hash+"-"+req.Quality+".mp4")
		if _, err := rw.Invoke(context.Background(), worker.Request{
			Op:     "render",
			Params: map[string]any{"timeline": tlPath, "output": output, "quality": req.Quality},
		}); err != nil {
			return err
		}
		if _, err := os.Stat(output); err != nil {
			return axerror.New("worker.protocol", "render worker did not write the edit").
				WithHint("the melt worker must write the rendered video to the requested output path").
				AsRetryable()
		}
		out.Output = output
	}
	return emitJSON(stdout, out)
}

// planShots turns attributed turns into a gapless shot list covering the media.
// Unattributed stretches become the wide two-shot — the honest framing when
// nobody is clearly speaking — and shots below minShot are absorbed so a brief
// interjection cannot make the edit ping-pong.
func planShots(s project.Speakers, duration, minShot float64) []reframeShot {
	var raw []reframeShot
	at := 0.0
	for _, t := range s.Turns {
		if t.Start > at {
			raw = append(raw, reframeShot{Shot: "wide", Start: at, End: t.Start})
		}
		end := t.End
		if end > duration {
			end = duration
		}
		if end > t.Start {
			raw = append(raw, reframeShot{Shot: t.Speaker, Start: t.Start, End: end})
			at = end
		}
	}
	if at < duration {
		raw = append(raw, reframeShot{Shot: "wide", Start: at, End: duration})
	}

	// Absorb runt shots into the previous one, then coalesce equal neighbours.
	var kept []reframeShot
	for _, sh := range raw {
		if len(kept) > 0 && sh.End-sh.Start < minShot {
			kept[len(kept)-1].End = sh.End
			continue
		}
		kept = append(kept, sh)
	}
	var out []reframeShot
	for _, sh := range kept {
		if len(out) > 0 && out[len(out)-1].Shot == sh.Shot {
			out[len(out)-1].End = sh.End
			continue
		}
		out = append(out, sh)
	}
	for i := range out {
		out[i].Start = round3(out[i].Start)
		out[i].End = round3(out[i].End)
	}
	return out
}

func lastTurnEnd(s project.Speakers) float64 {
	end := 0.0
	for _, t := range s.Turns {
		if t.End > end {
			end = t.End
		}
	}
	return end
}
