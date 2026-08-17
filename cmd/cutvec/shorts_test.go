package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/timeline"
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
	// The hook must stand alone: the first spoken word begins at clip zero, well
	// before the hook ends, yet no caption cue may overlap the hook card.
	cues := parseSRTCues(t, srt)
	assertNoCueOverlap(t, cues)
	if len(cues) == 0 || cues[0].text != "WHY DOES THIS MATTER?" {
		t.Fatalf("hook is not the opening cue:\n%s", srt)
	}
}

// A hook card and a caption cue must never render stacked: even when the first
// spoken word begins before the 2.5s hook ends, the captions are pushed clear.
func TestBuildSRTHookNeverOverlapsFirstCaption(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		{Word: "starts", Start: 0.5, End: 1.0}, // begins well inside the hook window
		{Word: "during", Start: 1.0, End: 1.8},
		{Word: "hook", Start: 1.8, End: 2.2},
		{Word: "after", Start: 3.0, End: 3.5},
	}}
	srt := buildSRT(tr, 0, 5, "Watch this")
	cues := parseSRTCues(t, srt)
	assertNoCueOverlap(t, cues)
	if len(cues) == 0 || cues[0].text != "WATCH THIS" {
		t.Fatalf("hook is not the opening cue:\n%s", srt)
	}
	// The first caption must begin no earlier than the hook's end.
	if cues[1].start < 2.5 {
		t.Fatalf("first caption starts at %.3fs, inside the [0,2.5) hook window:\n%s", cues[1].start, srt)
	}
}

// Pushing a cue clear of the hook must not erase it. When every word of a group
// is spoken inside the hook window, clamping the start to hookSeconds and the
// end to no-less-than-the-start collapses the cue to zero length — and a
// zero-length SRT cue renders as nothing, so the words disappear with no error
// and no sign anything was dropped. Every cue has to stay readable.
func TestBuildSRTCaptionPushedPastTheHookStaysOnScreen(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		// The whole group finishes before the 2.5s hook card does.
		{Word: "hey", Start: 0.1, End: 0.4},
		{Word: "there", Start: 0.5, End: 0.9},
		{Word: "folks", Start: 1.0, End: 1.4},
	}}
	srt := buildSRT(tr, 0, 10, "Watch this")
	cues := parseSRTCues(t, srt)
	assertNoCueOverlap(t, cues)
	if len(cues) < 2 {
		t.Fatalf("caption cue was dropped entirely:\n%s", srt)
	}
	for _, c := range cues {
		if c.end <= c.start {
			t.Errorf("cue %q is zero-length (%.3f --> %.3f) and will never render:\n%s",
				c.text, c.start, c.end, srt)
		}
	}
	if d := cues[1].end - cues[1].start; d < minCaptionShow {
		t.Errorf("pushed caption shows for %.3fs, want at least %.3fs:\n%s", d, minCaptionShow, srt)
	}
}

// The non-overlap guarantee holds with no hook too: consecutive caption cues
// must not overlap each other.
func TestBuildSRTCaptionsDoNotOverlapWithoutHook(t *testing.T) {
	tr := project.Transcript{Words: []project.Word{
		{Word: "one", Start: 10, End: 10.4},
		{Word: "two", Start: 10.4, End: 10.8},
		{Word: "three", Start: 10.8, End: 11.2},
		{Word: "four", Start: 11.2, End: 11.6},
		{Word: "five", Start: 11.6, End: 12.0},
		{Word: "six", Start: 12.0, End: 12.4},
	}}
	srt := buildSRT(tr, 10, 13, "")
	assertNoCueOverlap(t, parseSRTCues(t, srt))
}

type srtCue struct {
	start, end float64
	text       string
}

// parseSRTCues turns buildSRT's output back into ordered cues so a test can
// reason about their spans rather than string-matching timestamps.
func parseSRTCues(t *testing.T, srt string) []srtCue {
	t.Helper()
	var cues []srtCue
	for _, block := range strings.Split(strings.TrimSpace(srt), "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 3 {
			t.Fatalf("malformed SRT block %q in:\n%s", block, srt)
		}
		times := strings.Split(lines[1], " --> ")
		if len(times) != 2 {
			t.Fatalf("malformed timing line %q in:\n%s", lines[1], srt)
		}
		cues = append(cues, srtCue{
			start: parseSRTTime(t, times[0]),
			end:   parseSRTTime(t, times[1]),
			text:  strings.Join(lines[2:], "\n"),
		})
	}
	return cues
}

// parseSRTTime converts an "HH:MM:SS,mmm" stamp back to seconds.
func parseSRTTime(t *testing.T, s string) float64 {
	t.Helper()
	s = strings.TrimSpace(s)
	hms, ms, ok := strings.Cut(s, ",")
	if !ok {
		t.Fatalf("timestamp %q missing the ,millis separator", s)
	}
	parts := strings.Split(hms, ":")
	if len(parts) != 3 {
		t.Fatalf("timestamp %q is not HH:MM:SS", s)
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])
	milli, _ := strconv.Atoi(ms)
	return float64(h*3600+m*60+sec) + float64(milli)/1000
}

// assertNoCueOverlap fails if any consecutive pair of cue spans overlaps.
func assertNoCueOverlap(t *testing.T, cues []srtCue) {
	t.Helper()
	for i := 1; i < len(cues); i++ {
		if cues[i].start < cues[i-1].end {
			t.Fatalf("cue %d [%0.3f,%0.3f) overlaps cue %d [%0.3f,%0.3f)",
				i, cues[i].start, cues[i].end, i-1, cues[i-1].start, cues[i-1].end)
		}
	}
}

// The composed timeline is what melt renders, so its shape is the contract:
// vertical profile, trimmed producer, and both filters present.
func TestBuildShortTimelineIsVerticalTrimmedAndFiltered(t *testing.T) {
	tl := buildShortTimeline("/media/in.mp4", 12.5, 20, 0.28, 0.42, 1920, 1080, "/tmp/c.srt", 0)

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
		tl := buildShortTimeline("/m.mp4", 0, 1, cx, 0.42, 1920, 1080, "/c.srt", 0)
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

// Rendering several shorts at once must not change the answer. Results are
// written back by index rather than appended on completion, so ranks stay in
// order and each short keeps the output belonging to its own timeline — the
// failure this guards is a fast render landing its path on a slower short's
// record, which would pair rank 1 with rank 3's video and look entirely valid.
func TestRenderShortsIsOrderStableUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	var out shortsResult
	const n = 8
	for i := 1; i <= n; i++ {
		tl := filepath.Join(dir, fmt.Sprintf("%02d-left.mlt", i))
		if err := os.WriteFile(tl, []byte("<mlt/>"), 0o644); err != nil {
			t.Fatal(err)
		}
		out.Shorts = append(out.Shorts, shortResult{Rank: i, Timeline: tl})
	}
	wpath, wargs := renderWorkerEnv(t, "render")

	if err := renderShorts(&out, shortsRequest{
		RenderWorker: wpath, RenderWorkerArgs: wargs, Quality: "proxy",
	}); err != nil {
		t.Fatalf("renderShorts: %v", err)
	}
	for i, s := range out.Shorts {
		if s.Rank != i+1 {
			t.Errorf("shorts[%d].Rank = %d — order was not preserved", i, s.Rank)
		}
		// Each output must belong to its OWN timeline, not a neighbour's.
		want := strings.TrimSuffix(s.Timeline, ".mlt") + ".mp4"
		if s.Output != want {
			t.Errorf("shorts[%d].Output = %q, want %q (a result landed on the wrong short)", i, s.Output, want)
		}
		if _, err := os.Stat(s.Output); err != nil {
			t.Errorf("shorts[%d] output does not exist: %v", i, err)
		}
	}
}

// A render failure must be reported identically on every run. With several
// renders in flight the naive answer is "whichever goroutine failed first",
// which makes the same broken input produce different errors run to run.
func TestRenderShortsReportsTheLowestIndexedFailure(t *testing.T) {
	dir := t.TempDir()
	var out shortsResult
	for i := 1; i <= 4; i++ {
		tl := filepath.Join(dir, fmt.Sprintf("%02d-left.mlt", i))
		if err := os.WriteFile(tl, []byte("<mlt/>"), 0o644); err != nil {
			t.Fatal(err)
		}
		out.Shorts = append(out.Shorts, shortResult{Rank: i, Timeline: tl})
	}
	// render-partial writes nothing, so every short fails the did-it-write
	// check and the returned error must still be a stable one.
	wpath, wargs := renderWorkerEnv(t, "render-partial")

	var first error
	for run := 0; run < 5; run++ {
		err := renderShorts(&out, shortsRequest{
			RenderWorker: wpath, RenderWorkerArgs: wargs, Quality: "proxy",
		})
		if err == nil {
			t.Fatal("expected a failure when the worker writes nothing")
		}
		if run == 0 {
			first = err
			continue
		}
		if err.Error() != first.Error() {
			t.Fatalf("error differs between runs:\n  %v\n  %v", first, err)
		}
	}
}

// A push of zero must leave the emitted MLT byte-identical to before the
// feature existed: no extra filter, and a plain static crop. This is the guard
// on "adding a capability silently changed every existing edit".
func TestPushOfZeroChangesNothing(t *testing.T) {
	tl := buildShortTimeline("/m.mp4", 0, 3, 0.5, 0.4, 1920, 1080, "/c.srt", 0)
	fs := tl.Producers[0].Filters
	if len(fs) != 2 {
		t.Fatalf("push=0 emitted %d filters, want 2 (crop + subtitles)", len(fs))
	}
	for _, f := range fs {
		for _, p := range f.Properties {
			if strings.Contains(p.Value, ";") {
				t.Errorf("push=0 emitted an animated property %s=%q", p.Name, p.Value)
			}
			if p.Value == "qtblend" {
				t.Error("push=0 emitted a qtblend zoom filter")
			}
		}
	}
	if _, ok := timeline.PushZoom(timeline.Vertical(30), 0, 90); ok {
		t.Error("PushZoom reported a filter for push=0")
	}
}

// A non-zero push adds one qtblend whose rect animates to a LARGER rectangle —
// magnifying the picture. Shrinking it would push out, not in.
//
// The shape is what a unit test can check, and checking it matters: MLT ignores
// a keyframe string on a parameter it cannot animate rather than rejecting it,
// so a render proves nothing on its own. Neither `crop` nor `affine` marks any
// parameter animatable in MLT's metadata; qtblend marks `rect`. An earlier
// attempt animated `crop`, passed its unit tests, rendered successfully — and
// emitted the uncropped frame letterboxed, because the ignored crop applied
// nothing at all.
func TestPushAddsAnAnimatedZoomThatMagnifies(t *testing.T) {
	tl := buildShortTimeline("/m.mp4", 0, 3, 0.5, 0.4, 1920, 1080, "/c.srt", 0.12)
	fs := tl.Producers[0].Filters
	if len(fs) != 3 {
		t.Fatalf("push emitted %d filters, want 3 (crop + zoom + subtitles)", len(fs))
	}
	var rect string
	for _, p := range fs[1].Properties {
		if p.Name == "mlt_service" && p.Value != "qtblend" {
			t.Fatalf("zoom filter is %q, want qtblend (the only one of crop/affine/qtblend that animates)", p.Value)
		}
		if p.Name == "rect" {
			rect = p.Value
		}
	}
	var x0, y0, w0, h0, a0, x1, y1, w1, h1, a1 float64
	var lastFrame int
	if _, err := fmt.Sscanf(rect, "0=%g %g %g %g %g; %d=%g %g %g %g %g",
		&x0, &y0, &w0, &h0, &a0, &lastFrame, &x1, &y1, &w1, &h1, &a1); err != nil {
		t.Fatalf("rect %q is not a parseable keyframe pair: %v", rect, err)
	}
	if w1 <= w0 || h1 <= h0 {
		t.Errorf("rect goes %gx%g -> %gx%g; a push must magnify", w0, h0, w1, h1)
	}
	// Centred: the enlarged picture is offset by half its growth in each axis,
	// so the subject stays where the crop framed it instead of drifting.
	if got, want := x1, -(w1-w0)/2; got != want {
		t.Errorf("x offset %g, want %g (not centred — the subject drifts)", got, want)
	}
	if got, want := y1, -(h1-h0)/2; got != want {
		t.Errorf("y offset %g, want %g (not centred)", got, want)
	}
}

// Out-of-range pushes are clamped rather than allowed to crop to nothing.
func TestPushIsClamped(t *testing.T) {
	prof := timeline.Vertical(30)
	// Past half again the subject fills the frame entirely.
	huge, ok := timeline.PushZoom(prof, 5.0, 90)
	if !ok {
		t.Fatal("a large push produced no filter")
	}
	var w1 float64
	for _, p := range huge.Properties {
		if p.Name == "rect" {
			var x0, y0, w0, h0, a0, x, y, h, a float64
			var lf int
			if _, err := fmt.Sscanf(p.Value, "0=%g %g %g %g %g; %d=%g %g %g %g %g",
				&x0, &y0, &w0, &h0, &a0, &lf, &x, &y, &w1, &h, &a); err != nil {
				t.Fatalf("rect %q: %v", p.Value, err)
			}
		}
	}
	if w1 > float64(prof.Width)*1.5 {
		t.Errorf("a push of 5.0 magnified to %g px wide; it must clamp to +50%%", w1)
	}
	// A negative push is a locked frame, not an inverted one.
	if _, ok := timeline.PushZoom(prof, -1, 90); ok {
		t.Error("a negative push produced a zoom filter, want none")
	}
}
