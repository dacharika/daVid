// Command cutvec is the agent-native video-editing CLI and MCP server. Both
// surfaces are driven by one self-describing catalog (see internal/catalog): the
// CLI dispatches subcommands from it, and `cutvec mcp` will serve the same tools
// over MCP. IO is JSON on stdout; failures are the structured axerror shape.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/doctor"
)

// doctorCmd preflights the whole runtime environment in one call: it probes
// every binary, worker, python package, and model file the pipeline needs and
// writes the ok/missing report (with an install hint per gap) to stdout. It runs
// no worker and touches no project — pure inspection — so it can gate a pipeline
// before real work starts. The report is always emitted; when a required
// dependency is missing the command additionally returns a structured error so
// the process exits non-zero, exactly the signal a caller wants before spending
// minutes on a run that would fail halfway.
func doctorCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	for _, a := range args {
		switch a {
		case "--json":
			// The report is always JSON; the flag is accepted for parity with
			// the other tools' explicit --json.
		default:
			return axerror.New("cli.unknown_flag", fmt.Sprintf("unknown flag %q", a)).
				WithHint("doctor takes no arguments; the only flag is --json")
		}
	}

	rep := doctor.Run(osProbe{python: pythonInterpreter()})
	if err := emitJSON(stdout, rep); err != nil {
		return err
	}
	if !rep.Ok {
		return axerror.New("doctor.preflight_failed",
			fmt.Sprintf("environment preflight failed: %d required dependency(ies) missing (%s)",
				rep.RequiredMissing, strings.Join(rep.MissingNames, ", "))).
			WithHint("install or configure each missing dependency; see the per-item hints in the report on stdout")
	}
	return nil
}

// osProbe is the production doctor.Probe: it inspects the real host via PATH,
// the filesystem, the environment, and a python subprocess for module checks.
type osProbe struct {
	// python is the interpreter used to test module importability; empty when
	// no python is on PATH, in which case every package check reports missing.
	python string
}

func (osProbe) LookPath(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func (osProbe) FileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (osProbe) Getenv(key string) string { return os.Getenv(key) }

func (osProbe) Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// pythonInterpreter resolves the interpreter the module checks run under,
// preferring python3 and falling back to python; "" when neither is on PATH.
func pythonInterpreter() string {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
