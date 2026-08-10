// Package progress emits newline-delimited JSON progress events on a side
// channel (stderr) so a caller watching a long step can tell a slow run from a
// hung one. The events are deliberately kept off stdout: the CLI contract is
// that stdout carries exactly one JSON result, so progress rides stderr where
// it never corrupts the machine-readable result an agent parses.
//
// Each event is one JSON object on its own line (JSONL). A step opens with a
// "start" event, emits a "heartbeat" every interval while the worker runs, and
// closes with a "done" event (or "error" when the worker failed). The heartbeat
// is the whole point: its steady arrival is the signal that work is still
// advancing rather than wedged.
package progress

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Phases an event can carry.
const (
	PhaseStart     = "start"
	PhaseHeartbeat = "heartbeat"
	PhaseDone      = "done"
	PhaseError     = "error"
)

// kind labels every event so a reader can tell a progress line apart from any
// other JSON that might share the stream.
const kind = "progress"

// Event is one progress record. Seq counts up from 0 within a single step so a
// reader can order events and spot a gap; ElapsedS is seconds since the step
// started, the number that makes "slow" legible.
type Event struct {
	Kind     string  `json:"kind"`
	Step     string  `json:"step"`
	Phase    string  `json:"phase"`
	Seq      int     `json:"seq"`
	ElapsedS float64 `json:"elapsed_s"`
	Detail   string  `json:"detail,omitempty"`
}

// Reporter emits the events for one step to w. Writes are serialised so the
// heartbeat goroutine and the caller cannot interleave a half-written line.
// now is a clock seam so tests can pin elapsed times.
type Reporter struct {
	w     io.Writer
	step  string
	now   func() time.Time
	mu    sync.Mutex
	seq   int
	start time.Time
}

// New builds a Reporter that writes step's events to w. A nil w discards every
// event, so a caller with no side channel (a captured buffer, a closed pipe)
// stays a no-op rather than a panic.
func New(w io.Writer, step string) *Reporter {
	if w == nil {
		w = io.Discard
	}
	r := &Reporter{w: w, step: step, now: time.Now}
	r.start = r.now()
	return r
}

// Emit writes one event in the given phase carrying an optional detail. Each
// call takes the next sequence number and stamps the elapsed time since New.
func (r *Reporter) Emit(phase, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ev := Event{
		Kind:     kind,
		Step:     r.step,
		Phase:    phase,
		Seq:      r.seq,
		ElapsedS: r.now().Sub(r.start).Seconds(),
		Detail:   detail,
	}
	r.seq++
	// A single Encode writes the object and a trailing newline, which is exactly
	// the JSONL framing readers split on. An I/O error on the side channel is
	// deliberately swallowed: progress must never fail the step it reports.
	_ = json.NewEncoder(r.w).Encode(ev)
}

// defaultInterval is how often a step with no explicit cadence heartbeats. It is
// short enough that a hung minute is obvious, long enough that a healthy render
// does not bury its own result stream in beats.
const defaultInterval = 2 * time.Second

// While runs fn as the long step named step, emitting a "start" event before it
// begins, a "heartbeat" every interval while it runs, and a "done" (or "error")
// event when it returns. fn's error is returned unchanged so While drops
// transparently around an existing worker call. A non-positive interval falls
// back to the default cadence.
func While(w io.Writer, step string, interval time.Duration, fn func() error) error {
	if interval <= 0 {
		interval = defaultInterval
	}
	r := New(w, step)
	r.Emit(PhaseStart, "")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				r.Emit(PhaseHeartbeat, "")
			}
		}
	}()

	err := fn()

	close(stop)
	wg.Wait()

	if err != nil {
		r.Emit(PhaseError, err.Error())
		return err
	}
	r.Emit(PhaseDone, "")
	return nil
}
