// Package catalog is cutvec's single self-describing tool catalog. One catalog
// backs BOTH the JSON CLI and the MCP server so the two surfaces cannot drift:
// each Tool carries its name, a one-line summary, and JSON Schemas for its
// input and output. `cutvec describe` emits this catalog verbatim.
package catalog

// Tool is one agent-callable operation, described well enough to generate both
// a CLI subcommand and an MCP tool definition without any hand-written glue.
type Tool struct {
	Name         string `json:"name"`
	Summary      string `json:"summary"`
	InputSchema  Schema `json:"inputSchema"`
	OutputSchema Schema `json:"outputSchema"`
}

// Schema is a JSON Schema document, held opaquely as decoded JSON so the
// catalog can round-trip any schema shape without a fixed Go struct.
type Schema map[string]any

// Catalog is the ordered set of tools cutvec exposes.
type Catalog struct {
	Version string `json:"version"`
	Tools   []Tool `json:"tools"`
}

// objectSchema is the JSON Schema for an object with no required properties.
func objectSchema() Schema {
	return Schema{"type": "object", "properties": map[string]any{}}
}

// Default returns the catalog as it stands today. Later tasks register their
// tools here; the scaffold seeds only `describe`, the tool that emits the
// catalog itself, so both surfaces have at least one working operation.
func Default() Catalog {
	return Catalog{
		Version: "0",
		Tools: []Tool{
			{
				Name:         "describe",
				Summary:      "Emit the cutvec tool catalog and JSON Schemas.",
				InputSchema:  objectSchema(),
				OutputSchema: objectSchema(),
			},
		},
	}
}
