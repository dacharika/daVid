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
				Name:    "index",
				Summary: "Embed an ingested source's transcript segments into a local LanceDB index via the embed worker (no network, cached, idempotent).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project":    map[string]any{"type": "string", "description": "the project directory (also accepted as a positional argument)"},
						"source":     map[string]any{"type": "string", "description": "path, id, or hash of the ingested source to index; optional when the project has exactly one source"},
						"worker":     map[string]any{"type": "string", "description": "path to the local embedding worker executable; defaults to the CUTVEC_INDEX_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments passed to the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"index":    map[string]any{"type": "string", "description": "handle: path to the cached index metadata JSON"},
						"table":    map[string]any{"type": "string", "description": "handle: path to the LanceDB .lance table of embedded segments"},
						"source":   map[string]any{"type": "string", "description": "content hash of the indexed source"},
						"segments": map[string]any{"type": "integer", "description": "number of transcript segments embedded"},
						"dim":      map[string]any{"type": "integer", "description": "dimensionality of each embedding vector"},
						"model":    map[string]any{"type": "string", "description": "local embedding model used, when the worker reports one"},
						"cached":   map[string]any{"type": "boolean", "description": "true on a content-hash cache hit (the index already existed, no worker ran)"},
						"planned":  map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
			{
				Name:    "search",
				Summary: "Semantic search over a source's index: return ranked timecoded {start,end,text,score} candidates for a query (deterministic).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project", "query"},
					"properties": map[string]any{
						"project":    map[string]any{"type": "string", "description": "the project directory to search (also accepted as the first positional argument)"},
						"query":      map[string]any{"type": "string", "description": "the natural-language query (also accepted as the second positional argument)"},
						"source":     map[string]any{"type": "string", "description": "path, id, or hash of the indexed source to search; optional when the project has exactly one source"},
						"limit":      map[string]any{"type": "integer", "description": "cap on the number of ranked candidates returned; 0 or omitted returns all"},
						"worker":     map[string]any{"type": "string", "description": "path to the local search worker executable; defaults to the CUTVEC_SEARCH_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments passed to the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"query":  map[string]any{"type": "string", "description": "the query that was searched for"},
						"source": map[string]any{"type": "string", "description": "content hash of the searched source"},
						"results": map[string]any{
							"type":        "array",
							"description": "ranked timecoded candidates, most-relevant first",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"start": map[string]any{"type": "number", "description": "candidate start offset in seconds"},
									"end":   map[string]any{"type": "number", "description": "candidate end offset in seconds"},
									"text":  map[string]any{"type": "string", "description": "the matched transcript span"},
									"score": map[string]any{"type": "number", "description": "relevance score; results are sorted by it, descending"},
								},
							},
						},
						"count":   map[string]any{"type": "integer", "description": "number of candidates returned"},
						"model":   map[string]any{"type": "string", "description": "local embedding model used, when the worker reports one"},
						"planned": map[string]any{"type": "boolean", "description": "true when --plan was set and no worker ran"},
					},
				},
			},
			{
				Name:    "speakers",
				Summary: "Detect the active speaker over time for an ingested source via the mediapipe worker, against a fixed two-person layout (cached, idempotent).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project":    map[string]any{"type": "string", "description": "the project directory (also accepted as a positional argument)"},
						"source":     map[string]any{"type": "string", "description": "path, id, or hash of the ingested source to analyse; optional when the project has exactly one source"},
						"worker":     map[string]any{"type": "string", "description": "path to the mediapipe worker executable; defaults to the CUTVEC_MEDIAPIPE_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments passed to the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"speakers": map[string]any{"type": "string", "description": "handle: path to the cached active-speaker turns JSON"},
						"source":   map[string]any{"type": "string", "description": "content hash of the analysed source"},
						"layout":   map[string]any{"type": "string", "description": "the fixed speaker layout the turns were computed against"},
						"turns":    map[string]any{"type": "integer", "description": "number of per-time speaker turns"},
						"cached":   map[string]any{"type": "boolean", "description": "true on a content-hash cache hit (the analysis already existed, no worker ran)"},
						"planned":  map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
			{
				Name:    "shorts",
				Summary: "Recipe: turn a query into vertical captioned shorts — searches the index for matching moments, locks the frame on whoever is speaking, burns word-timed captions, and renders each. Requires an indexed, transcribed source with a speaker track.",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project", "query"},
					"properties": map[string]any{
						"project":          map[string]any{"type": "string", "description": "the project directory (also accepted as the first positional argument)"},
						"query":            map[string]any{"type": "string", "description": "what the shorts should be about (also accepted as the second positional argument)"},
						"source":           map[string]any{"type": "string", "description": "path, id, or hash of the source; optional when the project has exactly one"},
						"count":            map[string]any{"type": "integer", "description": "how many shorts to produce; defaults to 3"},
						"padding":          map[string]any{"type": "number", "description": "seconds of headroom added either side of a matched moment; defaults to 0.4"},
						"quality":          map[string]any{"type": "string", "description": "render quality: 'proxy' for a fast look, 'final' for delivery (default)"},
						"hook":             map[string]any{"type": "string", "description": "optional caption card shown over the opening seconds"},
						"leftCentre":       map[string]any{"type": "number", "description": "normalised horizontal centre of the left seat; defaults to 0.28"},
						"rightCentre":      map[string]any{"type": "number", "description": "normalised horizontal centre of the right seat; defaults to 0.72"},
						"searchWorker":     map[string]any{"type": "string", "description": "path to the search worker; defaults to the CUTVEC_SEARCH_WORKER env var"},
						"searchWorkerArgs": map[string]any{"type": "array", "description": "fixed leading arguments for the search worker", "items": map[string]any{"type": "string"}},
						"renderWorker":     map[string]any{"type": "string", "description": "path to the melt render worker; defaults to the CUTVEC_MELT_WORKER env var"},
						"renderWorkerArgs": map[string]any{"type": "array", "description": "fixed leading arguments for the render worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"query":  map[string]any{"type": "string", "description": "the query the shorts were built from"},
						"source": map[string]any{"type": "string", "description": "content hash of the source"},
						"shorts": map[string]any{
							"type":        "array",
							"description": "one entry per short, best match first",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"rank":     map[string]any{"type": "integer", "description": "1-based rank of the matched moment"},
									"start":    map[string]any{"type": "number", "description": "start of the clip in the source, in seconds"},
									"end":      map[string]any{"type": "number", "description": "end of the clip in the source, in seconds"},
									"speaker":  map[string]any{"type": "string", "description": "the seat the frame is locked onto for this clip"},
									"score":    map[string]any{"type": "number", "description": "search relevance of the matched moment"},
									"text":     map[string]any{"type": "string", "description": "the matched transcript text"},
									"timeline": map[string]any{"type": "string", "description": "handle: the short's MLT XML, editable and re-renderable"},
									"captions": map[string]any{"type": "string", "description": "handle: the generated SRT caption file"},
									"output":   map[string]any{"type": "string", "description": "handle: the rendered vertical video; absent under --plan"},
								},
							},
						},
						"planned": map[string]any{"type": "boolean", "description": "true when --plan was set: timelines and captions were written but nothing rendered"},
					},
				},
			},
			{
				Name:    "filters",
				Summary: "Discover what MLT can do on this machine: search the installed filter registry (several hundred, including the whole avfilter set) and get a filter's parameters. Use it to find a service name for `timeline`'s effects instead of guessing.",
				InputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"query":      map[string]any{"type": "string", "description": "substring to search filter names for, e.g. 'denoise' or 'color' (also accepted as the first positional argument); omit to list everything"},
						"name":       map[string]any{"type": "string", "description": "exact filter name to describe, e.g. 'avfilter.curves'; returns its full parameter list"},
						"limit":      map[string]any{"type": "integer", "description": "cap on filters returned; defaults to 40. Parameters are only expanded for focused result sets"},
						"worker":     map[string]any{"type": "string", "description": "path to the melt worker; defaults to the CUTVEC_MELT_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments for the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"filters": map[string]any{
							"type":        "array",
							"description": "matching MLT filters",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":        map[string]any{"type": "string", "description": "the mlt_service name to use in a timeline effect"},
									"description": map[string]any{"type": "string", "description": "what the filter does"},
									"params": map[string]any{
										"type":        "array",
										"description": "the filter's parameters; avfilter wrappers prefix theirs with 'av.'",
										"items": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"name":        map[string]any{"type": "string"},
												"type":        map[string]any{"type": "string"},
												"description": map[string]any{"type": "string"},
											},
										},
									},
								},
							},
						},
						"count":   map[string]any{"type": "integer", "description": "how many filters matched the query"},
						"total":   map[string]any{"type": "integer", "description": "how many filters MLT has registered in total"},
						"presets": map[string]any{"type": "array", "description": "cutvec's named filter chains, usable directly as a timeline effect", "items": map[string]any{"type": "string"}},
					},
				},
			},
			{
				Name:    "reframe-edit",
				Summary: "Recipe: cut a single locked-off wide shot into an edit that looks multi-camera — punch in on whoever is speaking, fall back to the wide two-shot when nobody is, and render. Requires a source with a speaker track.",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project":          map[string]any{"type": "string", "description": "the project directory (also accepted as the first positional argument)"},
						"source":           map[string]any{"type": "string", "description": "path, id, or hash of the source; optional when the project has exactly one"},
						"quality":          map[string]any{"type": "string", "description": "render quality: 'proxy' for a fast look, 'final' for delivery (default)"},
						"keep":             map[string]any{"type": "number", "description": "fraction of frame width a single keeps; smaller punches in tighter. Defaults to 0.58"},
						"minShot":          map[string]any{"type": "number", "description": "shortest shot in seconds; shorter ones are absorbed so the edit does not ping-pong. Defaults to 1.8"},
						"leftCentre":       map[string]any{"type": "number", "description": "normalised horizontal centre of the left seat; defaults to 0.28"},
						"rightCentre":      map[string]any{"type": "number", "description": "normalised horizontal centre of the right seat; defaults to 0.72"},
						"renderWorker":     map[string]any{"type": "string", "description": "path to the melt render worker; defaults to the CUTVEC_MELT_WORKER env var"},
						"renderWorkerArgs": map[string]any{"type": "array", "description": "fixed leading arguments for the render worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"source":   map[string]any{"type": "string", "description": "content hash of the edited source"},
						"timeline": map[string]any{"type": "string", "description": "handle: the edit's MLT XML, editable and re-renderable"},
						"duration": map[string]any{"type": "number", "description": "length of the edit in seconds"},
						"shots": map[string]any{
							"type":        "array",
							"description": "the gapless shot list the edit cuts through",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"shot":  map[string]any{"type": "string", "description": "'left' or 'right' for a single, 'wide' for the two-shot"},
									"start": map[string]any{"type": "number", "description": "start of the shot in seconds"},
									"end":   map[string]any{"type": "number", "description": "end of the shot in seconds"},
								},
							},
						},
						"output":  map[string]any{"type": "string", "description": "handle: the rendered edit; absent under --plan"},
						"planned": map[string]any{"type": "boolean", "description": "true when --plan was set: the timeline was written but nothing rendered"},
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
						"effects": map[string]any{
							"type":        "array",
							"description": "MLT filters attached to the timeline, applied in order. Each effect is either a named preset or one MLT service — run `cutvec filters` to discover services and their parameters.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"preset":     map[string]any{"type": "string", "description": "a named filter chain: grade, denoise, denoise-strong, loudness, warm"},
									"service":    map[string]any{"type": "string", "description": "an MLT filter name, e.g. 'avfilter.curves' or 'lift_gamma_gain'"},
									"properties": map[string]any{"type": "object", "description": "the service's properties; avfilter wrappers prefix theirs with 'av.'"},
									"producer":   map[string]any{"type": "string", "description": "limit the effect to one producer id; omit to apply it to every clip"},
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
			{
				Name:    "render",
				Summary: "Render a project's MLT timeline to an output video via the melt worker at a proxy or final quality (cached, idempotent).",
				InputSchema: Schema{
					"type":     "object",
					"required": []any{"project"},
					"properties": map[string]any{
						"project":    map[string]any{"type": "string", "description": "the project directory whose timeline to render (also accepted as a positional argument)"},
						"quality":    map[string]any{"type": "string", "enum": []any{"proxy", "final"}, "description": "render tier; proxy is a fast draft, final is full quality. Overridden by --proxy/--final; defaults to proxy"},
						"worker":     map[string]any{"type": "string", "description": "path to the melt render worker executable; defaults to the CUTVEC_MELT_WORKER env var"},
						"workerArgs": map[string]any{"type": "array", "description": "fixed leading arguments passed to the worker", "items": map[string]any{"type": "string"}},
					},
				},
				OutputSchema: Schema{
					"type": "object",
					"properties": map[string]any{
						"output":   map[string]any{"type": "string", "description": "handle: path to the rendered video"},
						"timeline": map[string]any{"type": "string", "description": "handle: path to the MLT timeline that was rendered"},
						"source":   map[string]any{"type": "string", "description": "content hash of the rendered timeline"},
						"quality":  map[string]any{"type": "string", "description": "the quality tier the output was rendered at"},
						"cached":   map[string]any{"type": "boolean", "description": "true on a content-hash cache hit (the render already existed, no worker ran)"},
						"planned":  map[string]any{"type": "boolean", "description": "true when --plan was set and nothing was written"},
					},
				},
			},
		},
	}
}
