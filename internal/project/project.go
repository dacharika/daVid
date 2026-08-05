// Package project models cutvec's explicit project-directory state: the on-disk
// home for proxies, renders, the content-hash cache, and step results that make
// runs deterministic, idempotent, and resumable. Handles passed between tool
// calls resolve to paths under a Project, never to inlined blobs.
package project

import "path/filepath"

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
