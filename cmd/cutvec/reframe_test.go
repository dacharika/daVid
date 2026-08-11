package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
)

// reframeFixture ingests a source and gives it a speaker track, which is the
// state reframe-edit requires: it cuts on detected turns, so a project without
// them has nothing to cut.
func reframeFixture(t *testing.T, turns []project.Turn) (dir, hash string) {
	t.Helper()
	dir, _ = ingestFixture(t, "reframe source bytes")
	proj := project.Open(dir)
	m, err := proj.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("fixture has %d sources, want 1", len(m.Sources))
	}
	hash = m.Sources[0].Hash
	if err := proj.WriteSpeakers(hash, project.Speakers{
		Source: hash,
		Layout: project.TwoPersonLayout,
		Turns:  turns,
		Seats: map[string]project.Seat{
			"left":  {CX: 0.30, CY: 0.35, Detections: 100},
			"right": {CX: 0.70, CY: 0.35, Detections: 100},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return dir, hash
}

// The recipe end to end under --plan: it must compose a gapless shot list from
// the turns, write the timeline, and render nothing. Until now reframeCmd had no
// test at all — every other command sits at 70-92% — so the whole path from
// request to composed edit was only ever checked by hand.
func TestReframeEditPlansAGaplessEditWithoutRendering(t *testing.T) {
	dir, _ := reframeFixture(t, []project.Turn{
		{Speaker: "left", Start: 0, End: 4},
		{Speaker: "right", Start: 4, End: 9},
	})

	var out bytes.Buffer
	if err := run([]string{"reframe-edit", dir, "--plan"}, strings.NewReader("{}"), &out); err != nil {
		t.Fatalf("reframe-edit --plan = %v, want nil", err)
	}
	var res struct {
		Timeline string  `json:"timeline"`
		Duration float64 `json:"duration"`
		Output   string  `json:"output"`
		Planned  bool    `json:"planned"`
		Shots    []struct {
			Shot  string  `json:"shot"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"shots"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if !res.Planned {
		t.Fatalf("planned = false under --plan: %s", out.String())
	}
	if res.Output != "" {
		t.Errorf("--plan reported a rendered output %q; it must render nothing", res.Output)
	}
	if len(res.Shots) == 0 {
		t.Fatalf("no shots planned: %s", out.String())
	}
	// The shot list must cover the media with no holes — a gap between shots is
	// a gap in the rendered video.
	if res.Shots[0].Start != 0 {
		t.Errorf("first shot starts at %.3f, want 0", res.Shots[0].Start)
	}
	for i := 1; i < len(res.Shots); i++ {
		if res.Shots[i].Start != res.Shots[i-1].End {
			t.Errorf("hole between shot %d and %d: %.3f != %.3f",
				i-1, i, res.Shots[i-1].End, res.Shots[i].Start)
		}
	}
	if last := res.Shots[len(res.Shots)-1].End; last != res.Duration {
		t.Errorf("shots end at %.3f but duration is %.3f — the tail would not render", last, res.Duration)
	}
	// The timeline is a handle, and --plan still writes it so it can be inspected
	// before committing to a render.
	if res.Timeline == "" {
		t.Error("no timeline handle returned")
	}
}

// The render half: with a melt worker configured, the recipe renders and returns
// a handle to a real file.
func TestReframeEditRendersAndReturnsAHandle(t *testing.T) {
	dir, _ := reframeFixture(t, []project.Turn{
		{Speaker: "left", Start: 0, End: 3},
		{Speaker: "right", Start: 3, End: 6},
	})
	wpath, wargs := renderWorkerEnv(t, "render")
	body, _ := json.Marshal(map[string]any{
		"renderWorker": wpath, "renderWorkerArgs": wargs, "quality": "proxy",
	})

	var out bytes.Buffer
	if err := run([]string{"reframe-edit", dir}, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("reframe-edit = %v, want nil", err)
	}
	var res struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.Output == "" {
		t.Fatalf("no output handle: %s", out.String())
	}
	// A handle pointing at nothing is worse than no handle.
	if _, err := os.Stat(res.Output); err != nil {
		t.Fatalf("output handle %q does not exist: %v", res.Output, err)
	}
}

// Each refusal is a branchable code an agent can act on, so each is pinned.
func TestReframeEditFailureRules(t *testing.T) {
	t.Run("no speaker track", func(t *testing.T) {
		// Ingested but never analysed: the recipe has nothing to cut on.
		dir, _ := ingestFixture(t, "no speakers here")
		var out bytes.Buffer
		err := run([]string{"reframe-edit", dir}, strings.NewReader("{}"), &out)
		assertReframeCode(t, err, "speakers.missing")
	})

	t.Run("speaker track with no turns", func(t *testing.T) {
		// Detection ran but attributed nothing — distinct from never running,
		// and it needs its own code or an agent cannot tell them apart.
		dir, _ := reframeFixture(t, nil)
		var out bytes.Buffer
		err := run([]string{"reframe-edit", dir}, strings.NewReader("{}"), &out)
		assertReframeCode(t, err, "speakers.empty")
	})

	t.Run("uninitialised project", func(t *testing.T) {
		var out bytes.Buffer
		err := run([]string{"reframe-edit", t.TempDir()}, strings.NewReader("{}"), &out)
		assertReframeCode(t, err, "project.uninitialized")
	})

	t.Run("no project named", func(t *testing.T) {
		var out bytes.Buffer
		err := run([]string{"reframe-edit"}, strings.NewReader("{}"), &out)
		assertReframeCode(t, err, "input.invalid")
	})
}

func assertReframeCode(t *testing.T, err error, want string) {
	t.Helper()
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error = %v, want a structured *axerror.Error with code %q", err, want)
	}
	if ae.Code != want {
		t.Fatalf("code = %q, want %q", ae.Code, want)
	}
	if ae.Hint == "" {
		t.Errorf("code %q carries no hint, so an agent cannot recover from it", ae.Code)
	}
}

// The edit must cover the whole media with no holes: a gap between shots is a
// gap in the rendered video.
func TestPlanShotsCoversTheMediaWithoutGaps(t *testing.T) {
	s := project.Speakers{Turns: []project.Turn{
		{Speaker: "right", Start: 4, End: 9},
		{Speaker: "left", Start: 12, End: 30},
	}}
	shots := planShots(s, 40, 1.8)
	if len(shots) == 0 {
		t.Fatal("planShots produced no shots")
	}
	if shots[0].Start != 0 {
		t.Fatalf("edit starts at %v, want 0", shots[0].Start)
	}
	if last := shots[len(shots)-1].End; last != 40 {
		t.Fatalf("edit ends at %v, want the full 40s duration", last)
	}
	for i := 1; i < len(shots); i++ {
		if shots[i].Start != shots[i-1].End {
			t.Fatalf("gap between shot %d (ends %v) and %d (starts %v)",
				i-1, shots[i-1].End, i, shots[i].Start)
		}
	}
}

// Unattributed stretches are the wide two-shot: the honest framing when nobody
// is clearly speaking.
func TestPlanShotsUsesWideWhenNobodyIsAttributed(t *testing.T) {
	s := project.Speakers{Turns: []project.Turn{{Speaker: "left", Start: 10, End: 20}}}
	shots := planShots(s, 30, 1.8)
	if shots[0].Shot != "wide" {
		t.Fatalf("opening shot is %q, want wide before the first turn", shots[0].Shot)
	}
	if shots[len(shots)-1].Shot != "wide" {
		t.Fatalf("closing shot is %q, want wide after the last turn", shots[len(shots)-1].Shot)
	}
}

// A brief interjection must not make the edit ping-pong, so shots under the
// minimum are absorbed and equal neighbours coalesce.
func TestPlanShotsAbsorbsRuntShotsAndCoalesces(t *testing.T) {
	s := project.Speakers{Turns: []project.Turn{
		{Speaker: "left", Start: 0, End: 10},
		{Speaker: "right", Start: 10, End: 10.4}, // a 0.4s interjection
		{Speaker: "left", Start: 10.4, End: 20},
	}}
	shots := planShots(s, 20, 1.8)
	for _, sh := range shots {
		if d := sh.End - sh.Start; d < 1.8 {
			t.Fatalf("shot %+v is %vs, shorter than the 1.8s minimum", sh, d)
		}
	}
	if len(shots) != 1 || shots[0].Shot != "left" {
		t.Fatalf("shots = %+v, want one coalesced left shot", shots)
	}
}

// A punch-in on a subject near the frame edge must stay inside the frame, or
// the crop is invalid and melt renders nothing usable.
func TestPunchInClampsInsideTheFrame(t *testing.T) {
	p := timeline.Landscape(30)
	for _, cx := range []float64{0.0, 0.05, 0.5, 0.95, 1.0} {
		f := timeline.PunchIn(p, 0.58, cx, 0.42)
		vals := map[string]string{}
		for _, prop := range f.Properties {
			vals[prop.Name] = prop.Value
		}
		for _, side := range []string{"left", "right", "top", "bottom"} {
			if strings.HasPrefix(vals[side], "-") {
				t.Fatalf("cx=%v produced a negative %s crop (%s): the window left the frame",
					cx, side, vals[side])
			}
		}
	}
}

// keep=1 is the untouched wide shot: it must crop nothing at all.
func TestPunchInWithFullKeepCropsNothing(t *testing.T) {
	f := timeline.PunchIn(timeline.Landscape(30), 1.0, 0.5, 0.5)
	for _, prop := range f.Properties {
		switch prop.Name {
		case "left", "right", "top", "bottom":
			if prop.Value != "0" {
				t.Fatalf("%s = %q, want 0 for an untouched wide shot", prop.Name, prop.Value)
			}
		}
	}
}

// Sequence is what makes a multi-producer timeline play as one edit rather than
// rendering only the first clip.
func TestSequenceWiresEveryProducerIntoAPlaylist(t *testing.T) {
	tl := timeline.New()
	tl.Profile = timeline.Landscape(30)
	for _, id := range []string{"a", "b", "c"} {
		tl.Producers = append(tl.Producers, timeline.Producer{ID: id, Resource: "/m.mp4"})
	}
	tl.Sequence("edit")

	if len(tl.Playlists) != 1 || len(tl.Playlists[0].Entries) != 3 {
		t.Fatalf("playlist = %+v, want one playlist with 3 entries", tl.Playlists)
	}
	if tl.Tractor == nil || len(tl.Tractor.Tracks) != 1 ||
		tl.Tractor.Tracks[0].Producer != "edit" {
		t.Fatalf("tractor = %+v, want one track playing the playlist", tl.Tractor)
	}
	xml, err := tl.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"<playlist", "<entry", "<tractor", "<track"} {
		if !strings.Contains(string(xml), want) {
			t.Fatalf("MLT XML missing %q:\n%s", want, xml)
		}
	}
}

// whisper is asked for one segment per word to get word-level timing, which
// splits punctuation and contractions into their own tokens. Joined naively
// they read as "CETERA . I 'VE" on screen.
func TestJoinTokensReassemblesWhisperWordSegments(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{[]string{"cetera", ".", "I", "'ve", "heard"}, "cetera. I've heard"},
		{[]string{"they", "haven", "'t", "written"}, "they haven't written"},
		{[]string{"really", ",", "you", "know", "?"}, "really, you know?"},
		{[]string{" spaced ", "out"}, "spaced out"},
	} {
		if got := joinTokens(tc.in); got != tc.want {
			t.Errorf("joinTokens(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The default must stay a hard cut. A dissolve changes the timeline's whole
// shape — two tracks instead of one, transitions, extended out points — so an
// unasked-for one would silently rewrite every existing edit.
func TestReframeDefaultsToAHardCut(t *testing.T) {
	dir, _ := reframeFixture(t, []project.Turn{
		{Speaker: "left", Start: 0, End: 4},
		{Speaker: "right", Start: 4, End: 9},
	})
	var out bytes.Buffer
	if err := run([]string{"reframe-edit", dir, "--plan"}, strings.NewReader("{}"), &out); err != nil {
		t.Fatalf("reframe-edit: %v", err)
	}
	var res struct {
		Timeline string `json:"timeline"`
	}
	_ = json.Unmarshal(out.Bytes(), &res)
	xml, err := os.ReadFile(res.Timeline)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(xml), "luma") {
		t.Error("the default edit contains a luma transition; it must be a hard cut")
	}
	if n := strings.Count(string(xml), "<playlist"); n != 1 {
		t.Errorf("the default edit has %d playlists, want 1 — a hard cut needs no A/B roll", n)
	}
}

// With a dissolve the edit becomes an A/B roll: two tracks, one luma transition
// per cut, and every shot but the last held past its cut so the next has
// something to blend with. A shot blended past media it does not have renders
// black, so the held out point is the part worth pinning.
func TestReframeDissolveBuildsAnABRollWithHeldTails(t *testing.T) {
	dir, _ := reframeFixture(t, []project.Turn{
		{Speaker: "left", Start: 0, End: 4},
		{Speaker: "right", Start: 4, End: 9},
		{Speaker: "left", Start: 9, End: 14},
	})
	body, _ := json.Marshal(map[string]any{"dissolve": 0.5})
	var out bytes.Buffer
	if err := run([]string{"reframe-edit", dir, "--plan"}, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("reframe-edit: %v", err)
	}
	var res struct {
		Timeline string `json:"timeline"`
		Shots    []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"shots"`
	}
	_ = json.Unmarshal(out.Bytes(), &res)
	xml, err := os.ReadFile(res.Timeline)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(xml)

	if n := strings.Count(doc, "<playlist"); n != 2 {
		t.Errorf("dissolve edit has %d playlists, want 2 (the A/B roll a blend requires)", n)
	}
	if n := strings.Count(doc, ">luma<"); n != len(res.Shots)-1 {
		t.Errorf("%d luma transitions for %d shots, want %d (one per cut)",
			n, len(res.Shots), len(res.Shots)-1)
	}
	// Every shot but the last is held half a second past its cut.
	tl, err := timeline.Parse(xml)
	if err != nil {
		t.Fatalf("dissolve edit does not parse back: %v", err)
	}
	for i, p := range tl.Producers {
		want := timeline.Clock(res.Shots[i].End)
		if i < len(tl.Producers)-1 {
			want = timeline.Clock(res.Shots[i].End + 0.5)
		}
		if p.Out != want {
			t.Errorf("shot %d out = %q, want %q (a tail that is not held blends into black)", i+1, p.Out, want)
		}
	}
}

// A dissolve that outlasts a shot would blend three pictures at once.
func TestReframeRejectsADissolveLongerThanAShot(t *testing.T) {
	dir, _ := reframeFixture(t, []project.Turn{
		{Speaker: "left", Start: 0, End: 2},
		{Speaker: "right", Start: 2, End: 9},
	})
	body, _ := json.Marshal(map[string]any{"dissolve": 3.0, "minShot": 0.5})
	var out bytes.Buffer
	err := run([]string{"reframe-edit", dir, "--plan"}, bytes.NewReader(body), &out)
	assertReframeCode(t, err, "input.invalid")
}
