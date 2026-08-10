package timeline

import (
	"encoding/xml"
	"strings"
	"testing"
)

// The MLT document is the contract with melt: melt reads the XML, not the Go
// structs, so these tests assert on marshalled output rather than field values.

func TestMarshalProducesAValidMLTDocument(t *testing.T) {
	tl := New()
	tl.Profile = Vertical(30)
	tl.Producers = []Producer{{
		ID: "clip", Resource: "/media/in.mp4",
		In: Clock(1.5), Out: Clock(9),
		Filters: []Filter{NewFilter("crop", "center", "1")},
	}}

	out, err := tl.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Round-tripping proves the document is well-formed, which a substring
	// check alone would not.
	var back Timeline
	if err := xml.Unmarshal(out, &back); err != nil {
		t.Fatalf("marshalled XML does not parse back: %v\n%s", err, out)
	}
	if back.Version != "7.0" {
		t.Errorf("version = %q, want 7.0", back.Version)
	}
	if len(back.Producers) != 1 || back.Producers[0].Resource != "/media/in.mp4" {
		t.Fatalf("producer lost in round trip: %+v", back.Producers)
	}
	if back.Producers[0].In != "00:00:01.500" {
		t.Errorf("in = %q, want the clock-formatted trim", back.Producers[0].In)
	}
}

func TestVerticalAndLandscapeCarryTheRightAspect(t *testing.T) {
	v := Vertical(30)
	if v.Width != 1080 || v.Height != 1920 || v.DisplayAspectNum != 9 {
		t.Errorf("vertical = %+v, want 1080x1920 at 9:16", v)
	}
	l := Landscape(30)
	if l.Width != 1920 || l.Height != 1080 || l.DisplayAspectNum != 16 {
		t.Errorf("landscape = %+v, want 1920x1080 at 16:9", l)
	}
	// A zero frame rate would make melt produce a zero-length render.
	if Vertical(0).FrameRateNum == 0 {
		t.Error("a zero fps was accepted, which renders nothing")
	}
}

func TestClockRejectsNegativeTime(t *testing.T) {
	// A negative in-point is a clamped bug elsewhere; emitting one would make
	// melt fail on a document that looks fine.
	if got := Clock(-5); got != "00:00:00.000" {
		t.Errorf("Clock(-5) = %q, want a clamped zero", got)
	}
}

func TestNewFilterAndTransitionPutServiceFirst(t *testing.T) {
	f := NewFilter("avfilter.eq", "av.contrast", "1.1")
	if f.Properties[0].Name != "mlt_service" || f.Properties[0].Value != "avfilter.eq" {
		t.Fatalf("filter properties = %+v, want mlt_service first", f.Properties)
	}
	tr := NewTransition("mix", "sum", "1")
	if tr.Properties[0].Name != "mlt_service" || tr.Properties[0].Value != "mix" {
		t.Fatalf("transition properties = %+v, want mlt_service first", tr.Properties)
	}
	// An odd trailing key with no value must not produce a half property.
	odd := NewFilter("x", "only_a_key")
	for _, p := range odd.Properties {
		if p.Name == "only_a_key" {
			t.Error("a key with no value became a property")
		}
	}
}

func TestSequenceMakesEveryProducerPlay(t *testing.T) {
	tl := New()
	for _, id := range []string{"a", "b"} {
		tl.Producers = append(tl.Producers, Producer{ID: id, Resource: "/m.mp4"})
	}
	tl.Sequence("edit")
	if len(tl.Playlists[0].Entries) != len(tl.Producers) {
		t.Fatalf("playlist has %d entries for %d producers",
			len(tl.Playlists[0].Entries), len(tl.Producers))
	}
	if tl.Tractor == nil || tl.Tractor.Tracks[0].Producer != "edit" {
		t.Fatalf("tractor = %+v, want it playing the playlist", tl.Tractor)
	}
}

// AddAudio on a bare producer list has no track structure to mix into, so it
// must create one rather than emit a tractor-less document melt cannot mix.
func TestAddAudioBuildsTrackStructureWhenThereIsNone(t *testing.T) {
	tl := New()
	tl.Producers = []Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.AddAudio("bed", "/m.wav", 0.2, 0, 0, 0)

	if tl.Tractor == nil {
		t.Fatal("no tractor was created, so nothing would be mixed")
	}
	if len(tl.Tractor.Tracks) != 2 || len(tl.Tractor.Transitions) != 1 {
		t.Fatalf("tracks=%d transitions=%d, want 2 and 1",
			len(tl.Tractor.Tracks), len(tl.Tractor.Transitions))
	}
}

func TestAddAudioFadesAttachToTheSoundNotTheDialogue(t *testing.T) {
	tl := New()
	tl.Producers = []Producer{{ID: "v", Resource: "/v.mp4"}}
	tl.Sequence("main")
	tl.AddAudio("bed", "/m.wav", 0.2, 0, 1.5, 2)

	var bed *Producer
	for i := range tl.Producers {
		if tl.Producers[i].ID == "bed" {
			bed = &tl.Producers[i]
		}
	}
	if bed == nil {
		t.Fatal("the sound producer was not added")
	}
	var services []string
	for _, f := range bed.Filters {
		for _, p := range f.Properties {
			if p.Name == "mlt_service" {
				services = append(services, p.Value)
			}
		}
	}
	joined := strings.Join(services, " ")
	if !strings.Contains(joined, "fadeInVolume") || !strings.Contains(joined, "fadeOutVolume") {
		t.Fatalf("bed filters = %q, want both fades", joined)
	}
	// The dialogue track must be untouched.
	for _, p := range tl.Producers {
		if p.ID == "v" && len(p.Filters) != 0 {
			t.Errorf("the video producer picked up %d filters from the audio bed", len(p.Filters))
		}
	}
}

func TestValidateReportsEveryStructuralProblem(t *testing.T) {
	tl := New()
	tl.Producers = []Producer{
		{ID: "dup", Resource: "/a.mp4"},
		{ID: "dup", Resource: "/b.mp4"}, // duplicate id
		{ID: "", Resource: "/c.mp4"},    // no id
		{ID: "nores"},                   // no resource
		{ID: "badfilter", Resource: "/d.mp4", // filter with no service
			Filters: []Filter{{Properties: []Property{{Name: "x", Value: "1"}}}}},
	}
	problems := strings.Join(tl.Validate(), " | ")
	for _, want := range []string{"duplicate", "no id", "no resource", "mlt_service"} {
		if !strings.Contains(problems, want) {
			t.Errorf("Validate did not report %q; got: %s", want, problems)
		}
	}
}

func TestPunchInKeepsTheRequestedFraction(t *testing.T) {
	p := Landscape(30)
	f := PunchIn(p, 0.5, 0.5, 0.5)
	vals := map[string]string{}
	for _, prop := range f.Properties {
		vals[prop.Name] = prop.Value
	}
	// Keeping half the width of a 1920 profile means cropping 480 either side.
	if vals["left"] != "480" || vals["right"] != "480" {
		t.Errorf("left/right = %q/%q, want 480/480 for keep=0.5 centred",
			vals["left"], vals["right"])
	}
	if vals["use_profile"] != "1" {
		t.Error("crop values are not declared relative to the profile, so they " +
			"would be read as source pixels and mean something different per source")
	}
}

// CropWindow expresses a kept source-pixel rectangle as edge insets, the form
// melt's crop filter reads. A wrong inset frames the wrong region of the source.
func TestCropWindowExpressesTheKeptRectangleAsInsets(t *testing.T) {
	// Keep a 606x1078 window of a 1920x1080 source starting at (657,1).
	f := CropWindow(1920, 1080, 657, 1, 606, 1078)
	vals := map[string]string{}
	for _, prop := range f.Properties {
		vals[prop.Name] = prop.Value
	}
	if vals["mlt_service"] != "crop" {
		t.Fatalf("service = %q, want the plain crop filter (qtcrop renders black on the render worker's melt)", vals["mlt_service"])
	}
	// left=x, right=srcW-x-w, top=y, bottom=srcH-y-h.
	for _, tc := range []struct{ name, want string }{
		{"left", "657"}, {"right", "657"}, {"top", "1"}, {"bottom", "1"},
	} {
		if vals[tc.name] != tc.want {
			t.Errorf("%s inset = %q, want %q", tc.name, vals[tc.name], tc.want)
		}
	}
	// use_profile must be absent: it makes the edges a no-op on the render melt.
	if _, ok := vals["use_profile"]; ok {
		t.Error("CropWindow set use_profile, which reduces the explicit crop to a no-op on the render worker's melt")
	}
}

// Parse is Marshal's inverse: a timeline written by a build step must load back
// into the same structured model an edit mutates, or "edit in place" is a lie.
func TestParseRoundTripsAMarshalledTimeline(t *testing.T) {
	tl := New()
	tl.Profile = Vertical(30)
	tl.Producers = []Producer{{
		ID: "clip", Resource: "/media/in.mp4",
		In: Clock(1.5), Out: Clock(9),
		Filters: []Filter{
			CropWindow(1920, 1080, 657, 1, 606, 1078),
			Subtitles("caps.srt", "0 0 100% 90%", "Arial", "64", "#ffffffff"),
		},
	}}
	tl.Sequence("edit")

	out, err := tl.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(back.Producers) != 1 {
		t.Fatalf("producers = %d, want 1", len(back.Producers))
	}
	p := back.Producers[0]
	if p.ID != "clip" || p.Resource != "/media/in.mp4" || p.In != "00:00:01.500" {
		t.Fatalf("producer lost fidelity: %+v", p)
	}
	if len(p.Filters) != 2 {
		t.Fatalf("filters = %d, want 2", len(p.Filters))
	}
	if p.Filters[0].Service() != "crop" || p.Filters[1].Service() != "subtitle" {
		t.Fatalf("filter services = %q/%q, want crop/subtitle",
			p.Filters[0].Service(), p.Filters[1].Service())
	}
	// The parsed timeline must re-validate, so an edit that only reads and writes
	// it back leaves a document melt can still render.
	if problems := back.Validate(); len(problems) != 0 {
		t.Fatalf("round-tripped timeline does not validate: %v", problems)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse([]byte("<not-mlt")); err == nil {
		t.Fatal("Parse accepted malformed XML, want an error")
	}
}

// Producer resolves a named producer to a live pointer, and RemoveFilterService
// drops matching filters in place — the two operations an edit is built from.
func TestProducerAndRemoveFilterService(t *testing.T) {
	tl := New()
	tl.Producers = []Producer{{
		ID: "a", Resource: "/a.mp4",
		Filters: []Filter{
			NewFilter("crop", "left", "10"),
			NewFilter("avfilter.eq", "av.contrast", "1.1"),
			NewFilter("crop", "left", "20"),
		},
	}}

	if tl.Producer("missing") != nil {
		t.Fatal("Producer(missing) returned non-nil")
	}
	p := tl.Producer("a")
	if p == nil {
		t.Fatal("Producer(a) = nil, want the producer")
	}

	// Removing a service the producer never carried is a no-op reported as zero.
	if n := p.RemoveFilterService("subtitle"); n != 0 {
		t.Fatalf("removed %d subtitle filters, want 0", n)
	}
	// Removing crop drops both crop filters and leaves the eq untouched.
	if n := p.RemoveFilterService("crop"); n != 2 {
		t.Fatalf("removed %d crop filters, want 2", n)
	}
	if len(p.Filters) != 1 || p.Filters[0].Service() != "avfilter.eq" {
		t.Fatalf("remaining filters = %+v, want only avfilter.eq", p.Filters)
	}
	// The pointer aliases the timeline, so the removal is visible through it.
	if len(tl.Producers[0].Filters) != 1 {
		t.Fatalf("timeline still has %d filters on producer a, want 1",
			len(tl.Producers[0].Filters))
	}
}

func TestFilterServiceIsEmptyWhenUnset(t *testing.T) {
	f := Filter{Properties: []Property{{Name: "left", Value: "10"}}}
	if f.Service() != "" {
		t.Errorf("Service() = %q, want empty for a filter with no mlt_service", f.Service())
	}
}

func BenchmarkMarshalLongEdit(b *testing.B) {
	// A full-episode edit is on the order of a hundred shots; marshalling is on
	// the path of every recipe, so it should stay far off the critical path.
	tl := New()
	tl.Profile = Landscape(30)
	for i := 0; i < 120; i++ {
		tl.Producers = append(tl.Producers, Producer{
			ID: "shot", Resource: "/media/in.mp4",
			In: Clock(float64(i)), Out: Clock(float64(i) + 1),
			Filters: []Filter{PunchIn(tl.Profile, 0.58, 0.3, 0.42)},
		})
	}
	tl.Sequence("edit")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tl.Marshal(); err != nil {
			b.Fatal(err)
		}
	}
}
