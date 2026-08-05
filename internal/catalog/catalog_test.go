package catalog

import (
	"encoding/json"
	"testing"
)

// TestDefaultSeedsDescribe checks the scaffold catalog exposes the describe
// tool so both surfaces have at least one working operation.
func TestDefaultSeedsDescribe(t *testing.T) {
	c := Default()
	if len(c.Tools) == 0 {
		t.Fatal("default catalog has no tools")
	}
	if c.Tools[0].Name != "describe" {
		t.Fatalf("first tool = %q, want %q", c.Tools[0].Name, "describe")
	}
}

// TestDefaultMarshalsToJSON guards that the catalog is emittable as JSON, the
// form `cutvec describe` returns.
func TestDefaultMarshalsToJSON(t *testing.T) {
	if _, err := json.Marshal(Default()); err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
}
