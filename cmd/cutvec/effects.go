package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
	"github.com/tahabsn/cutvec/internal/worker"
)

// Effects are how cutvec stays as capable as MLT itself. Rather than wrapping a
// handful of chosen operations, a timeline can carry any MLT filter by service
// name and properties — several hundred of them, including the whole avfilter
// set — and `filters` lets an agent discover that registry at runtime instead of
// guessing what this tool happens to support.
//
// Presets exist for the opposite reason: the common grades are a specific chain
// with specific numbers, and an agent should not have to rediscover them. A
// preset is only a named list of MLT filters, so anything a preset does can also
// be written out longhand and tweaked.

// effectSpec is one filter to attach: either a named preset, or an MLT service
// with its properties.
type effectSpec struct {
	Preset     string            `json:"preset"`
	Service    string            `json:"service"`
	Properties map[string]string `json:"properties"`
	// Producer limits the effect to one producer id; empty applies it to all.
	Producer string `json:"producer"`
}

// presets are named MLT filter chains for the operations a talking-head edit
// needs most. The grade values are the ones that worked on real backlit
// interview footage: the curve pulls a blown background back from pure white
// and lifts faces out of it, rather than raising exposure globally.
var presets = map[string][]timeline.Filter{
	"grade": {
		timeline.NewFilter("avfilter.curves",
			"av.master", "0/0.03 0.25/0.33 0.55/0.60 0.85/0.83 1/0.92"),
		timeline.NewFilter("avfilter.eq", "av.contrast", "1.03", "av.saturation", "1.14"),
		timeline.NewFilter("avfilter.unsharp",
			"av.luma_msize_x", "5", "av.luma_msize_y", "5", "av.luma_amount", "0.6"),
	},
	"denoise": {
		// A high-pass first removes rumble the spectral denoiser would otherwise
		// spend its budget on.
		timeline.NewFilter("avfilter.highpass", "av.f", "85"),
		timeline.NewFilter("avfilter.afftdn", "av.nr", "12", "av.nf", "-25"),
	},
	"denoise-strong": {
		timeline.NewFilter("avfilter.highpass", "av.f", "90"),
		timeline.NewFilter("avfilter.afftdn", "av.nr", "24", "av.nf", "-20"),
		timeline.NewFilter("avfilter.dynaudnorm", "av.g", "7"),
	},
	"loudness": {
		// -14 LUFS is the level the streaming platforms normalise to, so
		// delivering at it avoids being turned down on playback.
		timeline.NewFilter("avfilter.loudnorm", "av.I", "-14", "av.TP", "-1.0"),
	},
	"warm": {
		timeline.NewFilter("lift_gamma_gain",
			"gain_r", "1.03", "gain_b", "0.97", "gamma_g", "1.01"),
	},
}

// presetNames lists the presets in a stable order for error hints and docs.
func presetNames() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolveEffects turns the requested effects into MLT filters, keyed by the
// producer id they apply to ("" meaning every producer).
func resolveEffects(specs []effectSpec) (map[string][]timeline.Filter, error) {
	out := map[string][]timeline.Filter{}
	for i, spec := range specs {
		var filters []timeline.Filter
		switch {
		case spec.Preset != "" && spec.Service != "":
			return nil, axerror.New("input.invalid",
				fmt.Sprintf("effect %d sets both a preset and a service", i)).
				WithHint("an effect is either a named preset or one MLT service, not both")

		case spec.Preset != "":
			chain, ok := presets[spec.Preset]
			if !ok {
				return nil, axerror.New("effect.unknown_preset",
					fmt.Sprintf("no preset named %q", spec.Preset)).
					WithHint("known presets: " + strings.Join(presetNames(), ", "))
			}
			// Copy: a preset is shared state and must not be mutated per use.
			filters = append(filters, chain...)

		case spec.Service != "":
			f := timeline.NewFilter(spec.Service)
			// Sorted so the same request always produces the same XML — a
			// timeline that changes shape between identical runs is not
			// diffable, and breaks content-hash caching.
			keys := make([]string, 0, len(spec.Properties))
			for k := range spec.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				f.Properties = append(f.Properties,
					timeline.Property{Name: k, Value: spec.Properties[k]})
			}
			filters = append(filters, f)

		default:
			return nil, axerror.New("input.invalid",
				fmt.Sprintf("effect %d names neither a preset nor a service", i)).
				WithHint(`set "preset" (` + strings.Join(presetNames(), ", ") +
					`) or "service" to an MLT filter name — run ` + "`cutvec filters`" +
					` to discover them`)
		}
		out[spec.Producer] = append(out[spec.Producer], filters...)
	}
	return out, nil
}

// applyEffects attaches resolved effects to a timeline's producers.
func applyEffects(tl *timeline.Timeline, byProducer map[string][]timeline.Filter) error {
	known := map[string]bool{}
	for _, p := range tl.Producers {
		known[p.ID] = true
	}
	for id := range byProducer {
		if id != "" && !known[id] {
			ids := make([]string, 0, len(known))
			for k := range known {
				ids = append(ids, k)
			}
			sort.Strings(ids)
			return axerror.New("effect.unknown_producer",
				fmt.Sprintf("no producer %q on this timeline", id)).
				WithHint("producers on this timeline: " + strings.Join(ids, ", "))
		}
	}
	for i := range tl.Producers {
		tl.Producers[i].Filters = append(tl.Producers[i].Filters, byProducer[""]...)
		if named := byProducer[tl.Producers[i].ID]; len(named) > 0 {
			tl.Producers[i].Filters = append(tl.Producers[i].Filters, named...)
		}
	}
	return nil
}

// --- seat framing -----------------------------------------------------------

// seatCentre answers where to aim the frame for a speaker. Detection is trusted
// when it reported a position: an assumed seating plan is right for one set and
// wrong for every other, whereas the detector measured this one. The supplied
// fallback is used when a seat was not reliably detected, and an implausible
// centre is rejected rather than pointing the camera off the subject.
func seatCentre(s project.Speakers, seat string, fallback float64) float64 {
	pos, ok := s.Seats[seat]
	if !ok {
		return fallback
	}
	// A face detected hard against a frame edge is far more likely to be a
	// mis-detection than a real seating position.
	if pos.CX < 0.05 || pos.CX > 0.95 {
		return fallback
	}
	return pos.CX
}

// --- audio beds -------------------------------------------------------------

// audioSpec is a sound laid under the edit: a music bed, a sting, room tone.
type audioSpec struct {
	Resource string  `json:"resource"`
	ID       string  `json:"id"`
	Gain     float64 `json:"gain"`
	Start    float64 `json:"start"`
	FadeIn   float64 `json:"fadeIn"`
	FadeOut  float64 `json:"fadeOut"`
	// Effects apply to this sound alone — ducking or EQ on the bed without
	// touching the dialogue.
	Effects []effectSpec `json:"effects"`
}

// applyAudio lays each requested sound under the timeline as a mixed track.
func applyAudio(tl *timeline.Timeline, specs []audioSpec) error {
	for i, spec := range specs {
		if spec.Resource == "" {
			return axerror.New("input.invalid",
				fmt.Sprintf("audio %d has no resource", i)).
				WithHint("every audio entry needs a resource path to a sound file")
		}
		if spec.Gain < 0 || spec.Gain > 1 {
			return axerror.New("input.invalid",
				fmt.Sprintf("audio %d has gain %v", i, spec.Gain)).
				WithHint("gain is a level between 0 and 1; 0.15 is a typical music bed under dialogue")
		}
		id := spec.ID
		if id == "" {
			id = fmt.Sprintf("audio%d", i)
		}
		byProducer, err := resolveEffects(spec.Effects)
		if err != nil {
			return err
		}
		var own []timeline.Filter
		for _, filters := range byProducer {
			own = append(own, filters...)
		}
		tl.AddAudio(id, spec.Resource, spec.Gain, spec.Start, spec.FadeIn, spec.FadeOut, own...)
	}
	return nil
}

// --- the filters command ----------------------------------------------------

type filtersRequest struct {
	Query      string   `json:"query"`
	Name       string   `json:"name"`
	Limit      int      `json:"limit"`
	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

type filtersResult struct {
	Filters []any    `json:"filters"`
	Count   int      `json:"count"`
	Total   int      `json:"total"`
	Presets []string `json:"presets"`
}

// filtersCmd reports what MLT can actually do on this machine. It is deliberately
// a query against the installed melt rather than a hardcoded list, because the
// answer depends on how MLT was built and which plugins are present.
func filtersCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posQuery := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("pass a search term as an argument, e.g. `cutvec filters denoise`")
		}
		if posQuery != "" {
			return axerror.New("input.invalid", "filters takes at most one search term").
				WithHint("quote a multi-word term, or search for one keyword")
		}
		posQuery = a
	}
	var req filtersRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posQuery != "" {
		req.Query = posQuery
	}

	w, err := resolveMeltWorker(renderRequest{Worker: req.Worker, WorkerArgs: req.WorkerArgs})
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op: "filters",
		Params: map[string]any{
			"query": req.Query, "name": req.Name, "limit": req.Limit,
		},
	})
	if err != nil {
		return err
	}

	out := filtersResult{Presets: presetNames()}
	if list, ok := resp.Result["filters"].([]any); ok {
		out.Filters = list
	}
	if n, ok := resp.Result["count"].(float64); ok {
		out.Count = int(n)
	}
	if n, ok := resp.Result["total"].(float64); ok {
		out.Total = int(n)
	}
	return emitJSON(stdout, out)
}
