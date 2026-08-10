package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
)

func filterServices(t *testing.T, filters []timeline.Filter) []string {
	t.Helper()
	var out []string
	for _, f := range filters {
		for _, p := range f.Properties {
			if p.Name == "mlt_service" {
				out = append(out, p.Value)
			}
		}
	}
	return out
}

// A preset is only a named chain of real MLT services; if it silently expanded
// to nothing, a caller would think they had graded when they had not.
func TestPresetsExpandToRealMLTServices(t *testing.T) {
	for _, name := range presetNames() {
		got, err := resolveEffects([]effectSpec{{Preset: name}})
		if err != nil {
			t.Fatalf("preset %q: %v", name, err)
		}
		services := filterServices(t, got[""])
		if len(services) == 0 {
			t.Fatalf("preset %q expanded to no filters", name)
		}
		for _, s := range services {
			if s == "" {
				t.Fatalf("preset %q produced a filter with an empty mlt_service", name)
			}
		}
	}
}

// The point of effects is that a timeline is as capable as MLT: any service, not
// just the ones with presets.
func TestRawServiceEffectCarriesItsProperties(t *testing.T) {
	got, err := resolveEffects([]effectSpec{{
		Service:    "avfilter.vignette",
		Properties: map[string]string{"av.angle": "0.4", "av.mode": "backward"},
	}})
	if err != nil {
		t.Fatalf("resolveEffects: %v", err)
	}
	f := got[""][0]
	seen := map[string]string{}
	for _, p := range f.Properties {
		seen[p.Name] = p.Value
	}
	if seen["mlt_service"] != "avfilter.vignette" {
		t.Fatalf("service = %q, want avfilter.vignette", seen["mlt_service"])
	}
	if seen["av.angle"] != "0.4" || seen["av.mode"] != "backward" {
		t.Fatalf("properties lost: %v", seen)
	}
}

// Identical requests must produce identical XML, or the timeline stops being
// diffable and content-hash caching breaks.
func TestEffectPropertyOrderIsDeterministic(t *testing.T) {
	spec := []effectSpec{{
		Service: "avfilter.eq",
		Properties: map[string]string{
			"av.contrast": "1.1", "av.saturation": "1.2",
			"av.gamma": "1.0", "av.brightness": "0.01",
		},
	}}
	var first []string
	for i := 0; i < 8; i++ {
		got, err := resolveEffects(spec)
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, p := range got[""][0].Properties {
			order = append(order, p.Name)
		}
		if first == nil {
			first = order
			continue
		}
		if strings.Join(order, ",") != strings.Join(first, ",") {
			t.Fatalf("property order changed between runs:\n%v\n%v", first, order)
		}
	}
}

// An unknown preset must name the real ones, so an agent can correct itself in
// one step instead of guessing.
func TestUnknownPresetListsTheKnownOnes(t *testing.T) {
	_, err := resolveEffects([]effectSpec{{Preset: "cinematic"}})
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want an *axerror.Error", err)
	}
	if ae.Code != "effect.unknown_preset" {
		t.Errorf("code = %q, want effect.unknown_preset", ae.Code)
	}
	if !strings.Contains(ae.Hint, "grade") {
		t.Errorf("hint does not list known presets: %q", ae.Hint)
	}
}

func TestEffectMustNameEitherPresetOrService(t *testing.T) {
	if _, err := resolveEffects([]effectSpec{{}}); err == nil {
		t.Fatal("an empty effect was accepted")
	}
	if _, err := resolveEffects([]effectSpec{{Preset: "grade", Service: "avfilter.eq"}}); err == nil {
		t.Fatal("an effect naming both a preset and a service was accepted")
	}
}

// Effects apply to every clip by default, and to one clip when named — a
// mis-targeted grade would silently colour the wrong shot.
func TestEffectsTargetAllProducersOrOneByID(t *testing.T) {
	build := func() *timeline.Timeline {
		tl := timeline.New()
		tl.Producers = []timeline.Producer{
			{ID: "a", Resource: "/a.mp4"}, {ID: "b", Resource: "/b.mp4"},
		}
		return tl
	}

	tl := build()
	all, _ := resolveEffects([]effectSpec{{Preset: "warm"}})
	if err := applyEffects(tl, all); err != nil {
		t.Fatal(err)
	}
	for _, p := range tl.Producers {
		if len(p.Filters) != 1 {
			t.Fatalf("producer %q got %d filters, want the effect applied to all", p.ID, len(p.Filters))
		}
	}

	tl = build()
	one, _ := resolveEffects([]effectSpec{{Preset: "warm", Producer: "b"}})
	if err := applyEffects(tl, one); err != nil {
		t.Fatal(err)
	}
	if len(tl.Producers[0].Filters) != 0 {
		t.Errorf("producer a was affected by an effect targeted at b")
	}
	if len(tl.Producers[1].Filters) != 1 {
		t.Errorf("producer b did not get its targeted effect")
	}
}

// Targeting a producer that does not exist is a typo, and silently doing nothing
// would look like a working render that lost its grade.
func TestEffectOnUnknownProducerIsRejectedWithTheRealIDs(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "clip1", Resource: "/a.mp4"}}
	byProducer, _ := resolveEffects([]effectSpec{{Preset: "grade", Producer: "clipOne"}})

	err := applyEffects(tl, byProducer)
	var ae *axerror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want an *axerror.Error", err)
	}
	if !strings.Contains(ae.Hint, "clip1") {
		t.Errorf("hint does not name the real producer ids: %q", ae.Hint)
	}
}

// The audio presets must produce audio filters and the video ones video
// filters; swapping them would apply a grade to a waveform.
func TestGradeIsVideoAndDenoiseIsAudio(t *testing.T) {
	grade, _ := resolveEffects([]effectSpec{{Preset: "grade"}})
	if got := strings.Join(filterServices(t, grade[""]), " "); !strings.Contains(got, "curves") {
		t.Errorf("grade = %q, want the tone curve that tames a blown background", got)
	}
	denoise, _ := resolveEffects([]effectSpec{{Preset: "denoise"}})
	got := strings.Join(filterServices(t, denoise[""]), " ")
	if !strings.Contains(got, "afftdn") || !strings.Contains(got, "highpass") {
		t.Errorf("denoise = %q, want a high-pass into a spectral denoiser", got)
	}
}

// A bed must be an extra track combined by a mix transition. Without the
// transition MLT would play the sound instead of the dialogue, not under it.
func TestAudioBedAddsAMixedTrackNotAReplacement(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.Sequence("main")

	if err := applyAudio(tl, []audioSpec{{Resource: "/music.wav", Gain: 0.15}}); err != nil {
		t.Fatalf("applyAudio: %v", err)
	}
	if len(tl.Tractor.Tracks) != 2 {
		t.Fatalf("got %d tracks, want the video plus the bed", len(tl.Tractor.Tracks))
	}
	if len(tl.Tractor.Transitions) != 1 {
		t.Fatalf("got %d transitions, want one mix", len(tl.Tractor.Transitions))
	}
	props := map[string]string{}
	for _, p := range tl.Tractor.Transitions[0].Properties {
		props[p.Name] = p.Value
	}
	if props["mlt_service"] != "mix" {
		t.Errorf("service = %q, want mix", props["mlt_service"])
	}
	if props["sum"] != "1" {
		t.Errorf("sum = %q, want 1 — without it the bed crossfades over the dialogue", props["sum"])
	}
	if props["start"] != "0.15" || props["end"] != "0.15" {
		t.Errorf("gain = %q..%q, want the requested 0.15", props["start"], props["end"])
	}
}

// A sting has to be able to land on a moment, which means a delay on its track.
func TestAudioStartOffsetBecomesALeadingBlank(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.Sequence("main")
	if err := applyAudio(tl, []audioSpec{{Resource: "/sting.wav", Gain: 0.5, Start: 12.5}}); err != nil {
		t.Fatal(err)
	}
	var bed *timeline.Playlist
	for i := range tl.Playlists {
		if strings.HasSuffix(tl.Playlists[i].ID, "_track") {
			bed = &tl.Playlists[i]
		}
	}
	if bed == nil {
		t.Fatal("no audio track playlist was created")
	}
	if bed.Entries[0].Blank != "00:00:12.500" {
		t.Fatalf("first entry = %+v, want a 12.5s blank delaying the sting", bed.Entries[0])
	}
}

// Two beds must not collide on one track index, or the second silently
// replaces the first in the mix.
func TestMultipleBedsGetDistinctTracks(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.Sequence("main")
	err := applyAudio(tl, []audioSpec{
		{Resource: "/music.wav", Gain: 0.12},
		{Resource: "/sting.wav", Gain: 0.6, Start: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Tractor.Tracks) != 3 || len(tl.Tractor.Transitions) != 2 {
		t.Fatalf("tracks=%d transitions=%d, want 3 and 2",
			len(tl.Tractor.Tracks), len(tl.Tractor.Transitions))
	}
	seen := map[string]bool{}
	for _, tr := range tl.Tractor.Transitions {
		for _, p := range tr.Properties {
			if p.Name == "b_track" {
				if seen[p.Value] {
					t.Fatalf("two beds share b_track %q; one would be lost", p.Value)
				}
				seen[p.Value] = true
			}
		}
	}
}

// Gain is a level, and a nonsense one should be refused rather than producing
// a silent or clipped mix nobody notices until playback.
func TestAudioGainOutOfRangeIsRejected(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.Sequence("main")
	if err := applyAudio(tl, []audioSpec{{Resource: "/m.wav", Gain: 4}}); err == nil {
		t.Fatal("a gain of 4 was accepted")
	}
	if err := applyAudio(tl, []audioSpec{{Gain: 0.2}}); err == nil {
		t.Fatal("an audio entry with no resource was accepted")
	}
}

// A detected seat must win over the default seating plan — that is the whole
// point of measuring: the plan is right for one set and wrong for every other.
func TestSeatCentrePrefersDetectionOverTheDefault(t *testing.T) {
	s := project.Speakers{Seats: map[string]project.Seat{
		"left":  {CX: 0.19, CY: 0.4, Detections: 500},
		"right": {CX: 0.81, CY: 0.4, Detections: 500},
	}}
	if got := seatCentre(s, "left", 0.28); got != 0.19 {
		t.Errorf("left = %v, want the detected 0.19 rather than the 0.28 default", got)
	}
	if got := seatCentre(s, "right", 0.72); got != 0.81 {
		t.Errorf("right = %v, want the detected 0.81", got)
	}
}

// An undetected seat has to fall back rather than aim at zero, which would
// frame the edge of the room.
func TestSeatCentreFallsBackWhenUndetected(t *testing.T) {
	s := project.Speakers{Seats: map[string]project.Seat{
		"left": {CX: 0.3, Detections: 100},
	}}
	if got := seatCentre(s, "right", 0.72); got != 0.72 {
		t.Errorf("undetected right = %v, want the 0.72 fallback", got)
	}
	if got := seatCentre(project.Speakers{}, "left", 0.28); got != 0.28 {
		t.Errorf("no seats at all = %v, want the fallback", got)
	}
}

// A face reported hard against a frame edge is far more likely a mis-detection
// than a real seat, and trusting it would point the camera off the subject.
func TestSeatCentreRejectsImplausiblePositions(t *testing.T) {
	for _, cx := range []float64{0.0, 0.02, 0.98, 1.0} {
		s := project.Speakers{Seats: map[string]project.Seat{
			"left": {CX: cx, Detections: 300},
		}}
		if got := seatCentre(s, "left", 0.28); got != 0.28 {
			t.Errorf("cx=%v was trusted (got %v), want the fallback", cx, got)
		}
	}
}

// Coverage has to survive the cache, or a caller sees it on the first run and
// loses it on every cache hit — exactly when they are least likely to notice.
func TestSpeakerCoverageRoundTripsThroughTheCache(t *testing.T) {
	dir := t.TempDir()
	proj := project.Open(dir)
	if _, _, err := proj.Init("t"); err != nil {
		t.Fatal(err)
	}
	want := project.Speakers{
		Layout: project.TwoPersonLayout,
		Turns:  []project.Turn{{Speaker: "left", Start: 0, End: 5}},
		Seats:  map[string]project.Seat{"left": {CX: 0.3, CY: 0.4, Detections: 120}},
		Coverage: &project.Coverage{
			Duration: 45, Attributed: 25, Unattributed: 20,
			Left: 16.5, Right: 8.5, AttributedFraction: 0.556,
		},
	}
	if err := proj.WriteSpeakers("hash", want); err != nil {
		t.Fatal(err)
	}
	got, err := proj.ReadSpeakers("hash")
	if err != nil {
		t.Fatal(err)
	}
	if got.Coverage == nil {
		t.Fatal("coverage was lost through the cache")
	}
	if got.Coverage.Unattributed != 20 || got.Coverage.AttributedFraction != 0.556 {
		t.Fatalf("coverage = %+v, want the written values", *got.Coverage)
	}
	if seat, ok := got.Seats["left"]; !ok || seat.CX != 0.3 {
		t.Fatalf("seats = %+v, want the written left seat", got.Seats)
	}
}
