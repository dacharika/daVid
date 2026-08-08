package main

import (
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
)

// speakerAt decides which seat a short frames on, so a wrong answer points the
// camera at the person who is not talking.
func TestSpeakerAtPicksTheDominantSpeaker(t *testing.T) {
	s := project.Speakers{Turns: []project.Turn{
		{Speaker: "right", Start: 0, End: 6},
		{Speaker: "left", Start: 6, End: 30},
	}}
	// The span overlaps both turns but is mostly the left seat's.
	if got := speakerAt(s, 5, 20); got != "left" {
		t.Fatalf("speakerAt = %q, want left (it holds 14s of the span vs 1s)", got)
	}
	if got := speakerAt(s, 0, 5); got != "right" {
		t.Fatalf("speakerAt = %q, want right", got)
	}
}

// A moment with no detected speaker must still render rather than failing the
// whole recipe, so it falls back instead of returning empty.
func TestSpeakerAtFallsBackWhenNothingOverlaps(t *testing.T) {
	s := project.Speakers{Turns: []project.Turn{{Speaker: "left", Start: 0, End: 2}}}
	if got := speakerAt(s, 50, 60); got == "" {
		t.Fatal("speakerAt returned an empty speaker; a short would have no framing")
	}
}

// Captions are timed from the clip's own zero, not the source's, or they drift
// by however far into the episode the clip starts.
func TestBuildSRTIsRelativeToClipStartAndClipsToRange(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		{Word: "before", Start: 1, End: 2},
		{Word: "alpha", Start: 10, End: 10.5},
		{Word: "beta", Start: 10.5, End: 11},
		{Word: "gamma", Start: 11, End: 11.5},
		{Word: "after", Start: 30, End: 31},
	}}
	srt := buildSRT(tr, 10, 12, "")

	if strings.Contains(srt, "BEFORE") || strings.Contains(srt, "AFTER") {
		t.Fatalf("captions include words outside the clip range:\n%s", srt)
	}
	if !strings.Contains(srt, "ALPHA BETA GAMMA") {
		t.Fatalf("captions missing the in-range words:\n%s", srt)
	}
	// The first in-range word starts at source 10s, i.e. clip zero.
	if !strings.Contains(srt, "00:00:00,000 --> ") {
		t.Fatalf("captions are not timed from the clip's zero:\n%s", srt)
	}
}

func TestBuildSRTLeadsWithTheHook(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{{Word: "word", Start: 0, End: 1}}}
	srt := buildSRT(tr, 0, 2, "Why does this matter?")
	if !strings.HasPrefix(srt, "1\n00:00:00,000") || !strings.Contains(srt, "WHY DOES THIS MATTER?") {
		t.Fatalf("hook is not the opening cue:\n%s", srt)
	}
}

// The composed timeline is what melt renders, so its shape is the contract:
// vertical profile, trimmed producer, and both filters present.
func TestBuildShortTimelineIsVerticalTrimmedAndFiltered(t *testing.T) {
	req := shortsRequest{LeftCentre: 0.28, RightCentre: 0.72}
	tl := buildShortTimeline("/media/in.mp4", 12.5, 20, "left", "/tmp/c.srt", req)

	if tl.Profile == nil || tl.Profile.Width != 1080 || tl.Profile.Height != 1920 {
		t.Fatalf("profile = %+v, want 1080x1920 vertical", tl.Profile)
	}
	if len(tl.Producers) != 1 {
		t.Fatalf("got %d producers, want 1", len(tl.Producers))
	}
	p := tl.Producers[0]
	if p.In != "00:00:12.500" || p.Out != "00:00:20.000" {
		t.Fatalf("producer trim = %q..%q, want the matched span", p.In, p.Out)
	}
	if len(p.Filters) != 2 {
		t.Fatalf("got %d filters, want crop + subtitles", len(p.Filters))
	}
	if problems := tl.Validate(); len(problems) > 0 {
		t.Fatalf("composed timeline does not validate: %v", problems)
	}

	xml, err := tl.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"mlt_service", "crop", "subtitle", "/tmp/c.srt"} {
		if !strings.Contains(string(xml), want) {
			t.Fatalf("MLT XML missing %q:\n%s", want, xml)
		}
	}
}

// Framing on the left seat must move the kept window the opposite way from the
// right seat; a shared sign would point both shorts at the same person.
func TestShortTimelineBiasesOppositeWaysPerSeat(t *testing.T) {
	req := shortsRequest{LeftCentre: 0.28, RightCentre: 0.72}
	biasOf := func(who string) string {
		tl := buildShortTimeline("/m.mp4", 0, 1, who, "/c.srt", req)
		for _, p := range tl.Producers[0].Filters[0].Properties {
			if p.Name == "center_bias" {
				return p.Value
			}
		}
		return ""
	}
	left, right := biasOf("left"), biasOf("right")
	if left == "" || right == "" {
		t.Fatal("crop filter has no center_bias")
	}
	if strings.HasPrefix(left, "-") == strings.HasPrefix(right, "-") {
		t.Fatalf("left bias %q and right bias %q have the same sign; both seats would frame the same person", left, right)
	}
}

func TestClockFormatsMLTTime(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0, "00:00:00.000"},
		{12.5, "00:00:12.500"},
		{3671.25, "01:01:11.250"},
	} {
		if got := timeline.Clock(tc.in); got != tc.want {
			t.Errorf("Clock(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Validate exists so an agent learns a timeline is unrenderable before paying
// for a render, so it must actually catch a missing resource.
func TestValidateCatchesAProducerWithNoResource(t *testing.T) {
	tl := timeline.New()
	tl.Producers = []timeline.Producer{{ID: "a"}}
	if problems := tl.Validate(); len(problems) == 0 {
		t.Fatal("Validate accepted a producer with no resource")
	}
	if problems := timeline.New().Validate(); len(problems) == 0 {
		t.Fatal("Validate accepted an empty timeline")
	}
}
