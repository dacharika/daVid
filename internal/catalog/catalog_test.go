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

// TestEveryToolIsFullyTyped is the AC1 guard: describe --json returns an array
// of tools each with a name, a summary, and both an input and output schema.
// Every seeded tool must carry all four so no command is discoverable without
// its typed IO.
func TestEveryToolIsFullyTyped(t *testing.T) {
	for _, tool := range Default().Tools {
		if tool.Name == "" {
			t.Errorf("a tool has no name: %+v", tool)
		}
		if tool.Summary == "" {
			t.Errorf("tool %q has no summary", tool.Name)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("tool %q has no input schema", tool.Name)
		}
		if len(tool.OutputSchema) == 0 {
			t.Errorf("tool %q has no output schema", tool.Name)
		}
	}
}

// TestLookup resolves known and unknown commands, the operation behind
// `cutvec schema <cmd>`.
func TestLookup(t *testing.T) {
	c := Default()
	if _, ok := c.Lookup("describe"); !ok {
		t.Fatal("Lookup(describe) not found")
	}
	got, ok := c.Lookup("timeline")
	if !ok {
		t.Fatal("Lookup(timeline) not found")
	}
	if got.Name != "timeline" {
		t.Fatalf("Lookup(timeline).Name = %q, want timeline", got.Name)
	}
	if _, ok := c.Lookup("nope"); ok {
		t.Fatal("Lookup(nope) reported found, want not found")
	}
}

// TestNames lists every tool in catalog order for error hints.
func TestNames(t *testing.T) {
	names := Default().Names()
	if len(names) != len(Default().Tools) {
		t.Fatalf("Names() len = %d, want %d", len(names), len(Default().Tools))
	}
	if names[0] != "describe" {
		t.Fatalf("Names()[0] = %q, want describe", names[0])
	}
}

// TestJSONSchemaShape checks the per-command schema is a valid, self-describing
// JSON Schema document: it declares the draft dialect, is titled by the tool,
// and nests the tool's input and output schemas. This is what `schema <cmd>`
// emits, and it must round-trip through JSON unchanged.
func TestJSONSchemaShape(t *testing.T) {
	tool, _ := Default().Lookup("timeline")
	s := tool.JSONSchema()

	if s["$schema"] != jsonSchemaDialect {
		t.Errorf("$schema = %v, want %s", s["$schema"], jsonSchemaDialect)
	}
	if s["title"] != "timeline" {
		t.Errorf("title = %v, want timeline", s["title"])
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties is %T, want map", s["properties"])
	}
	if _, ok := props["input"]; !ok {
		t.Error("schema has no input property")
	}
	if _, ok := props["output"]; !ok {
		t.Error("schema has no output property")
	}

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("schema does not marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("schema does not round-trip: %v", err)
	}
}

// TestJSONSchemaFillsMissingSchemas guards that a tool with nil schemas still
// yields a valid object schema for each side, never a null.
func TestJSONSchemaFillsMissingSchemas(t *testing.T) {
	bare := Tool{Name: "bare", Summary: "no schemas"}
	props := bare.JSONSchema()["properties"].(map[string]any)
	for _, side := range []string{"input", "output"} {
		m, ok := props[side].(map[string]any)
		if !ok {
			t.Fatalf("%s is %T, want an object schema", side, props[side])
		}
		if m["type"] != "object" {
			t.Errorf("%s.type = %v, want object", side, m["type"])
		}
	}
}
