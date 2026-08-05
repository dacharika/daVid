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
