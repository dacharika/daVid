package workflows_test

import (
	"os"
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

func TestCIKeepsBothFullGoSuitesAndEveryWorkerGate(t *testing.T) {
	workflow := readWorkflow(t, "ci.yml")
	goJob := job(t, workflow, "go")
	for _, required := range []string{"name: Go ${{ matrix.os }}", "os: [ubuntu-latest, macos-latest]", "needs: staticcheck", "gofmt -l .", "go vet ./...", "go build ./...", "go test -race -covermode=atomic -coverprofile=coverage.out ./...", "./scripts/check_branding.sh", "go tool cover -func=coverage.out", "retention-days: 14"} {
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

func TestCIAnalysisCacheNeverSkipsChecks(t *testing.T) {
	analysis := job(t, readWorkflow(t, "ci.yml"), "staticcheck")
	if strings.Contains(analysis, "    needs:") {
		t.Fatal("cheap-first analysis must not depend on the jobs it admits")
	}
	for _, required := range []string{"name: Staticcheck", "STATICCHECK_VERSION: v0.7.0", "id: go", "go-version-file: 'go.mod'", "go test ./.github/workflows", "runner.os", "runner.arch", "steps.go.outputs.go-version", "env.STATICCHECK_VERSION", "if: steps.staticcheck-cache.outputs.cache-hit != 'true'", `go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}"`, `staticcheck 2026.1 (0.7.0)`, `"$RUNNER_TEMP/daVid-ci-tools/staticcheck" ./...`} {
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
