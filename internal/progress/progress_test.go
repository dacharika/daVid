package progress

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for the heartbeat goroutine to write while
// the test goroutine reads, so a heartbeat assertion never races the writer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// decodeEvents splits a JSONL stream into events, failing the test on any line
// that is not a well-formed Event object — the contract is one JSON object per
// line.
func decodeEvents(t *testing.T, s string) []Event {
	t.Helper()
	var evs []Event
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("progress line %q is not valid JSON: %v", line, err)
		}
		if ev.Kind != kind {
			t.Fatalf("event kind = %q, want %q (line %q)", ev.Kind, kind, line)
		}
		evs = append(evs, ev)
	}
	return evs
}

// TestEmitWritesOneJSONObjectPerLine pins the JSONL framing: each Emit is a
// single object on its own line, seq counts from 0, and the step travels on
// every event.
func TestEmitWritesOneJSONObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, "render")
	r.Emit(PhaseStart, "")
	r.Emit(PhaseHeartbeat, "still going")
	r.Emit(PhaseDone, "")

	evs := decodeEvents(t, buf.String())
	if len(evs) != 3 {
		t.Fatalf("got %d events, want 3: %q", len(evs), buf.String())
	}
	wantPhase := []string{PhaseStart, PhaseHeartbeat, PhaseDone}
	for i, ev := range evs {
		if ev.Step != "render" {
			t.Errorf("event %d step = %q, want render", i, ev.Step)
		}
		if ev.Phase != wantPhase[i] {
			t.Errorf("event %d phase = %q, want %q", i, ev.Phase, wantPhase[i])
		}
		if ev.Seq != i {
			t.Errorf("event %d seq = %d, want %d", i, ev.Seq, i)
		}
	}
	if evs[1].Detail != "still going" {
		t.Errorf("heartbeat detail = %q, want %q", evs[1].Detail, "still going")
	}
}

// TestEmitElapsedUsesTheClockSeam proves elapsed_s is measured, not guessed:
// with a pinned clock the second event reports the exact gap.
func TestEmitElapsedUsesTheClockSeam(t *testing.T) {
	var buf bytes.Buffer
	base := time.Unix(1000, 0)
	clock := base
	r := New(&buf, "transcribe")
	r.now = func() time.Time { return clock }
	r.start = base

	r.Emit(PhaseStart, "")
	clock = base.Add(3 * time.Second)
	r.Emit(PhaseHeartbeat, "")

	evs := decodeEvents(t, buf.String())
	if evs[0].ElapsedS != 0 {
		t.Errorf("start elapsed_s = %v, want 0", evs[0].ElapsedS)
	}
	if evs[1].ElapsedS != 3 {
		t.Errorf("heartbeat elapsed_s = %v, want 3", evs[1].ElapsedS)
	}
}

// TestWhileBracketsFnWithStartAndDone shows the happy path frames fn between a
// start and a done event and returns fn's nil error.
func TestWhileBracketsFnWithStartAndDone(t *testing.T) {
	var buf bytes.Buffer
	ran := false
	err := While(&buf, "speakers", time.Second, func() error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatalf("While = %v, want nil", err)
	}
	if !ran {
		t.Fatal("fn did not run")
	}
	evs := decodeEvents(t, buf.String())
	if len(evs) < 2 {
		t.Fatalf("got %d events, want at least start+done: %q", len(evs), buf.String())
	}
	if evs[0].Phase != PhaseStart {
		t.Errorf("first phase = %q, want start", evs[0].Phase)
	}
	last := evs[len(evs)-1]
	if last.Phase != PhaseDone {
		t.Errorf("last phase = %q, want done", last.Phase)
	}
	for _, ev := range evs {
		if ev.Step != "speakers" {
			t.Errorf("event step = %q, want speakers", ev.Step)
		}
	}
}

// TestWhileHeartbeatsWhileFnRuns proves the heartbeat actually fires during a
// running step. fn blocks until it observes a heartbeat line (with a generous
// safety deadline), so the assertion is driven by real emission, not a sleep.
func TestWhileHeartbeatsWhileFnRuns(t *testing.T) {
	var buf syncBuffer
	err := While(&buf, "render", time.Millisecond, func() error {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if strings.Contains(buf.String(), `"phase":"heartbeat"`) {
				return nil
			}
			time.Sleep(time.Millisecond)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("While = %v, want nil", err)
	}
	evs := decodeEvents(t, buf.String())
	beats := 0
	for _, ev := range evs {
		if ev.Phase == PhaseHeartbeat {
			beats++
		}
	}
	if beats == 0 {
		t.Fatalf("no heartbeat events emitted: %q", buf.String())
	}
}

// TestWhileEmitsErrorAndReturnsIt shows a failing step closes with an error
// event carrying the message and passes the error through unchanged.
func TestWhileEmitsErrorAndReturnsIt(t *testing.T) {
	var buf bytes.Buffer
	sentinel := errString("melt exploded")
	err := While(&buf, "render", time.Second, func() error {
		return sentinel
	})
	if err != sentinel {
		t.Fatalf("While = %v, want the sentinel error", err)
	}
	evs := decodeEvents(t, buf.String())
	last := evs[len(evs)-1]
	if last.Phase != PhaseError {
		t.Fatalf("last phase = %q, want error", last.Phase)
	}
	if last.Detail != "melt exploded" {
		t.Errorf("error detail = %q, want %q", last.Detail, "melt exploded")
	}
}

// TestNilWriterIsNoOp guards the discard path: a nil side channel must not panic.
func TestNilWriterIsNoOp(t *testing.T) {
	r := New(nil, "render")
	r.Emit(PhaseStart, "") // must not panic
	if err := While(nil, "render", time.Second, func() error { return nil }); err != nil {
		t.Fatalf("While(nil) = %v, want nil", err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
