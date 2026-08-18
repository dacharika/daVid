// Command daVid is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `daVid mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/mlnomadpy/daVid/internal/axerror"
	"github.com/mlnomadpy/daVid/internal/envconfig"
	"github.com/mlnomadpy/daVid/internal/worker"
)

// workerSpec is how one step names the executable it needs and explains its
// absence. env is the canonical DAVID_* var; legacyEnv is the deprecated
// CUTVEC_* var accepted during the deprecation window. `what` completes
// "no %s worker configured"; `target` completes "…env var to the %s".
type workerSpec struct {
	env       string
	legacyEnv string
	what      string
	target    string
}

// resolveNamedWorker prefers the request's explicit worker path, falls back to
// the step's env var (DAVID_* canonical, CUTVEC_* legacy), and refuses with a
// branchable worker.missing when neither names one.
func resolveNamedWorker(reqPath string, reqArgs []string, spec workerSpec) (worker.Worker, error) {
	path := reqPath
	if path == "" {
		path = envconfig.Lookup(spec.env, spec.legacyEnv)
	}
	if path == "" {
		return worker.Worker{}, axerror.New("worker.missing",
			fmt.Sprintf("no %s worker configured", spec.what)).
			WithHint(fmt.Sprintf(`set "worker" in the request or the %s env var to the %s`,
				spec.env, spec.target))
	}
	return worker.Worker{Path: path, Args: reqArgs}, nil
}

// resultAs lifts a worker's untyped result map into the typed value the core
// works with, by round-tripping it through JSON. Five steps each had their own
// identical copy of this; only the type and the shape hint ever differed.
//
// A result that will not decode is a protocol violation rather than a bad input:
// the worker answered, but not with the shape the contract promises, so the hint
// names that shape instead of asking the caller to change anything.
func resultAs[T any](result map[string]any, what, shapeHint string) (T, error) {
	var zero T
	b, err := json.Marshal(result)
	if err != nil {
		return zero, axerror.New("worker.protocol",
			"cannot re-encode worker result: "+err.Error())
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return zero, axerror.New("worker.protocol",
			fmt.Sprintf("worker result is not %s: %s", what, err.Error())).
			WithHint(shapeHint)
	}
	return v, nil
}
