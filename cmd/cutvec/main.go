// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/catalog"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		emitError(err)
		os.Exit(1)
	}
}

// run dispatches a subcommand. The scaffold wires `describe`; every other tool
// in the catalog is implemented by a later task.
func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "describe":
		return emitJSON(catalog.Default())
	case "mcp":
		return axerror.New("mcp.unimplemented", "the MCP server is not built yet").
			WithHint("run `cutvec describe` to see the tool catalog")
	case "-h", "--help", "help":
		return usage()
	default:
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("unknown command %q", args[0])).
			WithHint("run `cutvec describe` to list available tools")
	}
}

// usage prints the catalog's tool names so a caller (human or agent) can
// discover the surface without a separate help system.
func usage() error {
	fmt.Fprintln(os.Stderr, "cutvec — agent-native video-editing CLI + MCP server")
	fmt.Fprintln(os.Stderr, "\nusage: cutvec <command> [args]\n\ncommands:")
	fmt.Fprintln(os.Stderr, "  describe   emit the tool catalog and JSON Schemas")
	fmt.Fprintln(os.Stderr, "  mcp        serve the catalog over MCP (not yet implemented)")
	for _, t := range catalog.Default().Tools {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", t.Name, t.Summary)
	}
	return nil
}

// emitJSON writes v as indented JSON to stdout.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
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
