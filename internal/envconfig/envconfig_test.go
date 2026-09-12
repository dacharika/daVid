package envconfig

import "testing"

func TestLookupPrefersCanonicalValue(t *testing.T) {
	t.Setenv("DAVID_TEST_SETTING", "canonical")
	t.Setenv("CUTVEC_TEST_SETTING", "legacy")
	if got := Lookup("DAVID_TEST_SETTING", "CUTVEC_TEST_SETTING"); got != "canonical" {
		t.Fatalf("Lookup() = %q, want canonical value", got)
	}
}

func TestLookupFallsBackToLegacyValue(t *testing.T) {
	t.Setenv("DAVID_TEST_SETTING", "")
	t.Setenv("CUTVEC_TEST_SETTING", "legacy")
	if got := Lookup("DAVID_TEST_SETTING", "CUTVEC_TEST_SETTING"); got != "legacy" {
		t.Fatalf("Lookup() = %q, want legacy fallback", got)
	}
}

func TestLookupReturnsEmptyWhenUnset(t *testing.T) {
	t.Setenv("DAVID_TEST_SETTING", "")
	t.Setenv("CUTVEC_TEST_SETTING", "")
	if got := Lookup("DAVID_TEST_SETTING", "CUTVEC_TEST_SETTING"); got != "" {
		t.Fatalf("Lookup() = %q, want empty value", got)
	}
}
