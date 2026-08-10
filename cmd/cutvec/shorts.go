package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/timeline"
	"github.com/tahabsn/cutvec/internal/worker"
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
	// biasScale converts a normalised offset from frame centre into MLT's
	// center_bias, which with use_profile=1 is expressed in profile-width units.
	biasScale = 1080.0
	// captionWords is how many words share one caption cue: enough to read,
	// few enough to stay in sync with speech.
	captionWords = 3
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
	Hook      string  `json:"hook"`

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
			WithHint(`pass it as an argument (cutvec shorts <project> "<query>") or set "project" in the JSON request`)
	}
	if req.Query == "" {
		return axerror.New("input.empty", "no query to build shorts from").
			WithHint(`pass a query argument (cutvec shorts <project> "<query>") or set "query" in the JSON request`)
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
			WithHint("run `cutvec init` on this directory first")
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
			WithHint("run `cutvec index` on this source first")
	}
	if !proj.HasTranscript(src.Hash) {
		return axerror.New("transcript.missing", "source has no transcript for captions").
			WithHint("run `cutvec transcribe` on this source first")
	}
	if !proj.HasSpeakers(src.Hash) {
		return axerror.New("speakers.missing", "source has no speaker track to frame on").
			WithHint("run `cutvec speakers` on this source first")
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
		tl := buildShortTimeline(src.Path, start, end, who, srt, req)
		if problems := tl.Validate(); len(problems) > 0 {
			return axerror.New("timeline.invalid",
				"composed timeline is not renderable: "+strings.Join(problems, "; ")).
				WithHint("this is a cutvec bug — the recipe built an incomplete timeline")
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

		// 3. Render, unless this is a dry run: --plan leaves every timeline and
		// caption file on disk so an agent can inspect the edit before paying
		// for the renders.
		if !plan {
			rw, err := resolveMeltWorker(renderRequest{
				Worker: req.RenderWorker, WorkerArgs: req.RenderWorkerArgs,
			})
			if err != nil {
				return err
			}
			output := base + ".mp4"
			if _, err := rw.Invoke(context.Background(), worker.Request{
				Op:     "render",
				Params: map[string]any{"timeline": tlPath, "output": output, "quality": req.Quality},
			}); err != nil {
				return err
			}
			if _, err := os.Stat(output); err != nil {
				return axerror.New("worker.protocol", "render worker did not write the short").
					WithHint("the melt worker must write the rendered video to the requested output path").
					AsRetryable()
			}
			short.Output = output
		}
		out.Shorts = append(out.Shorts, short)
	}
	return emitJSON(stdout, out)
}

// buildShortTimeline expresses one short as MLT: a vertical profile, the source
// trimmed to the moment, a crop locked onto the speaker, and burned captions.
func buildShortTimeline(resource string, start, end float64, who, srt string,
	req shortsRequest) *timeline.Timeline {

	centre := req.RightCentre
	if who == "left" {
		centre = req.LeftCentre
	}
	// center=1 crops whatever is needed to fill the 9:16 frame; center_bias then
	// slides that window off the frame's midpoint onto the speaker. The sign is
	// inverted relative to frame coordinates — a positive bias walks the kept
	// window towards the left of frame — which is verified by rendering, not
	// assumed.
	bias := (0.5 - centre) * biasScale

	tl := timeline.New()
	tl.Profile = timeline.Vertical(30)
	tl.Producers = []timeline.Producer{{
		ID:       "short",
		Resource: resource,
		In:       timeline.Clock(start),
		Out:      timeline.Clock(end),
		Filters: []timeline.Filter{
			timeline.NewFilter("crop",
				"center", "1",
				"center_bias", fmt.Sprintf("%.0f", bias),
				"use_profile", "1",
			),
			timeline.Subtitles(srt, "0 0 100% 90%", "Arial", "64", "#ffffffff"),
		},
	}}
	return tl
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
	if hook != "" {
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n,
			srtTime(0), srtTime(2.5), strings.ToUpper(hook))
	}
	var group []project.Word
	flush := func() {
		if len(group) == 0 {
			return
		}
		n++
		words := make([]string, 0, len(group))
		for _, w := range group {
			words = append(words, w.Word)
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n,
			srtTime(group[0].Start-start), srtTime(group[len(group)-1].End-start),
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
