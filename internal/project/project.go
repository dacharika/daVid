// Package project models cutvec's explicit project-directory state: the on-disk
// home for proxies, renders, the content-hash cache, and step results that make
// runs deterministic, idempotent, and resumable. Handles passed between tool
// calls resolve to paths under a Project, never to inlined blobs.
//
// The state lives in two places under Dir: project.json (the Manifest — the
// durable record of what has been ingested) and cache/ (content-hash-keyed step
// results, so an unchanged step is skipped on a resumed run). A caller reads the
// whole graph in one shot with Status.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ManifestVersion is the schema version stamped into every project.json, so a
// later cutvec can recognise and migrate manifests it did not write.
const ManifestVersion = "0"

// ErrNotInitialized is returned by steps that require a project.json manifest
// (Ingest, Status) when the directory has not been Init'd yet. Callers map it to
// the stable "project.uninitialized" axerror code.
var ErrNotInitialized = errors.New("project not initialized")

// Project is a handle to a cutvec project directory. It is a value: opening a
// project records where state lives without yet touching the filesystem, so
// callers can plan (--plan / dry-run) without side effects.
type Project struct {
	// Dir is the absolute or relative root of the project's state directory.
	Dir string
}

// Open returns a Project rooted at dir. It does not create the directory;
// materialising state is the job of the steps that write into it.
func Open(dir string) Project {
	return Project{Dir: dir}
}

// CacheDir is the content-hash cache root: step outputs keyed by the hash of
// their inputs, so an unchanged step is skipped on a resumed run.
func (p Project) CacheDir() string {
	return filepath.Join(p.Dir, "cache")
}

// TimelinePath is where the project's MLT timeline document lives — the handle
// a timeline-building step returns instead of the XML blob itself.
func (p Project) TimelinePath() string {
	return filepath.Join(p.Dir, "timeline.mlt")
}

// ManifestPath is where the project.json manifest lives — the durable record of
// project state that init creates and status reads.
func (p Project) ManifestPath() string {
	return filepath.Join(p.Dir, "project.json")
}

// Manifest is the on-disk project.json: the durable, human- and agent-legible
// record of a project's state. It grows as steps append their results; sources
// are the media files ingested so far.
type Manifest struct {
	Version string   `json:"version"`
	Name    string   `json:"name"`
	Sources []Source `json:"sources"`
}

// Source is one ingested input media file, identified by the content hash of its
// bytes. Two files with identical contents share one Source, which is what makes
// ingest a content-addressed, idempotent step.
type Source struct {
	// ID is a short, stable handle for the source (the content-hash prefix).
	ID string `json:"id"`
	// Path is the resource path the source was ingested from.
	Path string `json:"path"`
	// Hash is the full hex sha256 of the file's contents — the cache key.
	Hash string `json:"hash"`
	// Probe holds the source's intrinsic pixel dimensions, duration and fps,
	// measured once at ingest by the ffmpeg worker. It is nil when ingest ran
	// with no ffmpeg worker configured (probing is skipped, not an error) or on a
	// source recorded before probing existed.
	Probe *Probe `json:"probe,omitempty"`
	// Proxy is a handle to the small, fast-seeking copy of the source built once
	// at ingest, so the expensive steps do not repeatedly seek through a 4K
	// master. Empty when ingest ran with no Deriver configured, or on a source
	// recorded before derivation existed.
	Proxy string `json:"proxy,omitempty"`
	// ProxyWidth and ProxyHeight are the proxy's own pixel dimensions. A step
	// that renders from the proxy must compute its crop windows against these,
	// never against Probe — those describe the master, and a 4K rectangle
	// applied to a 540p frame renders green and frames nothing.
	ProxyWidth  int `json:"proxyWidth,omitempty"`
	ProxyHeight int `json:"proxyHeight,omitempty"`
	// Audio is a handle to the source's audio track on its own, extracted once at
	// ingest — transcription wants samples, not pixels. Empty under the same
	// conditions as Proxy.
	Audio string `json:"audio,omitempty"`
}

// Probe is a source's intrinsic video dimensions, measured once at ingest by the
// ffmpeg worker: pixel width and height, duration in seconds, and frames per
// second. Framing reads these to compute a real crop window instead of assuming
// one. It is recorded on the Source, so a re-ingest — a content-hash cache hit —
// returns the same probe without re-running the worker.
type Probe struct {
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Duration float64 `json:"duration"`
	Fps      float64 `json:"fps"`
}

// Prober measures a source's intrinsic dimensions at ingest. Ingest calls Probe
// exactly once, and only on a content-hash cache miss, so a re-ingest of
// unchanged bytes stays a pure cache hit that runs no prober. A nil Prober skips
// probing entirely: ingest still records the source, just without a Probe — this
// keeps internal/project free of any worker import, the ffmpeg boundary lives in
// the caller.
type Prober interface {
	Probe(path string) (Probe, error)
}

// Deriver builds the media ingest caches alongside the source: a small proxy the
// expensive steps run on instead of the original, and the audio track on its
// own. Like Prober it is called only on a content-hash cache miss, so a
// re-ingest of unchanged bytes stays a pure cache hit that runs neither, and a
// nil Deriver skips derivation entirely — ingest still records the source, just
// without handles.
//
// The destination path is chosen by Ingest and passed in rather than returned,
// which is what keeps internal/project free of any worker import: the ffmpeg
// boundary lives in the caller, exactly as it does for Prober.
type Deriver interface {
	// Proxy returns the dimensions the proxy was actually written at, not the
	// ones asked for. A crop computed against assumed proxy dimensions but
	// applied to the real ones renders successfully and frames nothing, so this
	// is measured rather than derived.
	Proxy(src, dest string) (int, int, error)
	Audio(src, dest string) error
}

// ProxyPath and AudioPath are where a source's derived media live: beside every
// other artifact in the cache, keyed on the source's content hash so two
// sources never collide and a re-ingest finds the same files.
func (p Project) ProxyPath(hash string) string {
	return filepath.Join(p.CacheDir(), "proxy-"+hash+".mp4")
}

func (p Project) AudioPath(hash string) string {
	return filepath.Join(p.CacheDir(), "audio-"+hash+".wav")
}

// Initialized reports whether the project manifest exists on disk.
func (p Project) Initialized() bool {
	_, err := os.Stat(p.ManifestPath())
	return err == nil
}

// Init materialises the project directory and its manifest. It is idempotent: on
// a directory that already holds a manifest it returns the existing one with
// created=false, so a resumed or repeated run never clobbers state or fails.
// When name is empty the directory's base name is used.
func (p Project) Init(name string) (Manifest, bool, error) {
	if p.Initialized() {
		m, err := p.Load()
		return m, false, err
	}
	if name == "" {
		name = filepath.Base(filepath.Clean(p.Dir))
	}
	m := Manifest{Version: ManifestVersion, Name: name, Sources: []Source{}}
	if err := p.Save(m); err != nil {
		return Manifest{}, false, err
	}
	return m, true, nil
}

// Load reads and decodes the project manifest.
func (p Project) Load() (Manifest, error) {
	b, err := os.ReadFile(p.ManifestPath())
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	if m.Sources == nil {
		m.Sources = []Source{}
	}
	return m, nil
}

// Save writes the manifest to project.json, creating the project directory if
// needed. Output is indented and newline-terminated so it diffs cleanly.
func (p Project) Save(m Manifest) error {
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.ManifestPath(), append(b, '\n'), 0o644)
}

// HashFile returns the hex sha256 of the file's contents — the content hash that
// keys the cache, so an unchanged input always resolves to the same key.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DescribeSource computes the content hash of path and returns the Source record
// it would ingest to, without touching project state — the basis for --plan and
// the shared identity Ingest keys on.
func (p Project) DescribeSource(path string) (Source, error) {
	hash, err := HashFile(path)
	if err != nil {
		return Source{}, err
	}
	return Source{ID: hash[:12], Path: path, Hash: hash}, nil
}

// cachePath is where the cache entry for a content-hash key lives.
func (p Project) cachePath(key string) string {
	return filepath.Join(p.CacheDir(), key+".json")
}

// CacheHit reports whether a cache entry already exists for the content-hash
// key — i.e. whether the step keyed on it has already run for these inputs.
func (p Project) CacheHit(key string) bool {
	_, err := os.Stat(p.cachePath(key))
	return err == nil
}

// IngestResult reports the outcome of ingesting one source: the recorded source
// and whether the content-hash cache already held it. Cached=true means the
// bytes were seen before, no work or state change happened, and the re-run was
// idempotent.
type IngestResult struct {
	Source Source `json:"source"`
	Cached bool   `json:"cached"`
}

// Ingest records a source file in the project, keyed by the content hash of its
// bytes. Re-ingesting an unchanged file is a cache hit: the hash matches an
// existing cache entry, no manifest mutation happens, and Cached is true. This
// is what makes the step deterministic, idempotent, and resumable — the same
// input always lands the same source exactly once.
//
// On a cache miss the source is probed via the prober (the ffmpeg worker) before
// it is recorded, so its pixel dimensions, duration and fps are stored on the
// Source and read straight back on the next cache-hit re-run. A nil prober skips
// probing: the source is still recorded, just without a Probe.
func (p Project) Ingest(path string, prober Prober, deriver Deriver) (IngestResult, error) {
	if !p.Initialized() {
		return IngestResult{}, ErrNotInitialized
	}
	src, err := p.DescribeSource(path)
	if err != nil {
		return IngestResult{}, err
	}
	if p.CacheHit(src.Hash) {
		// Cache hit: identical bytes were ingested before. Return the cached
		// record untouched — the defining idempotent re-run. The probe recorded
		// on the first ingest comes back with it; no prober runs.
		cached, err := p.readCachedSource(src.Hash)
		if err != nil {
			return IngestResult{}, err
		}
		return IngestResult{Source: cached, Cached: true}, nil
	}

	// Cache miss: probe the source's dimensions once, before recording it, so the
	// probe is stored on the source and a later re-ingest is a pure cache hit that
	// runs no worker. A nil prober (no ffmpeg worker configured) leaves Probe nil.
	if prober != nil {
		pr, err := prober.Probe(path)
		if err != nil {
			return IngestResult{}, err
		}
		src.Probe = &pr
	}

	// Same contract as probing, one step further: build the proxy and the audio
	// track once, here, so every later step reads a small file instead of the
	// master and a re-ingest runs neither. Handles are recorded on the source, so
	// they come back with the cache hit.
	if deriver != nil {
		if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
			return IngestResult{}, err
		}
		proxy := p.ProxyPath(src.Hash)
		pw, ph, err := deriver.Proxy(path, proxy)
		if err != nil {
			return IngestResult{}, err
		}
		src.Proxy = proxy
		src.ProxyWidth, src.ProxyHeight = pw, ph

		audio := p.AudioPath(src.Hash)
		if err := deriver.Audio(path, audio); err != nil {
			return IngestResult{}, err
		}
		src.Audio = audio
	}

	// Record the source (deduped by hash) and store the cache entry so the next
	// run of the same bytes is a hit.
	m, err := p.Load()
	if err != nil {
		return IngestResult{}, err
	}
	if !hasSource(m.Sources, src.Hash) {
		m.Sources = append(m.Sources, src)
		sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].Hash < m.Sources[j].Hash })
		if err := p.Save(m); err != nil {
			return IngestResult{}, err
		}
	}
	if err := p.writeCachedSource(src.Hash, src); err != nil {
		return IngestResult{}, err
	}
	return IngestResult{Source: src, Cached: false}, nil
}

// writeCachedSource stores a source record as the cache entry for its hash.
func (p Project) writeCachedSource(hash string, src Source) error {
	if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.cachePath(hash), append(b, '\n'), 0o644)
}

// readCachedSource reads the source record cached under a content hash.
func (p Project) readCachedSource(hash string) (Source, error) {
	b, err := os.ReadFile(p.cachePath(hash))
	if err != nil {
		return Source{}, err
	}
	var s Source
	if err := json.Unmarshal(b, &s); err != nil {
		return Source{}, err
	}
	return s, nil
}

// Word is one transcribed token with its start and end offset, in seconds from
// the start of the media. Word-level timing is what lets an agent search the
// transcript and build karaoke captions.
type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Transcript is the word-level transcript of one source, as returned by the
// whisper worker and cached in the project. It is content-addressed by Source
// (the sha256 of the transcribed media), so re-transcribing identical bytes is a
// cache hit.
type Transcript struct {
	// Source is the content hash of the media this transcript was produced from.
	Source string `json:"source"`
	// Language is the detected language code, when the worker reports one.
	Language string `json:"language,omitempty"`
	// Text is the full transcript text, when the worker reports one.
	Text string `json:"text,omitempty"`
	// Words are the timecoded tokens, each with a start/end in seconds.
	Words []Word `json:"words"`
}

// transcriptKey namespaces a transcript cache entry so it never collides with
// the source record ingest writes under the bare content hash.
func transcriptKey(hash string) string { return "transcribe-" + hash }

// TranscriptPath is the handle a transcribe step returns: the path to the cached
// word-level transcript for the source with the given content hash.
func (p Project) TranscriptPath(hash string) string {
	return p.cachePath(transcriptKey(hash))
}

// HasTranscript reports whether a transcript for the source has already been
// produced — i.e. whether transcribe is a cache hit for these bytes.
func (p Project) HasTranscript(hash string) bool {
	return p.CacheHit(transcriptKey(hash))
}

// WriteTranscript stores the transcript as the cache entry for its source hash,
// creating the cache directory if needed.
func (p Project) WriteTranscript(hash string, t Transcript) error {
	if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.TranscriptPath(hash), append(b, '\n'), 0o644)
}

// ReadTranscript reads the cached transcript for the source hash.
func (p Project) ReadTranscript(hash string) (Transcript, error) {
	b, err := os.ReadFile(p.TranscriptPath(hash))
	if err != nil {
		return Transcript{}, err
	}
	var t Transcript
	if err := json.Unmarshal(b, &t); err != nil {
		return Transcript{}, err
	}
	return t, nil
}

// Index is the metadata of a LanceDB index built over one source's transcript
// segments: the embedded segments live in a sibling .lance table, and this
// record is the cached handle plus a small summary. It is content-addressed by
// Source (the sha256 of the indexed media), so re-indexing identical bytes is a
// cache hit that runs no worker.
type Index struct {
	// Source is the content hash of the media whose transcript was indexed.
	Source string `json:"source"`
	// Table is the path to the .lance table holding the embedded segments.
	Table string `json:"table"`
	// Segments is the number of transcript segments embedded into the table.
	Segments int `json:"segments"`
	// Dim is the dimensionality of each embedding vector.
	Dim int `json:"dim"`
	// Model is the local embedding model the worker used, when it reports one.
	Model string `json:"model,omitempty"`
}

// indexKey namespaces an index cache entry so it never collides with the source
// record or the transcript entry keyed on the same content hash.
func indexKey(hash string) string { return "index-" + hash }

// IndexPath is the handle an index step returns: the path to the cached index
// metadata JSON for the source with the given content hash.
func (p Project) IndexPath(hash string) string {
	return p.cachePath(indexKey(hash))
}

// IndexTablePath is where the LanceDB table for the source's index lives — the
// .lance sibling of the metadata JSON, written by the local embed worker.
func (p Project) IndexTablePath(hash string) string {
	return filepath.Join(p.CacheDir(), indexKey(hash)+".lance")
}

// HasIndex reports whether an index for the source has already been built —
// i.e. whether index is a cache hit for these bytes.
func (p Project) HasIndex(hash string) bool {
	return p.CacheHit(indexKey(hash))
}

// WriteIndex stores the index metadata as the cache entry for its source hash,
// creating the cache directory if needed. The .lance table itself is written by
// the worker, not here.
func (p Project) WriteIndex(hash string, idx Index) error {
	if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.IndexPath(hash), append(b, '\n'), 0o644)
}

// ReadIndex reads the cached index metadata for the source hash.
func (p Project) ReadIndex(hash string) (Index, error) {
	b, err := os.ReadFile(p.IndexPath(hash))
	if err != nil {
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return Index{}, err
	}
	return idx, nil
}

// Candidate is one ranked search hit: a timecoded transcript span (Start/End in
// seconds) with the matched Text and a relevance Score. Search returns these
// ranked most-relevant first. Unlike a transcript or index it is not cached — it
// is a query result computed on demand over the (cached) index.
type Candidate struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// RankCandidates returns candidates in the deterministic ranked order search
// emits: descending Score first, then ascending Start, End, and Text as stable
// tiebreaks. This total order means a fixed query+index always yields the
// identical ranking no matter what order the worker reported hits in — the
// determinism the search acceptance criterion requires. The input is not
// mutated; a sorted copy is returned.
func RankCandidates(cands []Candidate) []Candidate {
	ranked := make([]Candidate, len(cands))
	copy(ranked, cands)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		switch {
		case a.Score != b.Score:
			return a.Score > b.Score
		case a.Start != b.Start:
			return a.Start < b.Start
		case a.End != b.End:
			return a.End < b.End
		default:
			return a.Text < b.Text
		}
	})
	return ranked
}

// TwoPersonLayout is the fixed speaker layout the speakers step analyses against:
// two people in a stable side-by-side frame. It is a constant, not a request
// parameter — so the speakers cache key stays the bare source content hash with
// no risk of two layouts colliding on one key.
const TwoPersonLayout = "two-person"

// Turn is one contiguous span of time attributed to a single speaker, in seconds
// from the start of the media. A sequence of turns is the active-speaker track an
// editor uses to reframe and to build speaker-locked shorts.
type Turn struct {
	// Speaker is the label of who is speaking during the span (e.g. the fixed
	// two-person layout's "left"/"right").
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}

// Speakers is the active-speaker analysis of one source, as returned by the
// mediapipe worker and cached in the project. It is content-addressed by Source
// (the sha256 of the analysed media), so re-analysing identical bytes is a cache
// hit. Layout records the fixed layout the turns were computed against.
type Speakers struct {
	// Source is the content hash of the media this analysis was produced from.
	Source string `json:"source"`
	// Layout is the fixed speaker layout the turns were computed against.
	Layout string `json:"layout"`
	// Turns are the ordered per-time speaker spans.
	Turns []Turn `json:"turns"`
	// Seats is where each speaker actually sits in the frame, keyed by the same
	// labels the turns use. Detection reports it so a reframe can aim at the
	// real subject rather than an assumed seating plan; a seat that was not
	// reliably detected is absent, and callers fall back.
	Seats map[string]Seat `json:"seats,omitempty"`
	// Coverage is how much of the media detection was willing to attribute.
	Coverage *Coverage `json:"coverage,omitempty"`
}

// Coverage reports how much of a source detection attributed to a speaker.
// Ambiguous stretches are deliberately left unattributed rather than forced
// onto whoever spoke last, and those become the wide two-shot in an edit — so a
// low attributed fraction predicts a mostly-wide edit, and is the signal that a
// source is hard for detection rather than genuinely quiet.
type Coverage struct {
	Duration           float64 `json:"duration"`
	Attributed         float64 `json:"attributed"`
	Unattributed       float64 `json:"unattributed"`
	Left               float64 `json:"left"`
	Right              float64 `json:"right"`
	AttributedFraction float64 `json:"attributedFraction"`
}

// Seat is a speaker's position in the frame, normalised to 0..1.
type Seat struct {
	CX float64 `json:"cx"`
	CY float64 `json:"cy"`
	// Detections is how many frames the position was measured from — a seat
	// backed by few detections is a weaker claim about where someone is.
	Detections int `json:"detections"`
}

// speakersKey namespaces a speakers cache entry so it never collides with the
// source record ingest writes under the bare content hash, nor with a transcript.
func speakersKey(hash string) string { return "speakers-" + hash }

// SpeakersPath is the handle a speakers step returns: the path to the cached
// active-speaker turns for the source with the given content hash.
func (p Project) SpeakersPath(hash string) string {
	return p.cachePath(speakersKey(hash))
}

// HasSpeakers reports whether an active-speaker analysis for the source has
// already been produced — i.e. whether speakers is a cache hit for these bytes.
func (p Project) HasSpeakers(hash string) bool {
	return p.CacheHit(speakersKey(hash))
}

// WriteSpeakers stores the analysis as the cache entry for its source hash,
// creating the cache directory if needed.
func (p Project) WriteSpeakers(hash string, s Speakers) error {
	if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.SpeakersPath(hash), append(b, '\n'), 0o644)
}

// ReadSpeakers reads the cached active-speaker analysis for the source hash.
func (p Project) ReadSpeakers(hash string) (Speakers, error) {
	b, err := os.ReadFile(p.SpeakersPath(hash))
	if err != nil {
		return Speakers{}, err
	}
	var s Speakers
	if err := json.Unmarshal(b, &s); err != nil {
		return Speakers{}, err
	}
	return s, nil
}

// renderKey namespaces a render cache entry by the timeline's content hash and
// the quality tier, so proxy and final renders of the same timeline never
// collide and each re-run resolves to its own stable handle.
func renderKey(hash, quality string) string { return "render-" + hash + "-" + quality }

// RenderPath is the handle a render step returns: the path to the rendered video
// for the timeline with the given content hash at the given quality tier.
func (p Project) RenderPath(hash, quality string) string {
	return filepath.Join(p.CacheDir(), renderKey(hash, quality)+".mp4")
}

// HasRender reports whether the timeline has already been rendered at this
// quality — i.e. whether render is a cache hit for these timeline bytes.
func (p Project) HasRender(hash, quality string) bool {
	_, err := os.Stat(p.RenderPath(hash, quality))
	return err == nil
}

// hasSource reports whether a source with the given content hash is already in ss.
func hasSource(ss []Source, hash string) bool {
	for _, s := range ss {
		if s.Hash == hash {
			return true
		}
	}
	return false
}

// Artifact is one derived output the project can produce, with its status
// resolved from whether the file exists on disk: "done" once written, "pending"
// until then.
type Artifact struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

// Artifact statuses.
const (
	StatusDone    = "done"
	StatusPending = "pending"
)

// StateGraph is the whole project graph an agent reads in one call: the sources
// ingested so far and the artifacts, each marked done or pending. This is what
// `cutvec status` returns — enough to decide what step to run next.
type StateGraph struct {
	Project   string     `json:"project"`
	Name      string     `json:"name"`
	Sources   []Source   `json:"sources"`
	Artifacts []Artifact `json:"artifacts"`
}

// Status reads the manifest and resolves the current state graph, marking each
// known artifact done or pending from the filesystem.
func (p Project) Status() (StateGraph, error) {
	if !p.Initialized() {
		return StateGraph{}, ErrNotInitialized
	}
	m, err := p.Load()
	if err != nil {
		return StateGraph{}, err
	}
	artifacts := []Artifact{p.artifact("timeline", p.TimelinePath())}
	// Render artifacts are per-quality outputs keyed by the timeline's content
	// hash, so they only become resolvable once a timeline has been built to hash.
	// Each tier is done once its cache entry exists, pending until then.
	if hash, err := HashFile(p.TimelinePath()); err == nil {
		artifacts = append(artifacts,
			p.artifact("render:proxy", p.RenderPath(hash, "proxy")),
			p.artifact("render:final", p.RenderPath(hash, "final")))
	}
	// A transcript, an index, and a speaker track are per-source artifacts, each
	// done once its cache entry exists.
	for _, s := range m.Sources {
		artifacts = append(artifacts, p.artifact("transcript:"+s.ID, p.TranscriptPath(s.Hash)))
		artifacts = append(artifacts, p.artifact("index:"+s.ID, p.IndexPath(s.Hash)))
		artifacts = append(artifacts, p.artifact("speakers:"+s.ID, p.SpeakersPath(s.Hash)))
	}
	return StateGraph{
		Project:   p.Dir,
		Name:      m.Name,
		Sources:   m.Sources,
		Artifacts: artifacts,
	}, nil
}

// artifact resolves one artifact's done/pending status from whether its file
// exists on disk.
func (p Project) artifact(name, path string) Artifact {
	status := StatusPending
	if _, err := os.Stat(path); err == nil {
		status = StatusDone
	}
	return Artifact{Name: name, Path: path, Status: status}
}
