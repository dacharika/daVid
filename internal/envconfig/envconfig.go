// Package envconfig centralises env-var lookup for the DAVID_* / CUTVEC_*
// deprecation window. Call Lookup instead of os.Getenv so that callers
// automatically get the fallback and the deprecation warning without each
// site reimplementing the logic.
package envconfig

import (
	"fmt"
	"os"
)

// Lookup returns the value for the canonical davidKey env var. If davidKey is
// unset but the legacy cutvecKey is set, its value is returned and a structured
// deprecation warning is written to stderr (not stdout, so JSON output is not
// polluted). When both are set, davidKey wins and a warning is still emitted so
// operators know they have a stale variable. Returns "" when neither is set.
func Lookup(davidKey, cutvecKey string) string {
	david := os.Getenv(davidKey)
	cutvec := os.Getenv(cutvecKey)

	if david != "" {
		if cutvec != "" {
			fmt.Fprintf(os.Stderr,
				`{"level":"warn","msg":"both %s and %s are set; %s wins","deprecated":%q}`+"\n",
				davidKey, cutvecKey, davidKey, cutvecKey)
		}
		return david
	}
	if cutvec != "" {
		fmt.Fprintf(os.Stderr,
			`{"level":"warn","msg":"%s is deprecated, rename to %s","deprecated":%q,"canonical":%q}`+"\n",
			cutvecKey, davidKey, cutvecKey, davidKey)
		return cutvec
	}
	return ""
}
