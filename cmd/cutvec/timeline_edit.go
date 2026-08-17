package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/timeline"
)

// A timeline is a declarative, diffable artifact, so the cheapest way to change
// one filter on a composed edit is to load it, mutate it, and write it back —
// not to re-run search and active-speaker detection to rebuild the whole thing.
// `timeline inspect` returns the structure an agent reads to decide what to
// change; `timeline edit` applies that change and re-validates before writing, so
// a mutated timeline is still a timeline melt can render.

// --- inspect ----------------------------------------------------------------

// filterView is one filter on a producer, in the shape an edit targets: the
// mlt_service that names it, its position among the producer's filters (so a
// removal can single out one of several of the same service), and its remaining
// properties in document order.
type filterView struct {
	Service    string         `json:"service"`
	Index      int            `json:"index"`
	Properties []propertyView `json:"properties,omitempty"`
}

type propertyView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// producerView is one media source and the filters attached to it — the unit an
// edit names when it adds or removes a filter.
type producerView struct {
	ID       string       `json:"id"`
	Resource string       `json:"resource,omitempty"`
	In       string       `json:"in,omitempty"`
	Out      string       `json:"out,omitempty"`
	Filters  []filterView `json:"filters"`
}

// timelineInspectResult is the structure `timeline inspect` returns: the handle,
// the producers with their filters, and whether the document validates — so an
// agent can read what is there and whether it would render before editing it.
type timelineInspectResult struct {
	Timeline  string         `json:"timeline"`
	Producers []producerView `json:"producers"`
	Valid     bool           `json:"valid"`
	Problems  []string       `json:"problems,omitempty"`
}

// timelineInspectCmd reads the project's built timeline, parses it back into the
// structured model, and returns its producers and filters as JSON. It is a pure
// read: it never writes, so it has no --plan.
func timelineInspectCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	proj, err := timelineSubProject(args, stdin)
	if err != nil {
		return err
	}
	tl, tlPath, err := loadTimeline(proj)
	if err != nil {
		return err
	}

	result := timelineInspectResult{Timeline: tlPath, Producers: viewProducers(tl)}
	problems := tl.Validate()
	result.Valid = len(problems) == 0
	result.Problems = problems
	return emitJSON(stdout, result)
}

// viewProducers projects the timeline's producers into the inspect view,
// separating each filter's mlt_service from its other properties.
func viewProducers(tl *timeline.Timeline) []producerView {
	out := make([]producerView, 0, len(tl.Producers))
	for _, p := range tl.Producers {
		pv := producerView{ID: p.ID, Resource: p.Resource, In: p.In, Out: p.Out}
		pv.Filters = make([]filterView, 0, len(p.Filters))
		for i, f := range p.Filters {
			fv := filterView{Service: f.Service(), Index: i}
			for _, prop := range f.Properties {
				if prop.Name == "mlt_service" {
					continue
				}
				fv.Properties = append(fv.Properties,
					propertyView{Name: prop.Name, Value: prop.Value})
			}
			pv.Filters = append(pv.Filters, fv)
		}
		out = append(out, pv)
	}
	return out
}

// --- edit -------------------------------------------------------------------

// filterRemoval names a filter to drop: every filter with this mlt_service on the
// named producer. A removal that matches nothing is an error, not a silent no-op,
// so a typo'd service or producer is caught rather than leaving the edit intact.
type filterRemoval struct {
	Producer string `json:"producer"`
	Service  string `json:"service"`
}

// timelineEditRequest is the mutation applied to an existing timeline: filters to
// remove and effects to add, both targeting named producers. Removals are applied
// before additions, so replacing a filter is remove-then-add in one call.
type timelineEditRequest struct {
	Project string          `json:"project"`
	Remove  []filterRemoval `json:"remove"`
	Add     []effectSpec    `json:"add"`
}

// timelineEditResult reports what the edit changed: filters removed and added,
// the producers touched, and whether the mutated document still validates — the
// latter proving the edit left a timeline melt can render.
type timelineEditResult struct {
	Timeline  string   `json:"timeline"`
	Removed   int      `json:"removed"`
	Added     int      `json:"added"`
	Producers []string `json:"producers"`
	Valid     bool     `json:"valid"`
	Planned   bool     `json:"planned,omitempty"`
}

// timelineEditCmd mutates the project's built timeline in place: it loads the MLT
// document, applies the requested removals and additions to named producers,
// re-validates the result, and only then writes it back. An edit that would leave
// an invalid timeline fails without touching the file, so a bad mutation never
// costs a render. --plan reports the change without writing.
func timelineEditCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	plan := false
	var positionals []string
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --plan (dry-run) and --json")
		default:
			positionals = append(positionals, a)
		}
	}
	if len(positionals) > 1 {
		return axerror.New("input.invalid", "timeline edit takes at most one project argument").
			WithHint("name one project, e.g. `cutvec timeline edit ./proj`")
	}

	var req timelineEditRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if len(positionals) == 1 {
		req.Project = positionals[0]
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (cutvec timeline edit <project>) or set "project" in the JSON request`)
	}
	if len(req.Remove) == 0 && len(req.Add) == 0 {
		return axerror.New("input.empty", "edit changes nothing: no filters to add or remove").
			WithHint(`set "remove" (producer+service) and/or "add" (an effect) to change the timeline`)
	}

	proj := project.Open(req.Project)
	tl, tlPath, err := loadTimeline(proj)
	if err != nil {
		return err
	}

	touched := map[string]bool{}

	// Removals first, so a remove-then-add on the same service reads as a replace.
	removed := 0
	for i, r := range req.Remove {
		if r.Producer == "" || r.Service == "" {
			return axerror.New("input.invalid",
				fmt.Sprintf("removal %d needs both a producer and a service", i)).
				WithHint(`each removal is {"producer":"...","service":"..."}`)
		}
		p := tl.Producer(r.Producer)
		if p == nil {
			return unknownProducer(tl, r.Producer)
		}
		n := p.RemoveFilterService(r.Service)
		if n == 0 {
			return axerror.New("filter.not_found",
				fmt.Sprintf("producer %q has no %q filter to remove", r.Producer, r.Service)).
				WithHint("run `cutvec timeline inspect <project>` to see each producer's filters")
		}
		removed += n
		touched[r.Producer] = true
	}

	// Additions reuse the same effect resolution the build path uses, so an edit
	// can attach any preset or MLT service exactly as `timeline` does.
	byProducer, err := resolveEffects(req.Add)
	if err != nil {
		return err
	}
	added := 0
	for id, filters := range byProducer {
		added += len(filters)
		if id != "" {
			touched[id] = true
		}
	}
	if _, all := byProducer[""]; all {
		for _, p := range tl.Producers {
			touched[p.ID] = true
		}
	}
	if err := applyEffects(tl, byProducer); err != nil {
		return err
	}

	// Re-validate before writing: the whole point of edit is that a mutated
	// timeline is still one melt can render, so an edit that breaks that fails
	// without leaving a half-broken document on disk.
	if problems := tl.Validate(); len(problems) > 0 {
		return axerror.New("timeline.invalid",
			"edit would make the timeline invalid: "+strings.Join(problems, "; ")).
			WithHint("the timeline was left unchanged; inspect it and adjust the edit")
	}

	result := timelineEditResult{
		Timeline:  tlPath,
		Removed:   removed,
		Added:     added,
		Producers: sortedKeys(touched),
		Valid:     true,
	}
	if plan {
		result.Planned = true
		return emitJSON(stdout, result)
	}

	xmlBytes, err := tl.Marshal()
	if err != nil {
		return axerror.New("timeline.marshal", "failed to render MLT XML: "+err.Error())
	}
	if err := os.WriteFile(tlPath, xmlBytes, 0o644); err != nil {
		return axerror.New("project.write", "cannot write timeline: "+err.Error()).AsRetryable()
	}
	return emitJSON(stdout, result)
}

// --- shared helpers ---------------------------------------------------------

// timelineSubProject reads the project for the read-only `inspect` subcommand
// from a single positional argument or the JSON request's "project".
func timelineSubProject(args []string, stdin io.Reader) (project.Project, error) {
	var positionals []string
	for _, a := range args {
		switch {
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return project.Project{}, axerror.New("cli.unknown_flag",
				fmt.Sprintf("unknown flag %q", a)).WithHint("the only flag is --json")
		default:
			positionals = append(positionals, a)
		}
	}
	if len(positionals) > 1 {
		return project.Project{}, axerror.New("input.invalid",
			"this command takes at most one project argument").
			WithHint("name one project directory")
	}
	var req struct {
		Project string `json:"project"`
	}
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return project.Project{}, err
	}
	if len(positionals) == 1 {
		req.Project = positionals[0]
	}
	if req.Project == "" {
		return project.Project{}, axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument or set "project" in the JSON request`)
	}
	return project.Open(req.Project), nil
}

// loadTimeline reads and parses the project's built timeline, mapping a missing
// file to the same timeline.missing render reports, so an agent gets one code for
// "there is no timeline yet" across the commands that need one.
func loadTimeline(proj project.Project) (*timeline.Timeline, string, error) {
	tlPath := proj.TimelinePath()
	data, err := os.ReadFile(tlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", axerror.New("timeline.missing", "project has no built timeline").
				WithHint("run `cutvec timeline` to build a timeline first")
		}
		return nil, "", axerror.New("project.read", "cannot read timeline: "+err.Error()).AsRetryable()
	}
	tl, err := timeline.Parse(data)
	if err != nil {
		return nil, "", axerror.New("timeline.parse", "timeline is not valid MLT XML: "+err.Error()).
			WithHint("the timeline file is corrupt; rebuild it with `cutvec timeline`")
	}
	return tl, tlPath, nil
}

// unknownProducer builds the error naming which producers do exist, so an edit
// against a mistyped id fails with an actionable list rather than silently.
func unknownProducer(tl *timeline.Timeline, id string) error {
	ids := make([]string, 0, len(tl.Producers))
	for _, p := range tl.Producers {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	return axerror.New("effect.unknown_producer",
		fmt.Sprintf("no producer %q on this timeline", id)).
		WithHint("producers on this timeline: " + strings.Join(ids, ", "))
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
