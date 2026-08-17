package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/project"
	"github.com/mlnomadpy/daVid/internal/timeline"
	"github.com/mlnomadpy/daVid/internal/worker"
)

// shorts is the recipe that turns a question into publishable vertical clips.
// It composes the primitives rather than adding new capability: `search` finds
// the moments, `speakers` says who is talking so the frame can lock onto them,
// the transcript supplies word timings for captions, `timeline` expresses the
// result as MLT XML, and `render` produces the files. Every intermediate stays
// on disk, so an agent can inspect or hand-edit a short's timeline and re-render
// it without re-running the search.

// Default normalised horizontal centres of the two seats in a fixed two-person
// frame. Overridable per request because not every set is framed the same.
const (
	defaultLeftCentre  = 0.28
	defaultRightCentre = 0.72
	// captionWords is how many words share one caption cue: enough to read,
	// few enough to stay in sync with speech.
	captionWords = 3

	// shortFps is the delivery frame rate a vertical short is built at. Keyframe
	// positions are frame indices, so this is what converts a clip's seconds
	// into the frame a push ends on.
	shortFps = 30

	// hookSeconds is how long the opening hook card holds. Caption cues are
	// clamped to start no earlier than this so the hook stands alone in the
	// opening window instead of rendering stacked over the first captions.
	hookSeconds = 2.5

	// minCaptionShow is the shortest a caption may stay on screen. A cue pushed
	// clear of an earlier one (most often the hook card) can otherwise end
	// before or exactly at its own pushed start, and a zero-length SRT cue
	// simply never renders — the words vanish with nothing to indicate it.
	minCaptionShow = 0.5

	// headShouldersHeightKeep is the fraction of the source height the vertical
	// crop window spans. Less than the full height zooms in past a full-body
	// two-shot to a head-and-shoulders framing — the difference between a clip
	// that is watchable on a phone and one that is not.
	headShouldersHeightKeep = 0.7
	// headFaceFromTop places the detected face this far down inside the crop
	// window: high enough to leave headroom above the head, low enough that the
	// shoulders fill the rest of the frame.
	headFaceFromTop = 0.34
	// defaultSrcW/defaultSrcH are the dimensions assumed when a source was
	// ingested without an ffmpeg worker and so carries no probe. A landscape
	// interview is the overwhelming common case; the window is still explicit.
	defaultSrcW = 1920
	defaultSrcH = 1080
)

type shortsRequest struct {
	Project string  `json:"project"`
	Source  string  `json:"source"`
	Query   string  `json:"query"`
	Count   int     `json:"count"`
	Padding float64 `json:"padding"`
	// MaxExtend caps how far a cut may travel to reach a sentence boundary, so
	// a run-on passage cannot drag a short into an unwatchable length.
	MaxExtend float64 `json:"maxExtend"`
	Quality   string  `json:"quality"`
	// RenderConcurrency caps how many shorts render at once. Defaults to a
	// quarter of the cores, since melt is itself threaded.
	RenderConcurrency int `json:"renderConcurrency"`
	// Push slowly tightens the framing across each short — 0.06 zooms in 6% by
	// the end. Zero (the default) leaves the frame locked, exactly as before.
	Push float64 `json:"push"`
	Hook string  `json:"hook"`

	LeftCentre  float64 `json:"leftCentre"`
	RightCentre float64 `json:"rightCentre"`

	SearchWorker     string   `json:"searchWorker"`
	SearchWorkerArgs []string `json:"searchWorkerArgs"`
	RenderWorker     string   `json:"renderWorker"`
	RenderWorkerArgs []string `json:"renderWorkerArgs"`
}

type shortResult struct {
	Rank     int     `json:"rank"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Speaker  string  `json:"speaker"`
	Score    float64 `json:"score"`
	Text     string  `json:"text"`
	Timeline string  `json:"timeline"`
	Captions string  `json:"captions"`
	Output   string  `json:"output,omitempty"`
}

type shortsResult struct {
	Query   string        `json:"query"`
	Source  string        `json:"source"`
	Shorts  []shortResult `json:"shorts"`
	Planned bool          `json:"planned,omitempty"`
}

func shortsCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posProject, posQuery, plan, err := parseSearchArgs(args)
	if err != nil {
		return err
	}
	var req shortsRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posProject != "" {
		req.Project = posProject
	}
	if posQuery != "" {
		req.Query = posQuery
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`pass it as an argument (daVid shorts <project> "<query>") or set "project" in the JSON request`)
	}
	if req.Query == "" {
		return axerror.New("input.empty", "no query to build shorts from").
			WithHint(`pass a query argument (daVid shorts <project> "<query>") or set "query" in the JSON request`)
	}
	if req.Count <= 0 {
		req.Count = 3
	}
	if req.Padding <= 0 {
		req.Padding = 0.4
	}
	if req.MaxExtend <= 0 {
		req.MaxExtend = 6
	}
	if req.Quality == "" {
		req.Quality = "final"
	}
	if req.LeftCentre == 0 {
		req.LeftCentre = defaultLeftCentre
	}
	if req.RightCentre == 0 {
		req.RightCentre = defaultRightCentre
	}

	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `daVid init` on this directory first")
	}
	m, err := proj.Load()
	if err != nil {
		return axerror.New("project.read", "cannot read project state: "+err.Error()).AsRetryable()
	}
	src, err := resolveSource(m, req.Source, "shorts")
	if err != nil {
		return err
	}

	// Preconditions are stated, not discovered halfway through a render.
	if !proj.HasIndex(src.Hash) {
		return axerror.New("index.missing", "source has no index to search").
			WithHint("run `daVid index` on this source first")
	}
	if !proj.HasTranscript(src.Hash) {
		return axerror.New("transcript.missing", "source has no transcript for captions").
			WithHint("run `daVid transcribe` on this source first")
	}
	if !proj.HasSpeakers(src.Hash) {
		return axerror.New("speakers.missing", "source has no speaker track to frame on").
			WithHint("run `daVid speakers` on this source first")
	}
	transcript, err := proj.ReadTranscript(src.Hash)
	if err != nil {
		return axerror.New("project.read", "cannot read transcript: "+err.Error()).AsRetryable()
	}
	speakers, err := proj.ReadSpeakers(src.Hash)
	if err != nil {
		return axerror.New("project.read", "cannot read speaker track: "+err.Error()).AsRetryable()
	}
	// Prefer where detection actually found each speaker over the default
	// seating plan, so the framing works on a set this tool has never seen.
	req.LeftCentre = seatCentre(speakers, "left", req.LeftCentre)
	req.RightCentre = seatCentre(speakers, "right", req.RightCentre)
	// The crop window is measured in source pixels, so it needs the source's real
	// dimensions — recorded on the source at ingest by the ffmpeg probe.
	// Resource and dimensions come from one call so a draft can never pair the
	// proxy with the master's pixel dimensions.
	media, srcW, srcH := renderSource(src, req.Quality)

	// 1. Find the moments worth cutting.
	sw, err := resolveSearchWorker(searchRequest{
		Worker: req.SearchWorker, WorkerArgs: req.SearchWorkerArgs,
	})
	if err != nil {
		return err
	}
	resp, err := sw.Invoke(context.Background(), worker.Request{
		Op: "search",
		Params: map[string]any{
			"query": req.Query,
			"table": proj.IndexTablePath(src.Hash),
			"limit": req.Count,
		},
	})
	if err != nil {
		return err
	}
	candidates, _, err := searchFromResult(resp.Result)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return axerror.New("search.empty", "no moments matched the query").
			WithHint("try a broader query, or check the source is indexed")
	}
	// Re-rank into a deterministic total order regardless of the worker's
	// iteration order, then cap to the requested count — this mirrors searchCmd
	// so shorts and search agree on the top moments and two runs over one index
	// cannot yield different clips.
	candidates = project.RankCandidates(candidates)
	if len(candidates) > req.Count {
		candidates = candidates[:req.Count]
	}

	dir := filepath.Join(proj.CacheDir(), "shorts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return axerror.New("project.write", "cannot create shorts directory: "+err.Error()).AsRetryable()
	}

	// 2. Compose one vertical, captioned timeline per moment.
	out := shortsResult{Query: req.Query, Source: src.Hash, Planned: plan}
	for i, c := range candidates {
		// Snap to the sentence first, then add a breath either side: padding a
		// mid-clause edge only buys more of the wrong thing.
		start, end := snapToSentence(transcript.Words, c.Start, c.End, req.MaxExtend)
		start -= req.Padding
		if start < 0 {
			start = 0
		}
		end += req.Padding
		who := speakerAt(speakers, c.Start, c.End)
		base := filepath.Join(dir, fmt.Sprintf("%02d-%s", i+1, who))

		srt := base + ".srt"
		if err := os.WriteFile(srt, []byte(buildSRT(transcript, start, end, req.Hook)), 0o644); err != nil {
			return axerror.New("project.write", "cannot write captions: "+err.Error()).AsRetryable()
		}
		// Aim the frame at the speaking seat: its horizontal centre picks the
		// column, its vertical centre lifts the window onto the face.
		cx := req.RightCentre
		if who == "left" {
			cx = req.LeftCentre
		}
		cy := seatCY(speakers, who, faceY)
		tl := buildShortTimeline(media, start, end, cx, cy, srcW, srcH, srt, req.Push)
		if problems := tl.Validate(); len(problems) > 0 {
			return axerror.New("timeline.invalid",
				"composed timeline is not renderable: "+strings.Join(problems, "; ")).
				WithHint("this is a daVid bug — the recipe built an incomplete timeline")
		}
		xmlBytes, err := tl.Marshal()
		if err != nil {
			return axerror.New("timeline.encode", "cannot encode timeline: "+err.Error())
		}
		tlPath := base + ".mlt"
		if err := os.WriteFile(tlPath, xmlBytes, 0o644); err != nil {
			return axerror.New("project.write", "cannot write timeline: "+err.Error()).AsRetryable()
		}

		short := shortResult{
			Rank: i + 1, Start: round3(start), End: round3(end), Speaker: who,
			Score: c.Score, Text: c.Text, Timeline: tlPath, Captions: srt,
		}

		out.Shorts = append(out.Shorts, short)
	}

	// 3. Render, unless this is a dry run: --plan leaves every timeline and
	// caption file on disk so an agent can inspect the edit before paying for
	// the renders.
	if !plan {
		if err := renderShorts(&out, req); err != nil {
			return err
		}
	}
	return emitJSON(stdout, out)
}

// buildShortTimeline expresses one short as MLT: a vertical profile, the source
// trimmed to the moment, an explicit head-and-shoulders crop locked onto the
// speaker's seat, and burned captions. The crop is a real source-pixel window
// (headShouldersWindow), not a center=1 slide, so it frames the head and
// shoulders that make a short watchable rather than the full body.
func buildShortTimeline(resource string, start, end, cx, cy float64,
	srcW, srcH int, srt string, push float64) *timeline.Timeline {

	x, y, w, h := headShouldersWindow(srcW, srcH, cx, cy)

	tl := timeline.New()
	tl.Profile = timeline.Vertical(shortFps)
	// Keyframes are in frames, so the clip's length has to be converted at the
	// delivery frame rate. A zero push makes this a no-op and the emitted crop
	// is byte-identical to a static one.
	lastFrame := int((end - start) * float64(shortFps))
	filters := []timeline.Filter{timeline.CropWindow(srcW, srcH, x, y, w, h)}
	// The push magnifies the already-framed picture, so it goes after the crop.
	// Omitted entirely when off, which keeps a locked shot byte-identical.
	if zoom, ok := timeline.PushZoom(tl.Profile, push, lastFrame); ok {
		filters = append(filters, zoom)
	}
	// Captions last: they are burned onto the final framing, not zoomed with it.
	filters = append(filters, timeline.Subtitles(srt, "0 0 100% 90%", "Arial", "64", "#ffffffff"))
	tl.Producers = []timeline.Producer{{
		ID:       "short",
		Resource: resource,
		In:       timeline.Clock(start),
		Out:      timeline.Clock(end),
		Filters:  filters,
	}}
	return tl
}

// headShouldersWindow computes the source-pixel rectangle a vertical short keeps
// to frame a speaker head-and-shoulders. The window carries the delivery
// profile's 9:16 aspect so it fills the frame with no bars; it spans only a
// fraction of the source height (headShouldersHeightKeep) so the subject is a
// head-and-shoulders shot rather than a full body; it is centred horizontally on
// the seat (cx); and it is lifted so the face (cy) sits in the upper third, with
// headroom above and shoulders below. The window is clamped inside the source, so
// a seat near an edge yields the closest legal framing instead of an off-frame
// crop. cx and cy are normalised 0..1.
func headShouldersWindow(srcW, srcH int, cx, cy float64) (x, y, w, h int) {
	const aspect = 9.0 / 16.0
	fw, fh := float64(srcW), float64(srcH)

	ch := fh * headShouldersHeightKeep
	cw := ch * aspect
	// A source narrower than the window (e.g. an already-vertical clip) can't
	// supply the full width; shrink to fit while holding the aspect.
	if cw > fw {
		cw = fw
		ch = cw / aspect
	}
	if ch > fh {
		ch = fh
		cw = ch * aspect
	}

	left := cx*fw - cw/2
	top := cy*fh - headFaceFromTop*ch

	w = int(cw + 0.5)
	h = int(ch + 0.5)
	x = clampInt(int(left+0.5), 0, srcW-w)
	y = clampInt(int(top+0.5), 0, srcH-h)
	return x, y, w, h
}

// clampInt keeps v within [lo,hi]; a degenerate range (hi<lo) pins to lo.
func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// snapToSentence widens a matched span out to the sentence that contains it.
//
// Search matches a passage, not a thought: an index segment can begin four
// words into a sentence and stop before it lands. Cut there and the clip opens
// mid-clause and ends on a hanging "and" — the difference between finding the
// right moment and producing something postable. So the start walks back to the
// last sentence end before it, and the end walks forward to the next one.
//
// Both walks are bounded. A speaker who runs three sentences together without a
// full stop would otherwise drag a 20-second clip into a two-minute one, so
// beyond maxExtend the original edge is kept: a slightly rough cut is better
// than a clip nobody will watch.
func snapToSentence(words []project.Word, start, end, maxExtend float64) (float64, float64) {
	if len(words) == 0 {
		return start, end
	}
	snappedStart, snappedEnd := start, end

	// Walk back for an opening. A sentence start is the goal, but a speaker can
	// run for far longer than a clip should, so when the full stop is out of
	// budget a clause boundary is taken instead: opening on "and then..." reads
	// far better than opening three words into a clause. Only if neither is
	// reachable is the original edge kept.
	var clauseStart float64 = -1
	for i := len(words) - 1; i >= 0; i-- {
		w := words[i]
		if w.End > start || i+1 >= len(words) {
			continue
		}
		// Open on the next real word, not on the punctuation that closed the
		// previous one — a clip that starts on a bare comma shows one in its
		// captions.
		cand := words[nextWord(words, i+1)].Start
		if start-cand > maxExtend {
			break // everything further back is further still
		}
		if endsSentence(w.Word) {
			snappedStart = cand
			clauseStart = -1 // a sentence beats any clause found on the way
			break
		}
		if clauseStart < 0 && endsClause(w.Word) {
			clauseStart = cand
		}
	}
	if clauseStart >= 0 {
		snappedStart = clauseStart
	}

	// Walk forward to where the thought lands, with the same fallback.
	var clauseEnd float64 = -1
	for _, w := range words {
		if w.End < end {
			continue
		}
		if w.End-end > maxExtend {
			break
		}
		if endsSentence(w.Word) {
			snappedEnd = w.End
			clauseEnd = -1
			break
		}
		if clauseEnd < 0 && endsClause(w.Word) {
			clauseEnd = w.End
		}
	}
	if clauseEnd >= 0 {
		snappedEnd = clauseEnd
	}

	if snappedEnd <= snappedStart {
		return start, end // a snap that inverts the span is no snap at all
	}
	return snappedStart, snappedEnd
}

// endsSentence reports whether a token closes a sentence. whisper emits
// punctuation as its own token as often as attached, so both forms count.
func endsSentence(token string) bool {
	return endsWithAny(token, ".?!")
}

// endsClause reports a weaker break — a comma, semicolon or dash. Not as clean
// as a full stop, but a far better place to open or close a clip than an
// arbitrary word in the middle of a phrase.
func endsClause(token string) bool {
	return endsWithAny(token, ",;:—–")
}

// nextWord returns the index of the first token from i that carries actual
// text. whisper emits punctuation as standalone tokens, and a clip should open
// on a word rather than on the mark that ended the previous one.
func nextWord(words []project.Word, i int) int {
	for ; i < len(words)-1; i++ {
		if !punctuationOnly(words[i].Word) {
			return i
		}
	}
	return i
}

// punctuationOnly reports a token with no letters or digits in it at all.
func punctuationOnly(token string) bool {
	t := strings.TrimSpace(token)
	if t == "" {
		return true
	}
	return strings.IndexFunc(t, func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	}) < 0
}

func endsWithAny(token, marks string) bool {
	t := strings.TrimSpace(token)
	if t == "" {
		return false
	}
	r := []rune(t)
	return strings.ContainsRune(marks, r[len(r)-1])
}

// speakerAt attributes a span to whoever holds the most of it. Falling back to
// the left seat keeps a short renderable when detection has a gap, rather than
// failing the whole recipe over one ambiguous moment.
func speakerAt(s project.Speakers, start, end float64) string {
	held := map[string]float64{}
	for _, t := range s.Turns {
		lo, hi := max64(start, t.Start), min64(end, t.End)
		if hi > lo {
			held[t.Speaker] += hi - lo
		}
	}
	best, bestSpan := "", 0.0
	for who, span := range held {
		if span > bestSpan {
			best, bestSpan = who, span
		}
	}
	if best == "" {
		return "left"
	}
	return best
}

// buildSRT writes the words spoken in [start,end] as caption cues timed from
// the clip's own zero, optionally opening with a hook card.
func buildSRT(t project.Transcript, start, end float64, hook string) string {
	var b strings.Builder
	n := 0
	// prevEnd is the clip-local end of the last cue emitted. Every cue is clamped
	// to start no earlier than it, so no two cues' [start,end) spans overlap —
	// most importantly the hook card never renders stacked over the first caption
	// when the first spoken word begins before the hook ends.
	prevEnd := 0.0
	emit := func(cueStart, cueEnd float64, text string) {
		if cueStart < prevEnd {
			cueStart = prevEnd
		}
		if cueEnd < cueStart+minCaptionShow {
			// Clamping the end to the start alone yields a zero-length cue, which
			// renders as nothing: words spoken entirely inside the hook window get
			// pushed to hookSeconds and then end there too, so the caption is
			// dropped silently. Hold a pushed cue long enough to actually read.
			cueEnd = cueStart + minCaptionShow
		}
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n,
			srtTime(cueStart), srtTime(cueEnd), text)
		prevEnd = cueEnd
	}
	if hook != "" {
		emit(0, hookSeconds, strings.ToUpper(hook))
	}
	var group []project.Word
	flush := func() {
		if len(group) == 0 {
			return
		}
		words := make([]string, 0, len(group))
		for _, w := range group {
			words = append(words, w.Word)
		}
		emit(group[0].Start-start, group[len(group)-1].End-start,
			strings.ToUpper(joinTokens(words)))
		group = group[:0]
	}
	words := append([]project.Word(nil), t.Words...)
	sort.Slice(words, func(i, j int) bool { return words[i].Start < words[j].Start })
	for _, w := range words {
		if w.Start < start || w.End > end {
			continue
		}
		group = append(group, w)
		if len(group) >= captionWords {
			flush()
		}
	}
	flush()
	return b.String()
}

// joinTokens reassembles whisper's per-word segments into readable text.
// Word-level timing is obtained by asking whisper for one segment per word,
// which splits punctuation and contractions into standalone tokens (". ", "'ve",
// "n't"). Joined naively those read as "CETERA . I 'VE" on screen, so tokens
// that attach to the previous word are appended without a space.
func joinTokens(tokens []string) string {
	var b strings.Builder
	for _, raw := range tokens {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		// Punctuation attaches to the word before it, so a mark that opens a
		// cue has nothing to attach to. A clip cut mid-sentence inherits the
		// previous clause's comma; showing it as a floating ", I've heard"
		// looks like a typo on screen.
		if b.Len() == 0 && punctuationOnly(tok) {
			continue
		}
		if b.Len() > 0 && !attachesLeft(tok) {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
	}
	return strings.TrimSpace(b.String())
}

// attachesLeft reports whether a token belongs to the preceding word with no
// space: trailing punctuation, or a contraction tail like "'ve" / "n't".
func attachesLeft(tok string) bool {
	if strings.HasPrefix(tok, "'") || strings.HasPrefix(tok, "’") {
		return true
	}
	if tok == "n't" || tok == "n’t" {
		return true
	}
	// A token made only of punctuation clings to the word before it.
	return strings.IndexFunc(tok, func(r rune) bool {
		return !strings.ContainsRune(".,!?;:)]}%…", r)
	}) < 0
}

func srtTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }
func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// renderShorts renders every composed short, several at a time.
//
// Each short is an independent melt process over its own timeline, so rendering
// them one after another left most of the machine idle: three shorts measured
// 24.5s at 205% CPU on an 18-core box. Only the renders run concurrently —
// composition stays serial because it is cheap, writes files, and fixes the
// order the results are reported in.
//
// Determinism is the constraint, not just speed. Results are written back by
// index rather than appended as they finish, so the output is identical
// whatever order the renders complete in; and when several fail, the error
// returned is the lowest-indexed one, so a failing run reports the same error
// every time rather than whichever goroutine lost the race.
func renderShorts(out *shortsResult, req shortsRequest) error {
	rw, err := resolveMeltWorker(renderRequest{
		Worker: req.RenderWorker, WorkerArgs: req.RenderWorkerArgs,
	})
	if err != nil {
		return err
	}

	limit := req.RenderConcurrency
	if limit <= 0 {
		// melt is itself threaded — one render measured about two cores — so
		// this deliberately leaves headroom rather than running one per core.
		limit = runtime.NumCPU() / 4
	}
	if limit < 1 {
		limit = 1
	}
	if limit > len(out.Shorts) {
		limit = len(out.Shorts)
	}

	errs := make([]error, len(out.Shorts))
	outputs := make([]string, len(out.Shorts))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i := range out.Shorts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			short := out.Shorts[i]
			output := strings.TrimSuffix(short.Timeline, ".mlt") + ".mp4"
			if _, err := rw.Invoke(context.Background(), worker.Request{
				Op:     "render",
				Params: map[string]any{"timeline": short.Timeline, "output": output, "quality": req.Quality},
			}); err != nil {
				errs[i] = err
				return
			}
			if _, err := os.Stat(output); err != nil {
				errs[i] = axerror.New("worker.protocol", "render worker did not write the short").
					WithHint("the melt worker must write the rendered video to the requested output path").
					AsRetryable()
				return
			}
			outputs[i] = output
		}(i)
	}
	wg.Wait()

	// Lowest index wins, so the reported failure does not depend on scheduling.
	for i, err := range errs {
		if err != nil {
			return err
		}
		out.Shorts[i].Output = outputs[i]
	}
	return nil
}
