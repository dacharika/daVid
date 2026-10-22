package workflows_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func job(t *testing.T, workflow, name string) string {
	t.Helper()
	start := strings.Index(workflow, "\n  "+name+":\n")
	if start < 0 {
		t.Fatalf("missing job %s", name)
	}
	block := workflow[start+1:]
	if next := regexp.MustCompile(`(?m)^  [a-z][a-z-]*:\s*$`).FindStringIndex(block[len("  "+name+":\n"):]); next != nil {
		block = block[:len("  "+name+":\n")+next[0]]
	}
	return block
}

func TestCIKeepsSelectedFullGoSuitesAndEveryWorkerGate(t *testing.T) {
	workflow := readWorkflow(t, "ci.yml")
	goJob := job(t, workflow, "go")
	for _, required := range []string{"name: Go ${{ matrix.os }}", "needs: staticcheck", "gofmt -l .", "go vet ./...", "go build ./...", "go test -race -covermode=atomic -coverprofile=coverage.out ./...", "./scripts/check_branding.sh", "go tool cover -func=coverage.out", "retention-days: 14"} {
		if !strings.Contains(goJob, required) {
			t.Fatalf("full per-OS Go gate lost %q", required)
		}
	}
	if !strings.Contains(workflow, `COVERAGE_FLOOR: "62"`) || !strings.Contains(goJob, "fail-fast: ${{ github.event_name == 'pull_request' }}") {
		t.Fatal("coverage floor or manual full-matrix behavior changed")
	}
	workers := job(t, workflow, "workers")
	for _, required := range []string{"name: Python workers", "needs: staticcheck", "ruff check workers/", "ruff format --check workers/", "python -m compileall -q workers/", "python -m unittest discover -s workers -p 'test_*.py'", "git ls-files -s workers/", "exit $status"} {
		if !strings.Contains(workers, required) {
			t.Fatalf("worker gate lost %q", required)
		}
	}
	contract := job(t, workflow, "contract")
	for _, required := range []string{"name: Worker protocol contract", "needs: staticcheck", "go test -run TestWorkerContract -v ./cmd/daVid", "python-version: '3.12'"} {
		if !strings.Contains(contract, required) {
			t.Fatalf("worker protocol gate lost %q", required)
		}
	}
}

// Validation is opt-in; release/tag publishing has its separate workflow.
func TestValidationWorkflowsRunOnlyWhenRequested(t *testing.T) {
	for _, name := range []string{"ci.yml", "security.yml"} {
		t.Run(name, func(t *testing.T) {
			workflow := readWorkflow(t, name)
			start := strings.Index(workflow, "\non:\n")
			if start < 0 {
				t.Fatal("validation workflow must declare explicit manual events")
			}
			block := workflow[start+len("\non:\n"):]
			if end := regexp.MustCompile(`(?m)^[a-z][a-z-]*:`).FindStringIndex(block); end != nil {
				block = block[:end[0]]
			}
			matches := regexp.MustCompile(`(?m)^  ([a-z_]+):`).FindAllStringSubmatch(block, -1)
			var events []string
			for _, match := range matches {
				events = append(events, match[1])
			}
			if !reflect.DeepEqual(events, []string{"workflow_dispatch"}) {
				t.Fatalf("validation events = %v, want only workflow_dispatch", events)
			}
		})
	}
}

// Evaluate the matrix's actual manual branch; no validation runs on push/PR.
func TestCIManualAuditsKeepBothFullGoSuites(t *testing.T) {
	goJob := job(t, readWorkflow(t, "ci.yml"), "go")
	match := regexp.MustCompile(`(?m)^        os: \$\{\{ fromJSON\(github\.event_name == '([a-z_]+)' && '([^']+)' \|\| '([^']+)'\) \}\}$`).FindStringSubmatch(goJob)
	if len(match) != 4 || match[1] != "workflow_dispatch" {
		t.Fatal("Go matrix must select the full manual runner list")
	}
	var runners []string
	if err := json.Unmarshal([]byte(match[2]), &runners); err != nil {
		t.Fatalf("invalid manual matrix JSON: %v", err)
	}
	if !reflect.DeepEqual(runners, []string{"ubuntu-latest", "macos-latest"}) {
		t.Fatalf("manual runners = %v, want Ubuntu and macOS", runners)
	}
}

func TestCIAnalysisCacheNeverSkipsChecks(t *testing.T) {
	analysis := job(t, readWorkflow(t, "ci.yml"), "staticcheck")
	if strings.Contains(analysis, "    needs:") {
		t.Fatal("cheap-first analysis must not depend on the jobs it admits")
	}
	for _, required := range []string{"name: Staticcheck", "STATICCHECK_VERSION: v0.7.0", "id: go", "go-version-file: 'go.mod'", "go test ./.github/workflows", "runner.os", "runner.arch", "steps.go.outputs.go-version", "env.STATICCHECK_VERSION", "if: steps.staticcheck-cache.outputs.cache-hit != 'true'", `go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}"`, `staticcheck 2026.1 (v0.7.0)`, `"$RUNNER_TEMP/daVid-ci-tools/staticcheck" ./...`} {
		if !strings.Contains(analysis, required) {
			t.Fatalf("pinned cache/check contract lost %q", required)
		}
	}
	if strings.Index(analysis, "gofmt -l .") > strings.Index(analysis, "go install") || strings.Contains(analysis, "@latest") || strings.Contains(analysis, "restore-keys:") {
		t.Fatal("analysis must fail formatting first and restore only the exact tool identity")
	}
	// Only installation is cache-conditional. The version check and full analysis
	// remain ordinary steps, including on cache hits.
	for _, label := range []string{"- name: Verify pinned staticcheck", `- run: '"$RUNNER_TEMP/daVid-ci-tools/staticcheck" ./...'`} {
		start := strings.Index(analysis, label)
		if start < 0 {
			t.Fatal("missing unconditional analysis step")
		}
		block := analysis[start:]
		if end := strings.Index(block[1:], "\n      - "); end >= 0 {
			block = block[:end+1]
		}
		if strings.Contains(block, "if:") {
			t.Fatal("cache hit may not skip verification or staticcheck")
		}
	}
}

func TestCIStrictAggregateAndBoundedManualIsolation(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	security := readWorkflow(t, "security.yml")
	aggregate := job(t, ci, "build")
	for _, required := range []string{"name: Go build and test", "needs: [go, staticcheck, workers, contract]", "if: always()", `if [ "$r" != "success" ]; then`, "exit 1"} {
		if !strings.Contains(aggregate, required) {
			t.Fatalf("required aggregate no longer fails closed: missing %q", required)
		}
	}
	for _, name := range []string{"go", "staticcheck", "workers", "contract"} {
		if !strings.Contains(aggregate, `"${{ needs.`+name+`.result }}"`) {
			t.Fatalf("required aggregate omitted %s", name)
		}
	}
	for _, spec := range []struct {
		text, prefix string
		jobs         []string
	}{{ci, "ci-", []string{"go", "staticcheck", "workers", "contract", "build"}}, {security, "history-security-", []string{"secrets"}}} {
		if !strings.Contains(spec.text, "group: "+spec.prefix+"${{ github.event_name == 'workflow_dispatch' && github.run_id || github.event.pull_request.number || github.ref }}") || !strings.Contains(spec.text, "cancel-in-progress: ${{ github.event_name == 'pull_request' || github.event_name == 'push' }}") || !strings.Contains(spec.text, "  workflow_dispatch:") {
			t.Fatal("manual audit can collide with ordinary workflow cancellation")
		}
		for _, name := range spec.jobs {
			block := job(t, spec.text, name)
			if !regexp.MustCompile(`(?m)^    timeout-minutes: [1-9][0-9]*$`).MatchString(block) || strings.Contains(block, "continue-on-error:") {
				t.Fatalf("job %s lost finite bounds or can conceal failure", name)
			}
		}
	}
	for _, required := range []string{"name: Full-history secret scan", "fetch-depth: 0", "ghcr.io/gitleaks/gitleaks:v8.28.0@sha256:cdbb7c955abce02001a9f6c9f602fb195b7fadc1e812065883f695d1eeaba854", "detect --source /repo --log-opts=--all --redact --no-banner"} {
		if !strings.Contains(security, required) {
			t.Fatalf("full-history security contract weakened: missing %q", required)
		}
	}
}

// Execute the workflow's actual shell assertion, including pipefail. These
// controlled tool outputs complement the measured pinned-binary receipt; a
// literal-only schema test would have missed Staticcheck's module-version v.
func TestCIPinnedStaticcheckVersionCommandRejectsWrongRuntime(t *testing.T) {
	analysis := job(t, readWorkflow(t, "ci.yml"), "staticcheck")
	start := strings.Index(analysis, "- name: Verify pinned staticcheck")
	if start < 0 {
		t.Fatal("missing runtime version assertion")
	}
	block := analysis[start:]
	if end := strings.Index(block[1:], "\n      - "); end >= 0 {
		block = block[:end+1]
	}
	match := regexp.MustCompile(`(?m)^        run: '(.*)'$`).FindStringSubmatch(block)
	if len(match) != 2 || !strings.Contains(block, "shell: bash") {
		t.Fatal("version assertion must be an explicit fail-closed bash step")
	}
	for _, test := range []struct {
		name, output string
		exit         int
		wantSuccess  bool
	}{
		{"pinned", "staticcheck 2026.1 (v0.7.0)", 0, true},
		{"missing-module-prefix", "staticcheck 2026.1 (0.7.0)", 0, false},
		{"different-version", "staticcheck 2025.1.1 (v0.6.1)", 0, false},
		{"extra-output", "unexpected staticcheck 2026.1 (v0.7.0)", 0, false},
		{"failed-command", "staticcheck 2026.1 (v0.7.0)", 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "daVid-ci-tools")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			// All fixture strings are finite literals without shell quoting bytes.
			exit := "0"
			if test.exit != 0 {
				exit = "1"
			}
			tool := "#!/bin/sh\nprintf '%s\\n' '" + test.output + "'\nexit " + exit + "\n"
			if err := os.WriteFile(filepath.Join(dir, "staticcheck"), []byte(tool), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", match[1])
			cmd.Env = append(os.Environ(), "RUNNER_TEMP="+root)
			output, err := cmd.CombinedOutput()
			if (err == nil) != test.wantSuccess {
				t.Fatalf("workflow runtime version assertion %s success=%v, want %v: %s", test.name, err == nil, test.wantSuccess, output)
			}
		})
	}
}
