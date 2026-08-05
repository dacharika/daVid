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

	first, err := p.Ingest(src, nil)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Cached {
		t.Fatalf("first ingest cached = true, want false")
	}
	if !p.CacheHit(first.Source.Hash) {
		t.Fatalf("cache entry missing after ingest of %q", first.Source.Hash)
	}

	second, err := p.Ingest(src, nil)
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
	res, err := p.Ingest(other, nil)
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

// countingArtifactor is a fake project.Artifactor that records how many times it
// was invoked and returns canned proxy/audio/probe artifacts under cacheDir. It
// stands in for the ffmpeg worker so the seam can be tested with no toolchain.
type countingArtifactor struct {
	calls int
}

func (c *countingArtifactor) BuildArtifacts(src Source, cacheDir string) (Artifacts, error) {
	c.calls++
	proxy := filepath.Join(cacheDir, src.Hash+".proxy.mp4")
	audio := filepath.Join(cacheDir, src.Hash+".audio.wav")
	return Artifacts{
		Proxy: proxy,
		Audio: audio,
		Probe: map[string]any{"duration": 3.5},
	}, nil
}

// TestIngestRunsArtifactorOncePerHash proves the seam: on a cache miss the
// Artifactor builds the source's proxy/audio/probe and they are recorded on the
// Source; on a re-ingest of identical bytes the Artifactor is NOT called again
// (cache hit) yet the same handles come back — probe+proxy+audio are built once
// and cached by content hash.
func TestIngestRunsArtifactorOncePerHash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	p := Open(dir)
	if _, _, err := p.Init(""); err != nil {
		t.Fatal(err)
	}
	src := writeSource(t, "media bytes")
	a := &countingArtifactor{}

	first, err := p.Ingest(src, a)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if a.calls != 1 {
		t.Fatalf("artifactor called %d times on miss, want 1", a.calls)
	}
	if first.Source.Proxy == "" || first.Source.Audio == "" || first.Source.Probe["duration"] != 3.5 {
		t.Fatalf("source missing artifacts after miss: %+v", first.Source)
	}

	second, err := p.Ingest(src, a)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if a.calls != 1 {
		t.Fatalf("artifactor called %d times total, want 1 (cache hit must skip it)", a.calls)
	}
	if !second.Cached {
		t.Fatalf("second ingest cached = false, want true")
	}
	if second.Source.Proxy != first.Source.Proxy || second.Source.Audio != first.Source.Audio {
		t.Fatalf("cached handles drifted: %+v vs %+v", first.Source, second.Source)
	}
	if second.Source.Probe["duration"] != 3.5 {
		t.Fatalf("cached probe lost: %+v", second.Source.Probe)
	}
}

// TestIngestRequiresInit asserts a step that needs the manifest refuses to run on
// an uninitialised directory rather than silently creating partial state.
func TestIngestRequiresInit(t *testing.T) {
	p := Open(filepath.Join(t.TempDir(), "proj"))
	if _, err := p.Ingest(writeSource(t, "x"), nil); err != ErrNotInitialized {
		t.Fatalf("Ingest on uninit project = %v, want ErrNotInitialized", err)
	}
	if _, err := p.Status(); err != ErrNotInitialized {
		t.Fatalf("Status on uninit project = %v, want ErrNotInitialized", err)
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
