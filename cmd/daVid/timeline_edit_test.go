package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mlnomadpy/daVid/internal/axerror"
)

// buildTimelineFixture inits a project and builds a two-clip timeline with a crop
// on the first clip — the composed edit an inspect/edit test starts from.
func buildTimelineFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)
	runJSON(t, []string{"timeline"}, map[string]any{
		"project": dir,
		"clips": []map[string]any{
			{"id": "cam1", "resource": "raw.mp4"},
			{"id": "broll", "resource": "broll.mp4"},
		},
		"effects": []map[string]any{
			{"service": "crop", "producer": "cam1", "properties": map[string]any{"left": "10"}},
		},
	}, nil)
	return dir
}

type inspectReply struct {
	Timeline  string `json:"timeline"`
	Producers []struct {
		ID       string `json:"id"`
		Resource string `json:"resource"`
		Filters  []struct {
			Service    string `json:"service"`
			Index      int    `json:"index"`
			Properties []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"properties"`
		} `json:"filters"`
	} `json:"producers"`
	Valid    bool     `json:"valid"`
	Problems []string `json:"problems"`
}

type editReply struct {
	Timeline  string   `json:"timeline"`
	Removed   int      `json:"removed"`
	Added     int      `json:"added"`
	Producers []string `json:"producers"`
	Valid     bool     `json:"valid"`
	Planned   bool     `json:"planned"`
}

// TestTimelineInspectReturnsStructure is acceptance #1 (inspect half): inspect
// returns the producers and the filters attached to each — the structure an agent
// reads to decide what to change.
func TestTimelineInspectReturnsStructure(t *testing.T) {
	dir := buildTimelineFixture(t)

	var res inspectReply
	runJSON(t, []string{"timeline", "inspect", dir}, nil, &res)

	if len(res.Producers) != 2 {
		t.Fatalf("producers = %d, want 2", len(res.Producers))
	}
	if !res.Valid {
		t.Fatalf("valid = false, problems = %v", res.Problems)
	}
	cam1 := res.Producers[0]
	if cam1.ID != "cam1" || cam1.Resource != "raw.mp4" {
		t.Fatalf("first producer = %+v, want cam1/raw.mp4", cam1)
	}
	if len(cam1.Filters) != 1 || cam1.Filters[0].Service != "crop" {
		t.Fatalf("cam1 filters = %+v, want one crop", cam1.Filters)
	}
	// The service is lifted out of the properties, and the rest survive.
	var left string
	for _, p := range cam1.Filters[0].Properties {
		if p.Name == "left" {
			left = p.Value
		}
		if p.Name == "mlt_service" {
			t.Error("mlt_service leaked into properties; it should be the `service` field")
		}
	}
	if left != "10" {
		t.Fatalf("crop left = %q, want 10", left)
	}
	if res.Producers[1].ID != "broll" || len(res.Producers[1].Filters) != 0 {
		t.Fatalf("second producer = %+v, want broll with no filters", res.Producers[1])
	}
}

// TestTimelineEditRemovesAndAddsFilters is acceptance #1 (edit half): edit adds
// and removes a filter on a named producer, and reports the counts.
func TestTimelineEditRemovesAndAddsFilters(t *testing.T) {
	dir := buildTimelineFixture(t)

	var res editReply
	runJSON(t, []string{"timeline", "edit", dir}, map[string]any{
		"remove": []map[string]any{{"producer": "cam1", "service": "crop"}},
		"add": []map[string]any{
			{"service": "greyscale", "producer": "broll"},
		},
	}, &res)

	if res.Removed != 1 {
		t.Fatalf("removed = %d, want 1", res.Removed)
	}
	if res.Added != 1 {
		t.Fatalf("added = %d, want 1", res.Added)
	}
	if !res.Valid {
		t.Fatal("valid = false, want the edited timeline to still validate")
	}
	// The change is durable: re-inspecting shows cam1 with no crop and broll with
	// the new greyscale.
	var back inspectReply
	runJSON(t, []string{"timeline", "inspect", dir}, nil, &back)
	for _, p := range back.Producers {
		switch p.ID {
		case "cam1":
			if len(p.Filters) != 0 {
				t.Fatalf("cam1 still has filters %+v after removal", p.Filters)
			}
		case "broll":
			if len(p.Filters) != 1 || p.Filters[0].Service != "greyscale" {
				t.Fatalf("broll filters = %+v, want one greyscale", p.Filters)
			}
		}
	}
}

// TestTimelineEditPresetAppliesToAllProducers checks a preset add with no
// producer lands on every clip, exercising the shared effect-resolution path.
func TestTimelineEditPresetAppliesToAllProducers(t *testing.T) {
	dir := buildTimelineFixture(t)

	var res editReply
	runJSON(t, []string{"timeline", "edit", dir}, map[string]any{
		"add": []map[string]any{{"preset": "warm"}},
	}, &res)
	if res.Added != 1 {
		t.Fatalf("added = %d, want 1 (the warm preset is a single filter)", res.Added)
	}
	// warm applied to all → both producers named as touched.
	if len(res.Producers) != 2 {
		t.Fatalf("touched producers = %v, want both", res.Producers)
	}
}

// TestTimelineEditPlanWritesNothing asserts --plan reports the change but leaves
// the timeline on disk unchanged.
func TestTimelineEditPlanWritesNothing(t *testing.T) {
	dir := buildTimelineFixture(t)
	tlPath := filepath.Join(dir, "timeline.mlt")
	before, err := os.ReadFile(tlPath)
	if err != nil {
		t.Fatal(err)
	}

	var res editReply
	runJSON(t, []string{"timeline", "edit", dir, "--plan"}, map[string]any{
		"remove": []map[string]any{{"producer": "cam1", "service": "crop"}},
	}, &res)
	if !res.Planned || res.Removed != 1 {
		t.Fatalf("plan result = %+v, want planned:true removed:1", res)
	}
	after, err := os.ReadFile(tlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("--plan changed the timeline on disk, want it untouched")
	}
}

// TestTimelineEditThenRenders is acceptance #2: an edited timeline still renders.
// It edits the composed timeline and drives render through the fake melt worker —
// the same pipeline a real render takes — proving the mutated document is one
// melt can consume.
func TestTimelineEditThenRenders(t *testing.T) {
	dir := buildTimelineFixture(t)

	// Mutate: drop the crop, add a grade preset across the edit.
	runJSON(t, []string{"timeline", "edit", dir}, map[string]any{
		"remove": []map[string]any{{"producer": "cam1", "service": "crop"}},
		"add":    []map[string]any{{"preset": "grade"}},
	}, nil)

	wpath, wargs := renderWorkerEnv(t, "render")
	var res renderReply
	runJSON(t, []string{"render", dir}, map[string]any{"worker": wpath, "workerArgs": wargs}, &res)
	if res.Cached {
		t.Fatal("first render of the edited timeline cached = true, want a fresh render")
	}
	if res.Output == "" {
		t.Fatal("render of the edited timeline returned no output handle")
	}
	if _, err := os.ReadFile(res.Output); err != nil {
		t.Fatalf("edited timeline did not render to a file: %v", err)
	}
}

// TestTimelineEditFailureRules pins the failure and empty-input rules edit owes
// the AX contract: each bad request maps to its stable code and never writes.
func TestTimelineEditFailureRules(t *testing.T) {
	dir := buildTimelineFixture(t)
	cases := []struct {
		name string
		args []string
		req  string
		code string
	}{
		{"no project", []string{"timeline", "edit"}, `{"remove":[{"producer":"cam1","service":"crop"}]}`, "input.invalid"},
		{"no change", []string{"timeline", "edit", dir}, `{}`, "input.empty"},
		{"unknown producer", []string{"timeline", "edit", dir}, `{"remove":[{"producer":"ghost","service":"crop"}]}`, "effect.unknown_producer"},
		{"filter not there", []string{"timeline", "edit", dir}, `{"remove":[{"producer":"broll","service":"crop"}]}`, "filter.not_found"},
		{"removal missing service", []string{"timeline", "edit", dir}, `{"remove":[{"producer":"cam1"}]}`, "input.invalid"},
		{"unknown add producer", []string{"timeline", "edit", dir}, `{"add":[{"service":"greyscale","producer":"ghost"}]}`, "effect.unknown_producer"},
		{"unknown flag", []string{"timeline", "edit", dir, "--nope"}, `{"remove":[{"producer":"cam1","service":"crop"}]}`, "cli.unknown_flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, strings.NewReader(tc.req), io.Discard)
			var ae *axerror.Error
			if !errors.As(err, &ae) {
				t.Fatalf("err = %v, want *axerror.Error", err)
			}
			if ae.Code != tc.code {
				t.Fatalf("code = %q, want %q", ae.Code, tc.code)
			}
		})
	}
	// A failed edit never mutates the file: the fixture's crop is still there.
	var back inspectReply
	runJSON(t, []string{"timeline", "inspect", dir}, nil, &back)
	if back.Producers[0].Filters[0].Service != "crop" {
		t.Fatal("a failed edit mutated the timeline; the crop should be intact")
	}
}

// TestTimelineInspectMissingTimeline maps an inspect on a project with no built
// timeline to timeline.missing, the same code render uses.
func TestTimelineInspectMissingTimeline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	runJSON(t, []string{"init"}, map[string]any{"project": dir}, nil)

	err := run([]string{"timeline", "inspect", dir}, strings.NewReader(""), io.Discard)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *axerror.Error", err)
	}
	if ae.Code != "timeline.missing" {
		t.Fatalf("code = %q, want timeline.missing", ae.Code)
	}
}

// TestTimelineBuildStillWorks guards the historic no-subcommand behaviour: a bare
// `timeline` request still builds from clips, unshadowed by the new subcommands.
func TestTimelineBuildStillWorks(t *testing.T) {
	dir := t.TempDir()
	var res struct {
		Producers int `json:"producers"`
	}
	runJSON(t, []string{"timeline"}, map[string]any{
		"project": dir,
		"clips":   []map[string]any{{"resource": "raw.mp4"}},
	}, &res)
	if res.Producers != 1 {
		t.Fatalf("producers = %d, want 1", res.Producers)
	}
	if _, err := os.Stat(filepath.Join(dir, "timeline.mlt")); err != nil {
		t.Fatalf("build wrote no timeline: %v", err)
	}
}

// TestTimelineEditViaMCPOp proves the MCP surface reaches the edit subcommand
// through the "op" field, so an agent gets inspect/edit over JSON-RPC too.
func TestTimelineEditViaMCPOp(t *testing.T) {
	dir := buildTimelineFixture(t)
	args := mustMarshal(t, map[string]any{
		"op":      "edit",
		"project": dir,
		"remove":  []map[string]any{{"producer": "cam1", "service": "crop"}},
	})
	res := callTool("timeline", args)
	if res.IsError {
		t.Fatalf("callTool timeline op=edit errored: %s", res.Content[0].Text)
	}
	var reply editReply
	if err := json.Unmarshal([]byte(res.Content[0].Text), &reply); err != nil {
		t.Fatalf("edit result is not JSON: %v\n%s", err, res.Content[0].Text)
	}
	if reply.Removed != 1 {
		t.Fatalf("removed = %d, want 1 via MCP op", reply.Removed)
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
