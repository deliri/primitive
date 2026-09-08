package release

import (
	"bytes"
	json "encoding/json/v2"
	"strconv"
	"testing"
)

// Every public decoder owns both its input bytes and replacement lifetime.
// Cardinalities cover empty, singleton, and both admitted sides of the ceiling;
// the refusal immediately above it is covered by the admission table.
func TestDependencyDecoderOwnsInputAndReplacementStorage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count int
	}{
		{name: "empty closure cannot inherit replaced modules"},
		{name: "singleton owns its only module", count: 1},
		{name: "one below ceiling owns every slot", count: BuildDependencyMaximumCount - 1},
		{name: "exact ceiling owns its last slot", count: BuildDependencyMaximumCount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture, err := newBuildDependencies(mustModulePath(t, testMainModule), CurrentGoToolchain(), numberedModules(t, tc.count))
			if err != nil {
				t.Fatalf("newBuildDependencies() error = %v, want nil", err)
			}
			canonical, err := fixture.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON(fixture) error = %v, want nil", err)
			}
			input := bytes.Clone(canonical)
			var got BuildDependencies
			if err := got.UnmarshalJSON(input); err != nil {
				t.Fatalf("UnmarshalJSON(input) error = %v, want nil", err)
			}
			clear(input)
			owned, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(owned, canonical) {
				t.Fatalf("decoded custody after input overwrite = (%q, %v), want %q", owned, err, canonical)
			}
			retained := got
			var replacement buildDependenciesWire
			if err := json.Unmarshal(canonical, &replacement); err != nil {
				t.Fatalf("Go Unmarshal(fixture) error = %v, want nil", err)
			}
			replacement.MainModule = testMainModule + "/replacement"
			for index := range replacement.Modules {
				replacement.Modules[index].Version = "v2.0." + strconv.Itoa(index)
			}
			replacementBytes, err := json.Marshal(replacement)
			if err != nil {
				t.Fatalf("Go Marshal(replacement) error = %v, want nil", err)
			}
			if err := got.UnmarshalJSON(replacementBytes); err != nil {
				t.Fatalf("UnmarshalJSON(replacement) error = %v, want nil", err)
			}
			preserved, err := retained.MarshalJSON()
			if err != nil || !bytes.Equal(preserved, canonical) {
				t.Fatalf("retained receiver after replacement = (%q, %v), want %q", preserved, err, canonical)
			}
			updated, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(updated, replacementBytes) {
				t.Fatalf("replacement receiver = (%q, %v), want %q", updated, err, replacementBytes)
			}
		})
	}
}
