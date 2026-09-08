package release

import "testing"

// Walk the same frozen, decoded closure as a caller. Exact comparisons make
// every visited slot observable; decoding and fixture creation are untimed.
// Singleton batches contain 16 walks to avoid Go's iteration ceiling before
// the requested 30-second duration. Maximum batches contain one full walk.
func BenchmarkBuildDependenciesWalk(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		count int
		walks int
	}{
		{name: "SingletonBatch16", count: 1, walks: 16},
		{name: "Maximum", count: BuildDependencyMaximumCount, walks: 1},
	} {
		b.Run(tc.name, func(b *testing.B) {
			want := numberedModules(b, tc.count)
			var closure BuildDependencies
			if err := closure.UnmarshalJSON(mustDependencyAccessDocument(b, tc.count)); err != nil {
				b.Fatalf("UnmarshalJSON() error = %v, want nil", err)
			}
			if count := closure.Count(); count != len(want) || count == 0 {
				b.Fatalf("Count() = %d, want nonempty %d", count, len(want))
			}
			b.ReportAllocs()
			for b.Loop() {
				for range tc.walks {
					visited := 0
					for index := 0; index < closure.Count(); index++ {
						module, ok := closure.At(index)
						if !ok || module != want[index] {
							b.Fatalf("At(%d) = (%v, %t), want (%v, true)", index, module, ok, want[index])
						}
						visited++
					}
					if visited != tc.count {
						b.Fatalf("visited = %d, want %d", visited, tc.count)
					}
				}
			}
		})
	}
}
