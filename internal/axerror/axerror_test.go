package axerror

import (
	"encoding/json"
	"testing"
)

func TestErrorString(t *testing.T) {
	e := New("worker.missing", "python worker not found").WithHint("install workers")
	got := e.Error()
	want := "worker.missing: python worker not found (install workers)"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestAsRetryableIsCopy(t *testing.T) {
	base := New("net.timeout", "timed out")
	r := base.AsRetryable()
	if base.Retryable {
		t.Fatal("AsRetryable mutated the receiver")
	}
	if !r.Retryable {
		t.Fatal("returned error is not retryable")
	}
}

func TestMarshalShape(t *testing.T) {
	b, err := json.Marshal(New("code.x", "boom").AsRetryable())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"code", "message", "retryable"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %s", k, b)
		}
	}
}
