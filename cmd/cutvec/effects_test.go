package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/axerror"
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
