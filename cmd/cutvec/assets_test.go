package main

import (
	"strings"
	"testing"
)

// An unannotated library still has to be searchable, and the folders above a
// file carry real meaning in a sound library.
func TestDescribeAssetDerivesTextFromThePath(t *testing.T) {
	for _, tc := range []struct{ root, path, want string }{
		{"/lib", "/lib/sfx/impacts/deep-boom-01.wav", "sfx impacts deep boom"},
		{"/lib", "/lib/whoosh_transition.wav", "whoosh transition"},
		{"/lib", "/lib/Riser-Long.MP3", "riser long"},
	} {
		if got := describeAsset(tc.root, tc.path); got != tc.want {
			t.Errorf("describeAsset(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// Take numbers are not description; keeping them would dilute the embedding.
func TestDescribeAssetDropsBareNumbersButKeepsRealWords(t *testing.T) {
	got := describeAsset("/lib", "/lib/hit/2152.wav")
	if strings.Contains(got, "2152") {
		t.Errorf("description %q kept a bare take number", got)
	}
	if !strings.Contains(got, "hit") {
		t.Errorf("description %q lost the meaningful folder name", got)
	}
	// A file with nothing but digits still needs some text to embed.
	if describeAsset("/lib", "/lib/1485.mp3") == "" {
		t.Error("an all-numeric name produced no description at all")
	}
}

func TestParseOnePositionalAcceptsPlanAndRejectsExtras(t *testing.T) {
	pos, plan, err := parseOnePositional([]string{"/lib", "--plan"}, "assets index")
	if err != nil || pos != "/lib" || !plan {
		t.Fatalf("got (%q,%v,%v), want (/lib,true,nil)", pos, plan, err)
	}
	if _, _, err := parseOnePositional([]string{"a", "b"}, "assets index"); err == nil {
		t.Error("two positionals were accepted")
	}
	if _, _, err := parseOnePositional([]string{"--wat"}, "assets index"); err == nil {
		t.Error("an unknown flag was accepted")
	}
}
