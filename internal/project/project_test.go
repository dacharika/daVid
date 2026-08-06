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

	first, err := p.Ingest(src)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Cached {
		t.Fatalf("first ingest cached = true, want false")
	}
	if !p.CacheHit(first.Source.Hash) {
		t.Fatalf("cache entry missing after ingest of %q", first.Source.Hash)
	}

	second, err := p.Ingest(src)
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
	res, err := p.Ingest(other)
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

// TestIngestRequiresInit asserts a step that needs the manifest refuses to run on
// an uninitialised directory rather than silently creating partial state.
func TestIngestRequiresInit(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, err := p.Ingest(writeSource(t, "x")); err != ErrNotInitialized {
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
