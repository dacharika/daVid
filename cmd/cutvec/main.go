// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
	"github.com/tahabsn/cutvec/internal/project"
)

// progressInterval is how often the long steps heartbeat on stderr. It defaults
// to progress's own cadence and can be overridden with CUTVEC_PROGRESS_INTERVAL
// (any Go duration, e.g. "5s"); an unparseable value falls back to the default.
func progressInterval() time.Duration {
	if v := os.Getenv("CUTVEC_PROGRESS_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 0 // progress.While treats non-positive as its own default cadence
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		emitError(err)
		os.Exit(1)
	}
}

// run dispatches a subcommand. Tools read their JSON request from stdin and
// write their JSON result to stdout; both streams are passed in so the dispatch
// is testable without touching the process's real file descriptors.
//
// stderr is an optional side channel for progress events on the long steps
// (transcribe, speakers, render). It is variadic so the many callers that only
// care about the JSON result on stdout stay three-argument; when omitted (or
// nil) progress is discarded and the observable contract — exactly one JSON
// result on stdout — is unchanged.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr ...io.Writer) error {
	var progressOut io.Writer = io.Discard
	if len(stderr) > 0 && stderr[0] != nil {
		progressOut = stderr[0]
	}
	// Help goes to the caller's stderr, not the process's: a caller that injects
	// a writer (the MCP server, every test) must be able to capture it. Resolved
	// before the no-args check so a bare `cutvec` is captured too.
	helpOut := progressOut
	if helpOut == io.Discard {
		helpOut = os.Stderr
	}
	if len(args) == 0 {
		return usage(helpOut)
	}
	switch args[0] {
	case "describe":
		return describeCmd(args[1:], stdout)
	case "schema":
		return schemaCmd(args[1:], stdout)
	case "init":
		return initCmd(args[1:], stdin, stdout)
	case "ingest":
		return ingestCmd(args[1:], stdin, stdout)
	case "status":
		return statusCmd(args[1:], stdin, stdout)
	case "transcribe":
		return transcribeCmd(args[1:], stdin, stdout, progressOut)
	case "index":
		return indexCmd(args[1:], stdin, stdout)
	case "search":
		return searchCmd(args[1:], stdin, stdout)
	case "speakers":
		return speakersCmd(args[1:], stdin, stdout, progressOut)
	case "shorts":
		return shortsCmd(args[1:], stdin, stdout)
	case "reframe-edit":
		return reframeCmd(args[1:], stdin, stdout)
	case "assets":
		return assetsCmd(args[1:], stdin, stdout)
	case "filters":
		return filtersCmd(args[1:], stdin, stdout)
	case "timeline":
		return timelineCmd(args[1:], stdin, stdout)
	case "render":
		return renderCmd(args[1:], stdin, stdout, progressOut)
	case "doctor":
		return doctorCmd(args[1:], stdin, stdout)
	case "mcp":
		return mcpCmd(args[1:], stdin, stdout)
	case "-h", "--help", "help":
		return usage(helpOut)
	default:
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("unknown command %q", args[0])).
			WithHint("run `cutvec describe` to list available tools")
	}
}

// describeCmd emits the tool catalog so an agent can discover every command and
// its typed IO. `describe` returns the full catalog object ({version,tools});
// `describe --json` returns just the tools array — each tool carrying its name,
// summary, input schema, and output schema — which is the shape an agent maps
// straight onto its tool list.
func describeCmd(args []string, stdout io.Writer) error {
	c := catalog.Default()
	for _, a := range args {
		switch a {
		case "--json":
			return emitJSON(stdout, c.Tools)
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --json (emit the tools array)")
		}
	}
	return emitJSON(stdout, c)
}

// schemaCmd emits the JSON Schema for a single command: `cutvec schema <cmd>`
// returns a draft 2020-12 schema whose `input` and `output` describe that tool's
// typed IO. An unknown command is a stable, branchable error listing the tools
// the caller can ask about.
func schemaCmd(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return axerror.New("input.empty", "no command named").
			WithHint("run `cutvec schema <cmd>`, e.g. `cutvec schema timeline`")
	}
	if len(args) > 1 {
		return axerror.New("input.invalid",
			fmt.Sprintf("schema takes one command, got %d args", len(args))).
			WithHint("name exactly one command, e.g. `cutvec schema timeline`")
	}
	c := catalog.Default()
	tool, ok := c.Lookup(args[0])
	if !ok {
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("no command named %q", args[0])).
			WithHint("known commands: " + strings.Join(c.Names(), ", "))
	}
	return emitJSON(stdout, tool.JSONSchema())
}

// parseProjectArgs reads the shared per-source flags (--plan dry-run, --json
// affirming the output form) and at most one positional project argument off
// args. cmd names the command in error messages (e.g. transcribe, index).
func parseProjectArgs(args []string, cmd string) (proj string, plan bool, err error) {
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case a == "--json":
			// JSON is the only output form; the flag affirms it.
		case strings.HasPrefix(a, "-"):
			return "", false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the flags are --plan (dry-run) and --json")
		default:
			if proj != "" {
				return "", false, axerror.New("input.invalid",
					cmd+" takes at most one project argument").
					WithHint(fmt.Sprintf("name one project, e.g. `cutvec %s ./proj`", cmd))
			}
			proj = a
		}
	}
	return proj, plan, nil
}

// resolveSource picks the ingested source a per-source step acts on. With no
// name it returns the project's sole source, erroring when the project has none
// (nothing to act on) or more than one (ambiguous). A name matches a source by
// its path, id, or hash. verb names the action in error messages.
func resolveSource(m project.Manifest, want, verb string) (project.Source, error) {
	if len(m.Sources) == 0 {
		return project.Source{}, axerror.New("input.empty",
			"project has no ingested sources to "+verb).
			WithHint("run `cutvec ingest` to add a source first")
	}
	if want == "" {
		if len(m.Sources) == 1 {
			return m.Sources[0], nil
		}
		return project.Source{}, axerror.New("input.invalid",
			fmt.Sprintf("project has %d sources; name one with \"source\"", len(m.Sources))).
			WithHint("set \"source\" to the path, id, or hash of the source to " + verb)
	}
	for _, s := range m.Sources {
		if s.Path == want || s.ID == want || s.Hash == want {
			return s, nil
		}
	}
	return project.Source{}, axerror.New("source.missing",
		"no ingested source matches "+strconv.Quote(want)).
		WithHint("ingest the source first, or use a path/id from `cutvec status`")
}

// Render quality tiers: a fast draft ("proxy") and a full-quality final render
// ("final"). They key separate cache entries so both can coexist per timeline.
const (
	qualityProxy = "proxy"
	qualityFinal = "final"
)

// HasModule reports whether the interpreter can import name. It uses
// importlib.util.find_spec, which locates the module without importing it, so a
// heavy package (mediapipe) is not actually loaded just to answer yes/no.
func (o osProbe) HasModule(name string) bool {
	if o.python == "" {
		return false
	}
	cmd := exec.Command(o.python, "-c",
		"import importlib.util,sys; sys.exit(0 if importlib.util.find_spec(sys.argv[1]) else 1)", name)
	return cmd.Run() == nil
}

// parsePlan reads the shared --plan dry-run flag off args, rejecting any other
// flag with a stable code.
func parsePlan(args []string) (bool, error) {
	plan := false
	for _, a := range args {
		switch a {
		case "--plan":
			plan = true
		default:
			return false, axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --plan (dry-run)")
		}
	}
	return plan, nil
}

// decodeRequest reads a single JSON object from stdin into v, mapping an empty
// stream to input.empty and malformed JSON to input.invalid.
func decodeRequest(stdin io.Reader, v any) error {
	if err := json.NewDecoder(stdin).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return axerror.New("input.empty", "no JSON request on stdin").
				WithHint("pipe a JSON request object to stdin")
		}
		return axerror.New("input.invalid", "request is not valid JSON: "+err.Error()).
			WithHint("stdin must be a single JSON object")
	}
	return nil
}

// decodeOptionalRequest is decodeRequest for reads with no required input: an
// empty stdin leaves v at its defaults rather than erroring.
func decodeOptionalRequest(stdin io.Reader, v any) error {
	if err := json.NewDecoder(stdin).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return axerror.New("input.invalid", "request is not valid JSON: "+err.Error()).
			WithHint("stdin must be a single JSON object")
	}
	return nil
}

// mapSourceErr turns a filesystem error from reading a source into a stable
// axerror: a missing file is a caller mistake (source.missing), any other read
// error may be transient (source.read, retryable). ErrNotInitialized is mapped
// for defence in depth, though ingestCmd checks initialisation up front.
func mapSourceErr(err error) error {
	switch {
	case errors.Is(err, project.ErrNotInitialized):
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	case errors.Is(err, os.ErrNotExist):
		return axerror.New("source.missing", "source file not found: "+err.Error()).
			WithHint("check the source path exists and is readable")
	default:
		return axerror.New("source.read", "cannot read source file: "+err.Error()).AsRetryable()
	}
}

// usage prints the catalog's tool names so a caller (human or agent) can
// discover the surface without a separate help system.
func usage(w io.Writer) error {
	fmt.Fprintln(w, "cutvec — agent-native video-editing CLI + MCP server")
	fmt.Fprintln(w, "\nusage: cutvec <command> [args]\n\ncommands:")
	// The meta commands are not catalog tools, so they are listed by hand; every
	// real tool comes from the catalog below, which is the single source of truth.
	// `describe` is deliberately NOT repeated here — it is a catalog tool and the
	// loop prints it.
	// Padded to the same 12-char column the catalog loop below uses.
	fmt.Fprintf(w, "  %-12s %s\n", "schema", "emit the JSON Schema for one command: schema <cmd>")
	fmt.Fprintf(w, "  %-12s %s\n", "mcp serve", "serve the tool catalog over MCP (JSON-RPC on stdio)")
	// %-12s, not %-10s: the longest tool name is `reframe-edit` (12), and a
	// narrower field pushes its summary out of the column.
	for _, t := range catalog.Default().Tools {
		fmt.Fprintf(w, "  %-12s %s\n", t.Name, t.Summary)
	}
	// Deliberately hedged: `filters`, `schema` and `assets` take a positional
	// argument rather than a stdin body, and the read-only tools (describe,
	// doctor, filters) reject --plan outright. Claiming either applies
	// universally would send an agent into an avoidable cli.unknown_flag.
	fmt.Fprintln(w, "\ntools take a JSON request on stdin and emit one JSON document on stdout;")
	fmt.Fprintln(w, "a tool that acts also takes --plan to report what it would do, without doing it.")
	fmt.Fprintln(w, "run `cutvec schema <cmd>` for one tool's exact request shape.")
	return nil
}

// emitJSON writes v as indented JSON to w.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emitError writes err as the structured axerror JSON shape to stderr, wrapping
// any non-axerror error in a generic code so every failure is machine-legible.
func emitError(err error) {
	ae, ok := err.(*axerror.Error)
	if !ok {
		ae = axerror.New("internal", err.Error())
	}
	b, _ := json.Marshal(ae)
	fmt.Fprintln(os.Stderr, string(b))
}

// --- shared worker plumbing --------------------------------------------------
//
// Five steps each reach a different Python worker, and each used to carry its
// own ten-line copy of "take the request's path, else the env var, else refuse".
// The copies differed only in which env var they read and how they phrased the
// refusal, so the shape below keeps the wording per-step and the logic in one
// place — a fix to the resolution rule now lands once instead of five times.

var (
	whisperWorkerSpec   = workerSpec{"CUTVEC_WHISPER_WORKER", "whisper", "whisper worker executable"}
	indexWorkerSpec     = workerSpec{"CUTVEC_INDEX_WORKER", "index (embedding)", "local embedding worker executable"}
	searchWorkerSpec    = workerSpec{"CUTVEC_SEARCH_WORKER", "search", "local search worker executable"}
	mediapipeWorkerSpec = workerSpec{"CUTVEC_MEDIAPIPE_WORKER", "mediapipe", "mediapipe worker executable"}
	meltWorkerSpec      = workerSpec{"CUTVEC_MELT_WORKER", "melt", "melt render worker executable"}
)
