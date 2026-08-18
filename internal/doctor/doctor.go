// Package doctor preflights cutvec's runtime environment in one pass: every
// external binary, python worker, ML package, and model file the podcast-edit
// pipeline depends on is probed and reported as ok or missing with an actionable
// install hint. It is pure inspection — no worker is invoked and no project is
// touched — so an agent can call it before a pipeline spends minutes failing
// halfway on a dependency that was never installed.
//
// The checklist is authoritative and lives here (self-describing, like the tool
// catalog); how each item is probed against the real machine is injected through
// the Probe seam so the whole report is unit-testable without ffmpeg, melt,
// python, or any model on the test host.
package doctor

import (
	"fmt"
	"path/filepath"
)

// Check statuses. A check is either satisfied or not; there is no partial state.
const (
	StatusOK      = "ok"
	StatusMissing = "missing"
)

// Check kinds group the report by the class of dependency, so a reader can see
// at a glance whether the gap is a binary, a worker, a python package, or a
// model file.
const (
	KindBinary  = "binary"
	KindWorker  = "worker"
	KindPackage = "package"
	KindModel   = "model"
)

// Probe is the seam between the doctor's checklist and the real machine. Each
// method answers one primitive question about the host; the doctor composes them
// into the full report. Injecting this makes every check deterministic in tests.
type Probe interface {
	// LookPath resolves an executable on PATH, returning its path and whether
	// it was found.
	LookPath(name string) (string, bool)
	// FileExists reports whether an ordinary file exists at path.
	FileExists(path string) bool
	// Getenv returns an environment variable's value ("" when unset).
	Getenv(key string) string
	// Home is the current user's home directory, used to build the well-known
	// model install locations.
	Home() string
	// HasModule reports whether a python module can be imported (find_spec).
	HasModule(name string) bool
}

// Check is one evaluated environment requirement.
type Check struct {
	// Name identifies the dependency (e.g. "ffmpeg", "lancedb").
	Name string `json:"name"`
	// Kind is one of the Kind* constants.
	Kind string `json:"kind"`
	// Required is true when a pipeline cannot run without this dependency; a
	// missing required check fails the whole preflight.
	Required bool `json:"required"`
	// Status is StatusOK or StatusMissing.
	Status string `json:"status"`
	// Detail carries the resolved path or a short note when present.
	Detail string `json:"detail,omitempty"`
	// Hint tells the caller how to install or configure a missing dependency.
	Hint string `json:"hint,omitempty"`
}

// Report is the whole-environment preflight result.
type Report struct {
	// Ok is true iff no required check is missing.
	Ok bool `json:"ok"`
	// Checks are every dependency probed, in checklist order.
	Checks []Check `json:"checks"`
	// Missing counts every missing check (required or not).
	Missing int `json:"missing"`
	// RequiredMissing counts only the missing checks that fail the preflight.
	RequiredMissing int `json:"requiredMissing"`
	// MissingNames lists the missing required checks, for a one-line summary.
	MissingNames []string `json:"missingNames,omitempty"`
	// Recommendations lists advisory migration actions. A non-empty list does
	// not affect Ok — the environment works, but an upgrade is suggested.
	Recommendations []string `json:"recommendations,omitempty"`
}

// spec is one requirement plus how to probe it. probe returns whether the
// requirement is satisfied and an optional detail string (a resolved path or a
// short note) to surface either way. advisory, when set, is called after the
// probe and returns a non-empty migration recommendation when action is needed.
type spec struct {
	name     string
	kind     string
	required bool
	hint     string
	probe    func(p Probe) (ok bool, detail string)
	advisory func(p Probe) string
}

// Run probes the real (injected) environment and returns the full report.
func Run(p Probe) Report {
	return evaluate(specs(), p)
}

// evaluate runs each spec against the probe and assembles the report. It is
// separated from the fixed checklist so tests can drive the report logic with a
// crafted spec set (e.g. to prove an optional miss does not fail the preflight).
func evaluate(list []spec, p Probe) Report {
	rep := Report{Ok: true, Checks: make([]Check, 0, len(list))}
	for _, s := range list {
		ok, detail := s.probe(p)
		c := Check{Name: s.name, Kind: s.kind, Required: s.required, Detail: detail}
		if ok {
			c.Status = StatusOK
		} else {
			c.Status = StatusMissing
			c.Hint = s.hint
			rep.Missing++
			if s.required {
				rep.RequiredMissing++
				rep.MissingNames = append(rep.MissingNames, s.name)
				rep.Ok = false
			}
		}
		rep.Checks = append(rep.Checks, c)
		if s.advisory != nil {
			if rec := s.advisory(p); rec != "" {
				rep.Recommendations = append(rep.Recommendations, rec)
			}
		}
	}
	return rep
}

// binary builds a spec that is satisfied when name resolves on PATH.
func binary(name, hint string) spec {
	return spec{
		name: name, kind: KindBinary, required: true, hint: hint,
		probe: func(p Probe) (bool, string) {
			if path, ok := p.LookPath(name); ok {
				return true, path
			}
			return false, ""
		},
	}
}

// pkg builds a spec that is satisfied when any of the named python modules is
// importable. Passing several names expresses an interchangeable backend (the
// index/search workers embed with fastembed *or* sentence-transformers).
func pkg(display, hint string, modules ...string) spec {
	return spec{
		name: display, kind: KindPackage, required: true, hint: hint,
		probe: func(p Probe) (bool, string) {
			for _, m := range modules {
				if p.HasModule(m) {
					return true, m
				}
			}
			return false, ""
		},
	}
}

// worker builds a spec that is satisfied when the env var naming the worker
// executable is set and points at a file that exists. canonicalEnv is the
// DAVID_* var; legacyEnv is the deprecated CUTVEC_* var accepted during the
// deprecation window. DAVID_* wins when both are set. The Probe seam is used
// for both lookups so the whole check is testable without touching os.Getenv.
func worker(display, canonicalEnv, legacyEnv, hint string) spec {
	return spec{
		name: display, kind: KindWorker, required: true, hint: hint,
		probe: func(p Probe) (bool, string) {
			path := p.Getenv(canonicalEnv)
			if path == "" {
				path = p.Getenv(legacyEnv)
			}
			if path == "" {
				return false, ""
			}
			if p.FileExists(path) {
				return true, path
			}
			return false, path
		},
	}
}

// model builds a spec satisfied when the model file is found, checked first at
// the path in canonicalEnv (DAVID_*), then legacyEnv (CUTVEC_*, deprecation
// window), and finally at each well-known install location under the user's
// home directory. The Probe seam is used for both lookups.
func model(display, canonicalEnv, legacyEnv, hint string, relPaths ...string) spec {
	return spec{
		name: display, kind: KindModel, required: true, hint: hint,
		probe: func(p Probe) (bool, string) {
			v := p.Getenv(canonicalEnv)
			if v == "" {
				v = p.Getenv(legacyEnv)
			}
			if v != "" {
				if p.FileExists(v) {
					return true, v
				}
				return false, v
			}
			for _, rel := range relPaths {
				full := rel
				if !filepath.IsAbs(rel) {
					full = filepath.Join(p.Home(), rel)
				}
				if p.FileExists(full) {
					return true, full
				}
			}
			return false, ""
		},
	}
}

// specs is the authoritative checklist: every dependency the full podcast-edit
// pipeline (ingest, transcribe, index, search, speakers, render) needs, grouped
// by kind. Each carries the same install hint the worker itself reports when the
// dependency is missing mid-run, so the doctor and a live failure agree.
func specs() []spec {
	return []spec{
		// External binaries the workers shell out to.
		binary("python3", "install Python 3 — every ML worker runs as a python subprocess"),
		binary("ffmpeg", "brew install ffmpeg"),
		binary("ffprobe", "brew install ffmpeg (provides ffprobe)"),
		binary("whisper-cli", "brew install whisper-cpp (provides whisper-cli)"),
		binary("melt", "brew install mlt (provides melt, the MLT XML renderer)"),

		// Python worker executables, configured per the env vars the CLI reads.
		worker("ffmpeg worker", "DAVID_FFMPEG_WORKER", "CUTVEC_FFMPEG_WORKER",
			"set DAVID_FFMPEG_WORKER to the ffmpeg probe worker (see workers/ffmpeg_worker.py)"),
		worker("whisper worker", "DAVID_WHISPER_WORKER", "CUTVEC_WHISPER_WORKER",
			"set DAVID_WHISPER_WORKER to the whisper transcribe worker (see workers/whisper_worker.py)"),
		worker("index worker", "DAVID_INDEX_WORKER", "CUTVEC_INDEX_WORKER",
			"set DAVID_INDEX_WORKER to the embedding index worker (see workers/index_worker.py)"),
		worker("search worker", "DAVID_SEARCH_WORKER", "CUTVEC_SEARCH_WORKER",
			"set DAVID_SEARCH_WORKER to the vector search worker (see workers/search_worker.py)"),
		worker("mediapipe worker", "DAVID_MEDIAPIPE_WORKER", "CUTVEC_MEDIAPIPE_WORKER",
			"set DAVID_MEDIAPIPE_WORKER to the active-speaker worker (see workers/mediapipe_worker.py)"),
		worker("melt worker", "DAVID_MELT_WORKER", "CUTVEC_MELT_WORKER",
			"set DAVID_MELT_WORKER to the melt render worker (see workers/melt_worker.py)"),

		// Python packages the ML workers import.
		pkg("lancedb", "pip install lancedb", "lancedb"),
		pkg("mediapipe", "pip install mediapipe", "mediapipe"),
		pkg("numpy", "pip install numpy", "numpy"),
		pkg("opencv-python", "pip install opencv-python", "cv2"),
		pkg("embedding backend (fastembed|sentence-transformers)",
			"pip install fastembed (or sentence-transformers) — cutvec embeds locally, no API key required",
			"fastembed", "sentence_transformers"),

		// Model files the transcribe and speakers workers load.
		model("whisper ggml model", "DAVID_WHISPER_MODEL", "CUTVEC_WHISPER_MODEL",
			"set DAVID_WHISPER_MODEL to a ggml-*.bin, e.g. from https://huggingface.co/ggerganov/whisper.cpp",
			"Library/Application Support/podcli/models/ggml-base.bin",
			"/opt/homebrew/share/whisper-cpp/ggml-base.bin",
			".cache/whisper/ggml-base.bin"),
		faceLandmarkerSpec(),
	}
}

// faceLandmarkerSpec probes for the MediaPipe face_landmarker.task model,
// checking the canonical daVid path first and falling back to the legacy cutvec
// path. When only the legacy path exists the spec emits a migration
// recommendation so the caller can surface it without failing the preflight.
func faceLandmarkerSpec() spec {
	const (
		newPath    = "Library/Application Support/daVid/models/face_landmarker.task"
		legacyPath = "Library/Application Support/cutvec/models/face_landmarker.task"
		altPath    = "Downloads/podcast_edit/models/face_landmarker.task"
		envNew     = "DAVID_FACE_LANDMARKER"
		envLegacy  = "CUTVEC_FACE_LANDMARKER"
		hint       = "set DAVID_FACE_LANDMARKER, or download face_landmarker.task from https://storage.googleapis.com/mediapipe-models/face_landmarker/"
	)
	return spec{
		name: "mediapipe face_landmarker", kind: KindModel, required: true, hint: hint,
		probe: func(p Probe) (bool, string) {
			env := p.Getenv(envNew)
			if env == "" {
				env = p.Getenv(envLegacy)
			}
			if env != "" {
				if p.FileExists(env) {
					return true, env
				}
				return false, env
			}
			for _, rel := range []string{newPath, altPath, legacyPath} {
				full := filepath.Join(p.Home(), rel)
				if p.FileExists(full) {
					return true, full
				}
			}
			return false, ""
		},
		advisory: func(p Probe) string {
			if p.Getenv(envNew) != "" || p.Getenv(envLegacy) != "" {
				return ""
			}
			newFull := filepath.Join(p.Home(), newPath)
			legacyFull := filepath.Join(p.Home(), legacyPath)
			if !p.FileExists(newFull) && p.FileExists(legacyFull) {
				return fmt.Sprintf(
					"migrate face_landmarker.task: copy %s to %s — the cutvec path is deprecated in favour of daVid",
					legacyFull, newFull,
				)
			}
			return ""
		},
	}
}
