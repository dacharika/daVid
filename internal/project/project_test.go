package project

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSource creates a temp file with the given contents and returns its path.
func writeSource(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "src.bin")
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestIngestIsContentHashCacheHit is the unit-level proof of the idempotent
// step: ingesting identical bytes twice is a cache miss then a cache hit, and the
// manifest records the source exactly once. Different bytes are a separate miss.
func TestIngestIsContentHashCacheHit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}

	src := writeSource(t, "same bytes")

	first, err := p.Ingest(src, nil, nil)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Cached {
		t.Fatalf("first ingest cached = true, want false")
	}
	if !p.CacheHit(first.Source.Hash) {
		t.Fatalf("cache entry missing after ingest of %q", first.Source.Hash)
	}

	second, err := p.Ingest(src, nil, nil)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !second.Cached {
		t.Fatalf("second ingest cached = false, want true")
	}
	if second.Source.Hash != first.Source.Hash {
		t.Fatalf("hash drifted: %q then %q", first.Source.Hash, second.Source.Hash)
	}

	m, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("manifest has %d sources, want 1 after re-ingest", len(m.Sources))
	}

	// Different content is a genuinely new source: another miss, a second entry.
	other := writeSource(t, "different bytes")
	res, err := p.Ingest(other, nil, nil)
	if err != nil {
		t.Fatalf("ingest other: %v", err)
	}
	if res.Cached {
		t.Fatalf("distinct content cached = true, want false")
	}
	if m, _ := p.Load(); len(m.Sources) != 2 {
		t.Fatalf("manifest has %d sources, want 2 after a distinct ingest", len(m.Sources))
	}
}

// countingProber is a fake Prober that records how many times it ran and returns
// a fixed probe — enough to prove Ingest probes exactly once, on the cache miss.
type countingProber struct {
	probe Probe
	calls int
}

func (c *countingProber) Probe(string) (Probe, error) {
	c.calls++
	return c.probe, nil
}

// countingDeriver records what it was asked to build and writes a stand-in file
// at each destination, so a test can assert both the handles and the file the
// handle points at without needing ffmpeg.
type countingDeriver struct {
	proxyCalls, audioCalls int
	proxyDest, audioDest   string
}

func (c *countingDeriver) Proxy(_, dest string) (int, int, error) {
	c.proxyCalls++
	c.proxyDest = dest
	return 960, 540, os.WriteFile(dest, []byte("proxy bytes"), 0o644)
}

func (c *countingDeriver) Audio(_, dest string) error {
	c.audioCalls++
	c.audioDest = dest
	return os.WriteFile(dest, []byte("audio bytes"), 0o644)
}

// Derived media follows exactly the contract the probe does: built once on the
// cache miss, recorded on the source as handles, and returned by the cache-hit
// re-run without the deriver running again. Re-deriving on every ingest would
// undo the point — transcoding a 4K master is the most expensive thing the
// pipeline does, and ingest is the step most likely to be re-run.
func TestIngestDerivesProxyAndAudioOnceAndCaches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "video bytes")
	der := &countingDeriver{}

	first, err := p.Ingest(src, nil, der)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Source.Proxy == "" || first.Source.Audio == "" {
		t.Fatalf("ingest returned no derived handles: %+v", first.Source)
	}
	// The handles must be the content-hash-keyed cache paths, so two sources
	// cannot collide and a re-ingest finds the same files.
	if want := p.ProxyPath(first.Source.Hash); first.Source.Proxy != want {
		t.Errorf("proxy handle = %q, want %q", first.Source.Proxy, want)
	}
	if want := p.AudioPath(first.Source.Hash); first.Source.Audio != want {
		t.Errorf("audio handle = %q, want %q", first.Source.Audio, want)
	}
	// The proxy's own dimensions must be recorded, not inferred: a renderer that
	// reads the proxy computes its crops against these, and a zero here is what
	// makes it fall back to the master rather than crop against nothing.
	if first.Source.ProxyWidth != 960 || first.Source.ProxyHeight != 540 {
		t.Errorf("proxy dims = %dx%d, want 960x540 as the deriver reported",
			first.Source.ProxyWidth, first.Source.ProxyHeight)
	}
	// A handle that points at nothing is worse than no handle at all.
	for _, h := range []string{first.Source.Proxy, first.Source.Audio} {
		if _, err := os.Stat(h); err != nil {
			t.Errorf("handle %q does not exist: %v", h, err)
		}
	}
	if der.proxyCalls != 1 || der.audioCalls != 1 {
		t.Fatalf("deriver ran proxy=%d audio=%d on the miss, want 1 each", der.proxyCalls, der.audioCalls)
	}

	second, err := p.Ingest(src, nil, der)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !second.Cached {
		t.Fatal("second ingest cached = false, want true")
	}
	if second.Source.Proxy != first.Source.Proxy || second.Source.Audio != first.Source.Audio {
		t.Errorf("cache hit returned different handles: %+v vs %+v", second.Source, first.Source)
	}
	if der.proxyCalls != 1 || der.audioCalls != 1 {
		t.Fatalf("deriver ran proxy=%d audio=%d total, want 1 each (nothing on the cache hit)",
			der.proxyCalls, der.audioCalls)
	}
}

// A missing ffmpeg worker must not fail the ingest: derived media is a best
// effort, exactly as probing is. Losing the ability to record a source because
// ffmpeg is absent would make every later step unreachable.
func TestIngestWithoutDeriverStillRecordsTheSource(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	res, err := p.Ingest(writeSource(t, "no ffmpeg here"), nil, nil)
	if err != nil {
		t.Fatalf("ingest with no deriver: %v", err)
	}
	if res.Source.Hash == "" {
		t.Fatal("source was not recorded")
	}
	if res.Source.Proxy != "" || res.Source.Audio != "" {
		t.Errorf("handles reported with no deriver: %+v", res.Source)
	}
}

// TestIngestProbesOnceAndCaches proves the probe is measured on the cache miss,
// stored on the source, returned unchanged on the cache-hit re-run, and that the
// prober runs exactly once (never on the hit).
func TestIngestProbesOnceAndCaches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "video bytes")
	prober := &countingProber{probe: Probe{Width: 1920, Height: 1080, Duration: 12.5, Fps: 30}}

	first, err := p.Ingest(src, prober, nil)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Source.Probe == nil {
		t.Fatal("first ingest recorded no probe")
	}
	if got := *first.Source.Probe; got != prober.probe {
		t.Fatalf("recorded probe = %+v, want %+v", got, prober.probe)
	}
	if prober.calls != 1 {
		t.Fatalf("prober ran %d times on the miss, want 1", prober.calls)
	}

	// The cache-hit re-run returns the same probe and runs no prober.
	second, err := p.Ingest(src, prober, nil)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !second.Cached {
		t.Fatal("second ingest cached = false, want true")
	}
	if second.Source.Probe == nil || *second.Source.Probe != prober.probe {
		t.Fatalf("cache-hit probe = %+v, want %+v", second.Source.Probe, prober.probe)
	}
	if prober.calls != 1 {
		t.Fatalf("prober ran %d times total, want 1 (no run on the cache hit)", prober.calls)
	}

	// The probe round-trips through the manifest, not just the in-memory result.
	m, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 1 || m.Sources[0].Probe == nil || *m.Sources[0].Probe != prober.probe {
		t.Fatalf("manifest probe = %+v, want one source carrying %+v", m.Sources, prober.probe)
	}
}

// TestIngestNilProberRecordsNoProbe pins the graceful path: with no prober the
// source is still recorded, just without a probe.
func TestIngestNilProberRecordsNoProbe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	res, err := p.Ingest(writeSource(t, "no worker here"), nil, nil)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Source.Probe != nil {
		t.Fatalf("probe = %+v, want nil with no prober", res.Source.Probe)
	}
}

// TestIngestRequiresInit asserts a step that needs the manifest refuses to run on
// an uninitialised directory rather than silently creating partial state.
func TestIngestRequiresInit(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, err := p.Ingest(writeSource(t, "x"), nil, nil); err != ErrNotInitialized {
		t.Fatalf("Ingest on uninit project = %v, want ErrNotInitialized", err)
	}
	if _, err := p.Status(); err != ErrNotInitialized {
		t.Fatalf("Status on uninit project = %v, want ErrNotInitialized", err)
	}
}

// TestTranscriptCacheRoundTrip is the unit-level proof that a transcript is
// content-addressed and idempotent: before it is written HasTranscript is false,
// after WriteTranscript the same bytes round-trip back through ReadTranscript,
// and the entry is namespaced away from the bare source cache key.
func TestTranscriptCacheRoundTrip(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	const hash = "d667df3e603bd8c3deadbeef"

	if p.HasTranscript(hash) {
		t.Fatal("HasTranscript = true before any transcript was written")
	}
	tr := Transcript{
		Source:   hash,
		Language: "en",
		Text:     "hello world",
		Words: []Word{
			{Word: "hello", Start: 0.0, End: 0.5},
			{Word: "world", Start: 0.5, End: 1.0},
		},
	}
	if err := p.WriteTranscript(hash, tr); err != nil {
		t.Fatal(err)
	}
	if !p.HasTranscript(hash) {
		t.Fatal("HasTranscript = false after WriteTranscript (cache miss on unchanged bytes)")
	}
	// The transcript cache key must not collide with the source record ingest
	// writes under the bare hash.
	if p.TranscriptPath(hash) == p.cachePath(hash) {
		t.Fatalf("transcript path %q collides with the source cache path", p.TranscriptPath(hash))
	}

	got, err := p.ReadTranscript(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != hash || got.Language != "en" || len(got.Words) != 2 {
		t.Fatalf("read transcript = %+v, want the written one", got)
	}
	if got.Words[1].Start != 0.5 || got.Words[1].End != 1.0 {
		t.Fatalf("word[1] timing = %+v, want start 0.5 end 1.0", got.Words[1])
	}
}

// TestSpeakersCacheRoundTrip is the unit-level proof that a speaker track is
// content-addressed and idempotent: before it is written HasSpeakers is false,
// after WriteSpeakers the same bytes round-trip back through ReadSpeakers, and the
// entry is namespaced away from both the bare source cache key and the transcript.
func TestSpeakersCacheRoundTrip(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	const hash = "d667df3e603bd8c3deadbeef"

	if p.HasSpeakers(hash) {
		t.Fatal("HasSpeakers = true before any analysis was written")
	}
	sp := Speakers{
		Source: hash,
		Layout: TwoPersonLayout,
		Turns: []Turn{
			{Speaker: "left", Start: 0.0, End: 1.5},
			{Speaker: "right", Start: 1.5, End: 3.0},
		},
	}
	if err := p.WriteSpeakers(hash, sp); err != nil {
		t.Fatal(err)
	}
	if !p.HasSpeakers(hash) {
		t.Fatal("HasSpeakers = false after WriteSpeakers (cache miss on unchanged bytes)")
	}
	// The speakers cache key must collide with neither the source record nor the
	// transcript entry ingest/transcribe write under the same hash.
	if p.SpeakersPath(hash) == p.cachePath(hash) || p.SpeakersPath(hash) == p.TranscriptPath(hash) {
		t.Fatalf("speakers path %q collides with another cache entry", p.SpeakersPath(hash))
	}

	got, err := p.ReadSpeakers(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != hash || got.Layout != TwoPersonLayout || len(got.Turns) != 2 {
		t.Fatalf("read speakers = %+v, want the written one", got)
	}
	if got.Turns[1].Speaker != "right" || got.Turns[1].Start != 1.5 || got.Turns[1].End != 3.0 {
		t.Fatalf("turn[1] = %+v, want {right 1.5 3.0}", got.Turns[1])
	}
}

// TestStatusReflectsArtifacts asserts the state graph flips an artifact from
// pending to done when its file appears on disk.
func TestStatusReflectsArtifacts(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}

	g, err := p.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Artifacts) != 1 || g.Artifacts[0].Name != "timeline" || g.Artifacts[0].Status != StatusPending {
		t.Fatalf("artifacts = %+v, want one pending timeline", g.Artifacts)
	}

	if err := os.WriteFile(p.TimelinePath(), []byte("<mlt/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err = p.Status()
	if err != nil {
		t.Fatal(err)
	}
	if g.Artifacts[0].Status != StatusDone {
		t.Fatalf("timeline status = %q, want done after the file exists", g.Artifacts[0].Status)
	}
}

// TestIndexCacheRoundTrip is the unit-level proof that an index is
// content-addressed and idempotent: before it is written HasIndex is false, after
// WriteIndex the same bytes round-trip back through ReadIndex, and the entry is
// namespaced away from both the bare source cache key and the transcript entry.
func TestIndexCacheRoundTrip(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	const hash = "d667df3e603bd8c3deadbeef"

	if p.HasIndex(hash) {
		t.Fatal("HasIndex = true before any index was written")
	}
	idx := Index{
		Source:   hash,
		Table:    p.IndexTablePath(hash),
		Segments: 2,
		Dim:      3,
		Model:    "fake-local-embed",
	}
	if err := p.WriteIndex(hash, idx); err != nil {
		t.Fatal(err)
	}
	if !p.HasIndex(hash) {
		t.Fatal("HasIndex = false after WriteIndex (cache miss on unchanged bytes)")
	}
	// The index cache key must not collide with the source record or transcript.
	if p.IndexPath(hash) == p.cachePath(hash) || p.IndexPath(hash) == p.TranscriptPath(hash) {
		t.Fatalf("index path %q collides with another cache entry", p.IndexPath(hash))
	}

	got, err := p.ReadIndex(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != hash || got.Segments != 2 || got.Dim != 3 || got.Model != "fake-local-embed" {
		t.Fatalf("read index = %+v, want the written one", got)
	}
	if got.Table != p.IndexTablePath(hash) {
		t.Fatalf("index table = %q, want %q", got.Table, p.IndexTablePath(hash))
	}
}

// TestRankCandidatesDeterministicOrder is the unit-level proof of search's
// determinism guarantee: RankCandidates imposes a total order (score desc, then
// start, end, text) so the same candidates always rank identically no matter
// what order they arrived in, and the input slice is left untouched.
func TestRankCandidatesDeterministicOrder(t *testing.T) {
	in := []Candidate{
		{Start: 5.0, End: 6.0, Text: "low", Score: 0.10},
		{Start: 2.0, End: 3.0, Text: "beta", Score: 0.50}, // ties on score with the next two
		{Start: 1.0, End: 2.0, Text: "alpha", Score: 0.50},
		{Start: 2.0, End: 3.0, Text: "aardvark", Score: 0.50}, // same start as beta, earlier text
		{Start: 0.0, End: 1.0, Text: "top", Score: 0.90},
	}
	// A defensive copy to prove RankCandidates does not mutate its input.
	orig := make([]Candidate, len(in))
	copy(orig, in)

	got := RankCandidates(in)

	want := []string{"top", "alpha", "aardvark", "beta", "low"}
	if len(got) != len(want) {
		t.Fatalf("ranked %d candidates, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Text != w {
			t.Fatalf("rank[%d] = %q, want %q (full order: %+v)", i, got[i].Text, w, got)
		}
	}
	// Score must be non-increasing across the ranking.
	for i := 1; i < len(got); i++ {
		if got[i-1].Score < got[i].Score {
			t.Fatalf("ranking not sorted by descending score at %d: %+v", i, got)
		}
	}
	// The input slice is untouched.
	for i := range orig {
		if in[i] != orig[i] {
			t.Fatalf("RankCandidates mutated its input at %d: got %+v want %+v", i, in[i], orig[i])
		}
	}

	// Determinism: ranking the same input twice yields the identical order.
	again := RankCandidates(orig)
	for i := range got {
		if got[i] != again[i] {
			t.Fatalf("RankCandidates not deterministic at %d: %+v vs %+v", i, got[i], again[i])
		}
	}
}

// The fingerprint shortcut exists to skip re-reading every byte of a large
// source, but it must never hand back a hash that does not match the file. The
// full sha256 stays the identity; the fingerprint only answers "same bytes as
// last time?". These cases pin both halves: the shortcut is used, and it is
// never used when the bytes changed.
func TestDescribeSourceFingerprintShortcut(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "the original bytes")

	first, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	// A second call must agree, and must agree with a from-scratch hash.
	second, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if second.Hash != first.Hash {
		t.Fatalf("shortcut changed the hash: %q vs %q", second.Hash, first.Hash)
	}
	direct, err := HashFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != direct {
		t.Fatalf("recorded hash %q != HashFile %q", first.Hash, direct)
	}

	// Rewriting the file must produce a different hash. This is the case that
	// matters: a shortcut that survived a content change would key every later
	// step — transcript, index, speakers, renders — to the wrong source.
	if err := os.WriteFile(src, []byte("completely different bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Hash == first.Hash {
		t.Fatal("a rewritten file kept its old hash — the fingerprint outlived the content")
	}
	if want, _ := HashFile(src); changed.Hash != want {
		t.Fatalf("hash after rewrite = %q, want %q", changed.Hash, want)
	}
}

// ForgetFingerprint is the escape hatch behind `ingest --rehash`: after it, the
// next describe reads every byte again rather than trusting the record.
func TestForgetFingerprintForcesAFullRehash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "bytes")
	got, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	// The mapping is on disk after the first describe.
	if entries, _ := filepath.Glob(filepath.Join(p.CacheDir(), "fp-*.txt")); len(entries) != 1 {
		t.Fatalf("expected exactly one fingerprint entry, got %d", len(entries))
	}
	if err := p.ForgetFingerprint(src); err != nil {
		t.Fatalf("ForgetFingerprint: %v", err)
	}
	if entries, _ := filepath.Glob(filepath.Join(p.CacheDir(), "fp-*.txt")); len(entries) != 0 {
		t.Fatalf("fingerprint survived ForgetFingerprint: %v", entries)
	}
	// Forgetting must not change the answer, only the work done to reach it.
	again, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if again.Hash != got.Hash {
		t.Fatalf("hash changed after a forced rehash: %q vs %q", again.Hash, got.Hash)
	}
	// Forgetting something never recorded is success, not an error.
	if err := p.ForgetFingerprint(src); err != nil {
		t.Fatalf("second ForgetFingerprint: %v", err)
	}
}

// The adversarial case for any cheap fingerprint: content replaced with the
// SAME byte count and the mtime forced back, so size and mtime both agree. Only
// the head/tail read can tell these apart, which is why the fingerprint hashes
// content at all rather than stat()ing and hoping.
func TestFingerprintCatchesSameSizeSameMtimeRewrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "AAAAAAAAAAAAAAAA")
	st, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	first, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}

	// Same length, different bytes, and the clock wound back to match.
	if err := os.WriteFile(src, []byte("BBBBBBBBBBBBBBBB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(src, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	now, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if now.Size() != st.Size() || !now.ModTime().Equal(st.ModTime()) {
		t.Fatalf("setup failed to match size+mtime: %d/%v vs %d/%v",
			now.Size(), now.ModTime(), st.Size(), st.ModTime())
	}

	changed, err := p.DescribeSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Hash == first.Hash {
		t.Fatal("same-size same-mtime rewrite kept the old hash — the fingerprint is not reading content")
	}
	if want, _ := HashFile(src); changed.Hash != want {
		t.Fatalf("hash = %q, want %q", changed.Hash, want)
	}
}
