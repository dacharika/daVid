package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tahabsn/cutvec/internal/axerror"
)

// A timeline melt cannot load must come back ok:false with the reason. The
// reason this needs its own path at all is that melt EXITS 0 when a producer
// fails to load — measured — so a preflight that trusted the exit code would
// report success on a broken edit. That is worse than having no preflight,
// because it manufactures confidence right before a long render.
func TestTimelineVerifyReportsALoadFailure(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "edit-broken.mlt")
	if err := os.WriteFile(broken, []byte("<mlt/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	wpath, wargs := renderWorkerEnv(t, "verify")
	body, _ := json.Marshal(map[string]any{
		"timeline": broken, "worker": wpath, "workerArgs": wargs,
	})

	var out bytes.Buffer
	if err := run([]string{"timeline", "verify"}, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("verify returned an error: %v", err)
	}
	var res verifyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out.String())
	}
	if res.OK {
		t.Fatalf("a timeline melt cannot load reported ok:true — the exit code was trusted:\n%s", out.String())
	}
	if len(res.Problems) == 0 {
		t.Fatal("ok:false with no problems — an agent is told it failed but not why")
	}
	if !strings.Contains(res.Problems[0], "failed to load producer") {
		t.Errorf("problem %q does not name the load failure", res.Problems[0])
	}
}

// The happy path is an answer, not an error: a timeline that loads reports
// ok:true with no problems, plus what the check cost.
func TestTimelineVerifyPassesAGoodTimeline(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "edit.mlt")
	if err := os.WriteFile(good, []byte("<mlt/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	wpath, wargs := renderWorkerEnv(t, "verify")
	body, _ := json.Marshal(map[string]any{
		"timeline": good, "worker": wpath, "workerArgs": wargs,
	})

	var out bytes.Buffer
	if err := run([]string{"timeline", "verify"}, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var res verifyResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(res.Problems) != 0 {
		t.Fatalf("good timeline reported %+v", res)
	}
	// The cost is reported so the tradeoff against a full render is visible.
	if res.Frames == 0 || res.Seconds == 0 {
		t.Errorf("verify did not report what it cost: %+v", res)
	}
}

func TestTimelineVerifyFailureRules(t *testing.T) {
	t.Run("no project and no timeline", func(t *testing.T) {
		var out bytes.Buffer
		err := run([]string{"timeline", "verify"}, strings.NewReader("{}"), &out)
		var ae *axerror.Error
		if !errors.As(err, &ae) || ae.Code != "input.invalid" {
			t.Fatalf("error = %v, want input.invalid", err)
		}
	})

	t.Run("uninitialised project", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"project": t.TempDir()})
		var out bytes.Buffer
		err := run([]string{"timeline", "verify"}, bytes.NewReader(body), &out)
		var ae *axerror.Error
		if !errors.As(err, &ae) || ae.Code != "project.uninitialized" {
			t.Fatalf("error = %v, want project.uninitialized", err)
		}
	})
}
