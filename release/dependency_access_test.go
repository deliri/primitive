package release

import (
	"math"
	"testing"
)

// Contract ratchet: public access must preserve exact admitted facts, reject
// every index outside the collection, and never expose writable storage.
func TestDependencyAccessPreservesOwnedFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count int
		unset bool
	}{
		{name: "unset cannot impersonate an admitted collection", unset: true},
		{name: "admitted empty cannot invent a module"},
		{name: "singleton cannot alias its returned value", count: 1},
		{name: "ceiling cannot truncate or alias its final module", count: BuildDependencyMaximumCount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := numberedModules(t, tc.count)
			var got BuildDependencies
			if !tc.unset {
				if err := got.UnmarshalJSON(mustDependencyAccessDocument(t, tc.count)); err != nil {
					t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
				}
			}
			if count := got.Count(); count != tc.count {
				t.Fatalf("Count() = %d, want %d", count, tc.count)
			}
			for _, index := range []int{math.MinInt, -1, tc.count, math.MaxInt} {
				if module, ok := got.At(index); ok || module != (BuildDependency{}) {
					t.Fatalf("At(%d) = (%v, %t), want (zero, false)", index, module, ok)
				}
			}
			for index, expected := range want {
				module, ok := got.At(index)
				if !ok || module != expected {
					t.Fatalf("At(%d) = (%v, %t), want (%v, true)", index, module, ok, expected)
				}
				// Mutate the returned struct, then read the same slot again.
				module.path = GoModulePath{}
				module.version = GoModuleVersion{}
				module.sum = GoModuleSum{}
				if retained, ok := got.At(index); !ok || retained != expected || retained == module {
					t.Fatalf("At(%d) after returned-value mutation = (%v, %t), want (%v, true); overwritten copy = %v", index, retained, ok, expected, module)
				}
			}
		})
	}
}

func mustDependencyAccessDocument(t testing.TB, count int) []byte {
	t.Helper()
	fixture, err := newBuildDependencies(mustModulePath(t, testMainModule), CurrentGoToolchain(), numberedModules(t, count))
	if err != nil {
		t.Fatalf("newBuildDependencies(%d) error = %v, want nil", count, err)
	}
	document, err := fixture.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	return document
}
