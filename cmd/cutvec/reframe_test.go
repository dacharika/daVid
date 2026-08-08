package main

import (
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
)

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
