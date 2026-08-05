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

// jsonSchemaDialect is the JSON Schema draft every emitted schema declares, so
// an agent knows exactly which dialect to validate against.
const jsonSchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// Catalog is the ordered set of tools cutvec exposes.
type Catalog struct {
	Version string `json:"version"`
	Tools   []Tool `json:"tools"`
}

// Lookup returns the tool with the given name and whether it was found. This is
// how `cutvec schema <cmd>` resolves a command to its typed IO.
func (c Catalog) Lookup(name string) (Tool, bool) {
	for _, t := range c.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Names returns the tool names in catalog order, for error hints that list the
// commands an agent can ask about.
func (c Catalog) Names() []string {
	names := make([]string, len(c.Tools))
	for i, t := range c.Tools {
		names[i] = t.Name
	}
	return names
}

// JSONSchema returns a self-contained JSON Schema document for the tool: a
// draft 2020-12 schema titled by the tool whose `input` and `output` properties
// hold the schemas the caller must satisfy and can expect back. This is the
// document `cutvec schema <cmd>` emits.
func (t Tool) JSONSchema() Schema {
	input := t.InputSchema
	if input == nil {
		input = objectSchema()
	}
	output := t.OutputSchema
	if output == nil {
		output = objectSchema()
	}
	return Schema{
		"$schema":     jsonSchemaDialect,
		"title":       t.Name,
		"description": t.Summary,
		"type":        "object",
		"properties": map[string]any{
			"input":  map[string]any(input),
			"output": map[string]any(output),
		},
		"required": []any{"input", "output"},
	}
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
			{
				Name:    "init",
				Summary: "Create a project directory with a project.json manifest (idempotent).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project": map[string]any{"type": "string", "description": "directory to initialise as a project"},
						"name":    map[string]any{"type": "string", "description": "display name; defaults to the directory base name"},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"project":  map[string]any{"type": "string", "description": "the project directory"},
						"manifest": map[string]any{"type": "string", "description": "handle: path to project.json"},
						"name":     map[string]any{"type": "string", "description": "the project's display name"},
						"created":  map[string]any{"type": "boolean", "description": "true when this call created the manifest; false if it already existed"},
						"planned":  map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
			{
				Name:    "ingest",
				Summary: "Record a source media file in the project, keyed by its content hash (idempotent, cached).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project", "source"},
					"properties": map[string]any{
						"project": map[string]any{"type": "string", "description": "the project directory"},
						"source":  map[string]any{"type": "string", "description": "path to the media file to ingest"},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"source": map[string]any{
							"type":        "object",
							"description": "the recorded source",
							"properties": map[string]any{
								"id":   map[string]any{"type": "string", "description": "short content-hash handle"},
								"path": map[string]any{"type": "string", "description": "resource path ingested from"},
								"hash": map[string]any{"type": "string", "description": "full sha256 of the file contents"},
							},
						},
						"cached":  map[string]any{"type": "boolean", "description": "true on a content-hash cache hit (identical bytes seen before)"},
						"planned": map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
			{
				Name:    "status",
				Summary: "Return the project state graph: sources ingested and artifacts marked done/pending.",
				InputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"project": map[string]any{"type": "string", "description": "the project directory; defaults to the current directory"},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"project":   map[string]any{"type": "string", "description": "the project directory"},
						"name":      map[string]any{"type": "string", "description": "the project's display name"},
						"sources":   map[string]any{"type": "array", "description": "sources ingested so far", "items": map[string]any{"type": "object"}},
						"artifacts": map[string]any{"type": "array", "description": "derived outputs, each with a done/pending status", "items": map[string]any{"type": "object"}},
					},
				},
			},
			{
				Name:    "transcribe",
				Summary: "Transcribe an ingested source to a word-level transcript via the whisper worker (cached, idempotent).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project":    map[string]any{"type": "string", "description": "the project directory (also accepted as a positional argument)"},
						"source":     map[string]any{"type": "string", "description": "path, id, or hash of the ingested source to transcribe; optional when the project has exactly one source"},
						"worker":     map[string]any{"type": "string", "description": "path to the whisper worker executable; defaults to the CUTVEC_WHISPER_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments passed to the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"transcript": map[string]any{"type": "string", "description": "handle: path to the cached word-level transcript JSON"},
						"source":     map[string]any{"type": "string", "description": "content hash of the transcribed source"},
						"words":      map[string]any{"type": "integer", "description": "number of timecoded words in the transcript"},
						"language":   map[string]any{"type": "string", "description": "detected language, when the worker reports one"},
						"cached":     map[string]any{"type": "boolean", "description": "true on a content-hash cache hit (the transcript already existed, no worker ran)"},
						"planned":    map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
			{
				Name:    "timeline",
				Summary: "Build an MLT XML timeline from clips and write it into the project directory.",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project", "clips"},
					"properties": map[string]any{
						"project": map[string]any{
							"type":        "string",
							"description": "project directory the timeline is written into",
						},
						"clips": map[string]any{
							"type":        "array",
							"description": "ordered media sources placed on the timeline",
							"items": map[string]any{
								"type":     "object",
								"required": []any{"resource"},
								"properties": map[string]any{
									"id":       map[string]any{"type": "string", "description": "producer id; defaults to producerN"},
									"resource": map[string]any{"type": "string", "description": "path to the media file"},
								},
							},
						},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"timeline":  map[string]any{"type": "string", "description": "handle: path to the written MLT document"},
						"producers": map[string]any{"type": "integer", "description": "number of producers placed"},
						"planned":   map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
		},
	}
}
