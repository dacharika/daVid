package main

import (
	"strconv"
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
	// The hook must stand alone: no later cue may begin before it ends.
	assertNoCueOverlap(t, srt)
}

// The opening retention window is exactly when the hook is meant to stand alone,
// yet shorts are cut to begin on speech — so the first word routinely starts
// inside the hook's 2.5s. buildSRT must push that caption clear rather than
// stack it under the hook.
func TestBuildSRTHookDoesNotOverlapFirstCaption(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		// Speech begins at clip zero, well inside the 2.5s hook card.
		{Word: "alpha", Start: 0, End: 0.4},
		{Word: "beta", Start: 0.4, End: 0.8},
		{Word: "gamma", Start: 0.8, End: 1.2},
		{Word: "delta", Start: 1.5, End: 2.0},
		{Word: "epsilon", Start: 3.0, End: 3.5},
	}}
	srt := buildSRT(tr, 0, 10, "Watch this")

	cues := parseCueSpans(t, srt)
	if len(cues) < 2 {
		t.Fatalf("expected a hook cue plus caption cues, got %d:\n%s", len(cues), srt)
	}
	// Cue #1 is the hook, occupying the opening card.
	if cues[0].start != 0 {
		t.Fatalf("hook cue does not open at zero: %v\n%s", cues[0], srt)
	}
	// Every caption cue must begin at or after the hook ends.
	for _, c := range cues[1:] {
		if c.start < cues[0].end {
			t.Fatalf("caption cue %v begins before the hook ends at %.3fs:\n%s", c, cues[0].end, srt)
		}
	}
	assertNoCueOverlap(t, srt)
}

// Without a hook the same non-overlap guarantee must hold across caption cues.
func TestBuildSRTCuesNeverOverlapWithoutHook(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		{Word: "one", Start: 10, End: 10.3},
		{Word: "two", Start: 10.3, End: 10.6},
		{Word: "three", Start: 10.6, End: 10.9},
		{Word: "four", Start: 10.9, End: 11.2},
	}}
	srt := buildSRT(tr, 10, 12, "")
	assertNoCueOverlap(t, srt)
}

type cueSpan struct {
	start, end float64
}

// parseCueSpans reads the "HH:MM:SS,mmm --> HH:MM:SS,mmm" timing line of every
// SRT cue in order, so a test can compare consecutive spans directly.
func parseCueSpans(t *testing.T, srt string) []cueSpan {
	t.Helper()
	var spans []cueSpan
	for _, line := range strings.Split(srt, "\n") {
		if !strings.Contains(line, "-->") {
			continue
		}
		parts := strings.SplitN(line, "-->", 2)
		spans = append(spans, cueSpan{
			start: parseSRTStamp(t, strings.TrimSpace(parts[0])),
			end:   parseSRTStamp(t, strings.TrimSpace(parts[1])),
		})
	}
	return spans
}

func parseSRTStamp(t *testing.T, s string) float64 {
	t.Helper()
	// HH:MM:SS,mmm
	clock, ms, ok := strings.Cut(s, ",")
	if !ok {
		t.Fatalf("malformed SRT stamp %q", s)
	}
	hms := strings.Split(clock, ":")
	if len(hms) != 3 {
		t.Fatalf("malformed SRT stamp %q", s)
	}
	h, _ := strconv.Atoi(hms[0])
	m, _ := strconv.Atoi(hms[1])
	sec, _ := strconv.Atoi(hms[2])
	milli, _ := strconv.Atoi(ms)
	return float64(h)*3600 + float64(m)*60 + float64(sec) + float64(milli)/1000
}

// assertNoCueOverlap fails if any cue begins before the previous one ends, the
// core promise: at most one cue is on screen at a time.
func assertNoCueOverlap(t *testing.T, srt string) {
	t.Helper()
	spans := parseCueSpans(t, srt)
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			t.Fatalf("cue %d [%.3f,%.3f) overlaps cue %d [%.3f,%.3f):\n%s",
				i, spans[i].start, spans[i].end, i-1, spans[i-1].start, spans[i-1].end, srt)
		}
	}
}

// The composed timeline is what melt renders, so its shape is the contract:
// vertical profile, trimmed producer, and both filters present.
func TestBuildShortTimelineIsVerticalTrimmedAndFiltered(t *testing.T) {
	tl := buildShortTimeline("/media/in.mp4", 12.5, 20, 0.28, 0.42, 1920, 1080, "/tmp/c.srt")

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
	// AC1: the crop must be an explicit source-pixel window, not a center=1 slide.
	if strings.Contains(string(xml), `name="center_bias"`) || strings.Contains(string(xml), `name="center"`) {
		t.Fatalf("crop still uses center=1 rather than an explicit window:\n%s", xml)
	}
	// The kept window's edges must be present and explicit.
	for _, want := range []string{`name="left"`, `name="right"`, `name="top"`, `name="bottom"`} {
		if !strings.Contains(string(xml), want) {
			t.Fatalf("crop filter is missing the explicit inset %s:\n%s", want, xml)
		}
	}
}

// Framing on the left seat must place the kept window to the left of where the
// right seat's window sits; a shared position would point both shorts at the
// same person.
func TestShortTimelineWindowTracksTheSeatColumn(t *testing.T) {
	leftOf := func(cx float64) int {
		tl := buildShortTimeline("/m.mp4", 0, 1, cx, 0.42, 1920, 1080, "/c.srt")
		for _, p := range tl.Producers[0].Filters[0].Properties {
			if p.Name == "left" {
				n, err := strconv.Atoi(p.Value)
				if err != nil {
					t.Fatalf("left inset %q is not an integer: %v", p.Value, err)
				}
				return n
			}
		}
		t.Fatal("crop filter has no left inset")
		return 0
	}
	left, right := leftOf(0.28), leftOf(0.72)
	if !(left < right) {
		t.Fatalf("left-seat window left=%d not left of right-seat window left=%d; both seats would frame the same person", left, right)
	}
}

// The window is what makes a short a head-and-shoulders shot instead of a full
// body: it must carry the 9:16 delivery aspect (so it fills the frame), zoom in
// past the full source height, stay inside the source, and sit the face high in
// the frame with headroom above.
func TestHeadShouldersWindowFramesHeadAndShoulders(t *testing.T) {
	const srcW, srcH = 1920, 1080
	cx, cy := 0.28, 0.42
	x, y, w, h := headShouldersWindow(srcW, srcH, cx, cy)

	if x < 0 || y < 0 || x+w > srcW || y+h > srcH {
		t.Fatalf("window %d,%d %dx%d escapes the %dx%d source", x, y, w, h, srcW, srcH)
	}
	if h >= srcH {
		t.Fatalf("window height %d does not zoom in past the full source height %d — that is a full body, not head-and-shoulders", h, srcH)
	}
	// 9:16 aspect within a pixel of rounding, so melt fills rather than bars.
	if got, want := float64(w)/float64(h), 9.0/16.0; got < want-0.01 || got > want+0.01 {
		t.Fatalf("window aspect %.4f (=%d/%d), want 9:16 (%.4f) so it fills the vertical frame", got, w, h, want)
	}
	faceY := cy * srcH
	if faceY <= float64(y) || faceY >= float64(y+h) {
		t.Fatalf("face at y=%.0f is outside the window rows %d..%d", faceY, y, y+h)
	}
	// The face sits in the upper portion (headroom above, shoulders below).
	if pos := (faceY - float64(y)) / float64(h); pos > 0.5 {
		t.Fatalf("face sits %.0f%% down the window; a head-and-shoulders framing keeps it in the upper half", pos*100)
	}
	// The horizontal window is centred on the seat.
	if centre := float64(x) + float64(w)/2; centre < cx*srcW-2 || centre > cx*srcW+2 {
		t.Fatalf("window horizontal centre %.0f is not on the seat column %.0f", centre, cx*srcW)
	}
}

// A seat detected near a frame edge must yield the closest legal window, never a
// crop that runs off the source.
func TestHeadShouldersWindowClampsToEdges(t *testing.T) {
	const srcW, srcH = 1920, 1080
	for _, cx := range []float64{0.01, 0.99} {
		x, _, w, _ := headShouldersWindow(srcW, srcH, cx, 0.42)
		if x < 0 || x+w > srcW {
			t.Fatalf("cx=%.2f produced window x=%d w=%d escaping [0,%d]", cx, x, w, srcW)
		}
	}
	// A face near the top must not push the window above the source.
	if _, y, _, _ := headShouldersWindow(srcW, srcH, 0.5, 0.02); y < 0 {
		t.Fatalf("top-edge face produced negative window y=%d", y)
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

// Search matches a passage, not a thought: an index segment can open four words
// into a sentence. Cutting there produces a clip that starts mid-clause.
func TestSnapToSentenceWidensToTheSentence(t *testing.T) {
	words := []project.Word{
		{Word: "Anyway", Start: 0, End: 1}, {Word: ".", Start: 1, End: 1.1},
		{Word: "The", Start: 2, End: 2.4}, {Word: "winners", Start: 2.4, End: 3},
		{Word: "find", Start: 3, End: 3.5}, {Word: "the", Start: 3.5, End: 3.8},
		{Word: "edges", Start: 3.8, End: 4.4}, {Word: ".", Start: 4.4, End: 4.5},
		{Word: "Next", Start: 5, End: 5.5},
	}
	// A match that begins at "find" and ends at "the" is mid-sentence at both ends.
	start, end := snapToSentence(words, 3.0, 3.8, 6)
	if start != 2.0 {
		t.Errorf("start = %v, want 2.0 (the sentence's first word)", start)
	}
	if end != 4.5 {
		t.Errorf("end = %v, want 4.5 (where the sentence lands)", end)
	}
}

// A speaker who runs on must not drag a short into an unwatchable length, so a
// boundary beyond the budget is not taken.
func TestSnapToSentenceRespectsTheExtensionBudget(t *testing.T) {
	words := []project.Word{
		{Word: "Start", Start: 0, End: 0.5}, {Word: ".", Start: 0.5, End: 0.6},
		{Word: "on", Start: 100, End: 100.4},
		{Word: "and", Start: 200, End: 200.4}, {Word: ".", Start: 200.4, End: 200.5},
	}
	start, end := snapToSentence(words, 100, 100.4, 5)
	if start != 100 || end != 100.4 {
		t.Errorf("got %v..%v, want the original edges: both boundaries are far outside the budget", start, end)
	}
}

// When a full stop is out of budget, a clause boundary still beats an arbitrary
// word in the middle of a phrase.
func TestSnapToSentenceFallsBackToAClause(t *testing.T) {
	words := []project.Word{
		{Word: "Long", Start: 0, End: 0.5}, {Word: "preamble", Start: 0.5, End: 1},
		{Word: ",", Start: 30, End: 30.1},
		{Word: "the", Start: 31, End: 31.3}, {Word: "point", Start: 31.3, End: 32},
	}
	start, _ := snapToSentence(words, 31.3, 32, 5)
	if start != 31 {
		t.Errorf("start = %v, want 31 (just past the comma), not the mid-phrase original", start)
	}
}

// A clip cut mid-sentence inherits the previous clause's comma; rendering it as
// a floating ", I've heard" looks like a typo burned into the video.
func TestJoinTokensDropsPunctuationWithNothingToAttachTo(t *testing.T) {
	if got := joinTokens([]string{",", "I", "'ve", "heard"}); got != "I've heard" {
		t.Errorf("joinTokens = %q, want %q", got, "I've heard")
	}
	if got := joinTokens([]string{".", "?", "Next"}); got != "Next" {
		t.Errorf("joinTokens = %q, want %q", got, "Next")
	}
	// Punctuation that does have a word before it must still attach.
	if got := joinTokens([]string{"done", "."}); got != "done." {
		t.Errorf("joinTokens = %q, want %q", got, "done.")
	}
}
