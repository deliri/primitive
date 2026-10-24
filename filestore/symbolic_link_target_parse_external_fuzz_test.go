package filestore_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzSymbolicLinkTargetParseSemanticClosure(f *testing.F) {
	seed, err := filestore.ParseSymbolicLinkTarget("../outside/./opaque:+:@")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	for _, raw := range []string{"", "\x00", "x\x00y", "/absolute/missing", ".", "..", "\xff", " ", strings.Repeat("x", 65537)} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		// This bounds only this development campaign and its independent oracle.
		// The production nominal has no length ceiling.
		raw = raw[:min(len(raw), 65537)]
		admissible := len(raw) != 0
		for i := range len(raw) {
			if raw[i] == 0 {
				admissible = false
				break
			}
		}
		got, err := filestore.ParseSymbolicLinkTarget(raw)
		if !admissible {
			if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.SymbolicLinkTarget{}) {
				t.Fatalf("refused parse = (%q,%v), want zero and contract refusal", got.String(), err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != raw {
			t.Fatalf("accepted parse = (%q,%v), want exact opaque %q", got.String(), err, raw)
		}
		again, err := filestore.ParseSymbolicLinkTarget(got.String())
		if err != nil || again != got {
			t.Fatalf("nominal round trip = (%q,%v), want %q", again.String(), err, raw)
		}
	})
}
