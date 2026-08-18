package doctor

import (
	"path/filepath"
	"testing"
)

// fakeProbe is a scripted Probe: every primitive answers from a fixed table so
// a whole report is deterministic without any real binary, python, or model on
// the test host.
type fakeProbe struct {
	path    map[string]string // binaries resolvable on PATH
	files   map[string]bool   // files that exist
	env     map[string]string // environment
	home    string
	modules map[string]bool // importable python modules
}

func (f fakeProbe) LookPath(name string) (string, bool) {
	p, ok := f.path[name]
	return p, ok
}
func (f fakeProbe) FileExists(path string) bool { return f.files[path] }
func (f fakeProbe) Getenv(key string) string    { return f.env[key] }
func (f fakeProbe) Home() string                { return f.home }
func (f fakeProbe) HasModule(name string) bool  { return f.modules[name] }

// fullyProvisioned returns a probe under which every real spec is satisfied, so
// tests can knock out one dependency at a time and assert only that gap shows.
func fullyProvisioned() fakeProbe {
	home := "/home/u"
	f := fakeProbe{
		path:    map[string]string{},
		files:   map[string]bool{},
		env:     map[string]string{},
		home:    home,
		modules: map[string]bool{},
	}
	for _, s := range specs() {
		switch s.kind {
		case KindBinary:
			f.path[s.name] = "/usr/bin/" + s.name
		case KindPackage:
			// Satisfy whichever module names the spec accepts by probing it.
		case KindWorker:
			// handled below via env
		case KindModel:
			// handled below via env
		}
	}
	// Workers resolve from their env var to an existing file.
	f.env["DAVID_FFMPEG_WORKER"] = "/w/ffmpeg"
	f.env["DAVID_WHISPER_WORKER"] = "/w/whisper"
	f.env["DAVID_INDEX_WORKER"] = "/w/index"
	f.env["DAVID_SEARCH_WORKER"] = "/w/search"
	f.env["DAVID_MEDIAPIPE_WORKER"] = "/w/mediapipe"
	f.env["DAVID_MELT_WORKER"] = "/w/melt"
	for _, p := range []string{"/w/ffmpeg", "/w/whisper", "/w/index", "/w/search", "/w/mediapipe", "/w/melt"} {
		f.files[p] = true
	}
	// Models resolve from their env var to an existing file.
	f.env["DAVID_WHISPER_MODEL"] = "/m/ggml.bin"
	f.env["DAVID_FACE_LANDMARKER"] = "/m/face.task"
	f.files["/m/ggml.bin"] = true
	f.files["/m/face.task"] = true
	// Every python module the packages ask for is importable.
	for _, m := range []string{"lancedb", "mediapipe", "numpy", "cv2", "fastembed", "sentence_transformers"} {
		f.modules[m] = true
	}
	return f
}

func TestRunReportsEveryKind(t *testing.T) {
	rep := Run(fullyProvisioned())
	if !rep.Ok {
		t.Fatalf("fully provisioned env should pass, missing=%v", rep.MissingNames)
	}
	if rep.Missing != 0 || rep.RequiredMissing != 0 {
		t.Fatalf("expected 0 missing, got missing=%d requiredMissing=%d", rep.Missing, rep.RequiredMissing)
	}
	kinds := map[string]bool{}
	for _, c := range rep.Checks {
		kinds[c.Kind] = true
		if c.Status != StatusOK {
			t.Errorf("%s: status = %q, want ok", c.Name, c.Status)
		}
	}
	for _, want := range []string{KindBinary, KindWorker, KindPackage, KindModel} {
		if !kinds[want] {
			t.Errorf("report has no %q check", want)
		}
	}
}

func TestMissingBinaryFailsPreflightWithHint(t *testing.T) {
	f := fullyProvisioned()
	delete(f.path, "melt")
	rep := Run(f)
	if rep.Ok {
		t.Fatal("missing melt should fail the preflight")
	}
	if rep.RequiredMissing != 1 || len(rep.MissingNames) != 1 || rep.MissingNames[0] != "melt" {
		t.Fatalf("expected exactly melt missing, got %+v", rep.MissingNames)
	}
	c := find(t, rep, "melt")
	if c.Status != StatusMissing || c.Hint == "" {
		t.Fatalf("melt check = %+v, want missing with a hint", c)
	}
}

func TestMissingModuleReported(t *testing.T) {
	f := fullyProvisioned()
	delete(f.modules, "lancedb")
	rep := Run(f)
	c := find(t, rep, "lancedb")
	if c.Status != StatusMissing {
		t.Fatalf("lancedb should be missing, got %q", c.Status)
	}
	if c.Hint == "" {
		t.Fatal("missing package should carry an install hint")
	}
}

func TestEmbeddingBackendSatisfiedByEither(t *testing.T) {
	f := fullyProvisioned()
	// Drop fastembed but keep the sentence-transformers fallback.
	delete(f.modules, "fastembed")
	rep := Run(f)
	if !rep.Ok {
		t.Fatalf("sentence-transformers alone should satisfy the backend, missing=%v", rep.MissingNames)
	}
	// Now drop both: the backend check must fail.
	delete(f.modules, "sentence_transformers")
	rep = Run(f)
	if rep.Ok {
		t.Fatal("no embedding backend should fail the preflight")
	}
}

func TestWorkerMissingWhenEnvUnset(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_MELT_WORKER")
	rep := Run(f)
	c := find(t, rep, "melt worker")
	if c.Status != StatusMissing {
		t.Fatalf("melt worker should be missing when its env var is unset, got %q", c.Status)
	}
	if c.Hint == "" {
		t.Fatal("missing worker should carry a hint")
	}
}

func TestWorkerMissingWhenPathDoesNotExist(t *testing.T) {
	f := fullyProvisioned()
	f.env["DAVID_MELT_WORKER"] = "/w/gone"
	// /w/gone is not in files, so it does not exist.
	rep := Run(f)
	c := find(t, rep, "melt worker")
	if c.Status != StatusMissing {
		t.Fatalf("worker pointing at a nonexistent file should be missing, got %q", c.Status)
	}
	if c.Detail != "/w/gone" {
		t.Fatalf("detail should surface the configured-but-missing path, got %q", c.Detail)
	}
}

func TestModelFoundAtWellKnownPathWithoutEnv(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_WHISPER_MODEL")
	// Provision one of the well-known home-relative locations instead.
	rel := filepath.Join(f.home, "Library/Application Support/podcli/models/ggml-base.bin")
	f.files[rel] = true
	rep := Run(f)
	c := find(t, rep, "whisper ggml model")
	if c.Status != StatusOK {
		t.Fatalf("whisper model should resolve at a well-known path, got %q", c.Status)
	}
	if c.Detail != rel {
		t.Fatalf("detail should be the resolved path, got %q", c.Detail)
	}
}

func TestOptionalMissDoesNotFailPreflight(t *testing.T) {
	// Drive the report logic directly with a crafted spec set to prove an
	// optional gap is reported but does not flip Ok.
	list := []spec{
		{name: "req", kind: KindBinary, required: true, hint: "h",
			probe: func(Probe) (bool, string) { return true, "" }},
		{name: "opt", kind: KindPackage, required: false, hint: "h",
			probe: func(Probe) (bool, string) { return false, "" }},
	}
	rep := evaluate(list, fakeProbe{})
	if !rep.Ok {
		t.Fatal("an optional miss must not fail the preflight")
	}
	if rep.Missing != 1 || rep.RequiredMissing != 0 {
		t.Fatalf("missing=%d requiredMissing=%d, want 1/0", rep.Missing, rep.RequiredMissing)
	}
}

// TestFaceLandmarkerFoundAtNewDaVidPath verifies the primary lookup path.
func TestFaceLandmarkerFoundAtNewDaVidPath(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_FACE_LANDMARKER")
	delete(f.env, "CUTVEC_FACE_LANDMARKER")
	newPath := filepath.Join(f.home, "Library/Application Support/daVid/models/face_landmarker.task")
	f.files[newPath] = true
	rep := Run(f)
	c := find(t, rep, "mediapipe face_landmarker")
	if c.Status != StatusOK {
		t.Fatalf("face_landmarker should resolve at daVid path, got %q (detail=%q)", c.Status, c.Detail)
	}
	if c.Detail != newPath {
		t.Fatalf("detail should be the daVid path, got %q", c.Detail)
	}
}

// TestFaceLandmarkerLegacyFallback verifies the cutvec legacy path is used when
// the new daVid path does not exist.
func TestFaceLandmarkerLegacyFallback(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_FACE_LANDMARKER")
	delete(f.env, "CUTVEC_FACE_LANDMARKER")
	legacyPath := filepath.Join(f.home, "Library/Application Support/cutvec/models/face_landmarker.task")
	f.files[legacyPath] = true
	rep := Run(f)
	c := find(t, rep, "mediapipe face_landmarker")
	if c.Status != StatusOK {
		t.Fatalf("face_landmarker should resolve at legacy cutvec path, got %q (detail=%q)", c.Status, c.Detail)
	}
	if c.Detail != legacyPath {
		t.Fatalf("detail should be the legacy path, got %q", c.Detail)
	}
}

// TestFaceLandmarkerMissingBothPaths verifies the check fails when neither path
// exists and no env var is set.
func TestFaceLandmarkerMissingBothPaths(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_FACE_LANDMARKER")
	delete(f.env, "CUTVEC_FACE_LANDMARKER")
	// ensure neither well-known path is present
	delete(f.files, filepath.Join(f.home, "Library/Application Support/daVid/models/face_landmarker.task"))
	delete(f.files, filepath.Join(f.home, "Library/Application Support/cutvec/models/face_landmarker.task"))
	rep := Run(f)
	c := find(t, rep, "mediapipe face_landmarker")
	if c.Status != StatusMissing {
		t.Fatalf("face_landmarker should be missing when both paths absent, got %q", c.Status)
	}
	if c.Hint == "" {
		t.Fatal("missing face_landmarker should carry an install hint")
	}
}

// TestMigrationRecommendationEmittedWhenOnlyLegacyExists verifies that the
// doctor report carries a migration recommendation when only the legacy cutvec
// path exists and the new daVid path does not.
func TestMigrationRecommendationEmittedWhenOnlyLegacyExists(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "DAVID_FACE_LANDMARKER")
	delete(f.env, "CUTVEC_FACE_LANDMARKER")
	legacyPath := filepath.Join(f.home, "Library/Application Support/cutvec/models/face_landmarker.task")
	f.files[legacyPath] = true
	rep := Run(f)
	if len(rep.Recommendations) == 0 {
		t.Fatal("expected at least one migration recommendation, got none")
	}
	found := false
	for _, r := range rep.Recommendations {
		if len(r) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("migration recommendation is empty")
	}
}

// TestNoMigrationRecommendationWhenNewPathExists verifies no spurious
// recommendation is emitted when the new daVid path is present.
func TestNoMigrationRecommendationWhenNewPathExists(t *testing.T) {
	f := fullyProvisioned()
	delete(f.env, "CUTVEC_FACE_LANDMARKER")
	newPath := filepath.Join(f.home, "Library/Application Support/daVid/models/face_landmarker.task")
	f.files[newPath] = true
	rep := Run(f)
	if len(rep.Recommendations) != 0 {
		t.Fatalf("expected no recommendations when new path exists, got %v", rep.Recommendations)
	}
}

func find(t *testing.T, rep Report, name string) Check {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q in report", name)
	return Check{}
}
