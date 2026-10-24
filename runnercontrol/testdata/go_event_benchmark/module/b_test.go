package nm

import "testing"

func BenchmarkSkip(b *testing.B) { b.Skip("no") }
func BenchmarkFail(b *testing.B) { b.Fatal("bad") }
func BenchmarkSub(b *testing.B) {
	b.Run("s", func(b *testing.B) {
		for b.Loop() {
		}
	})
}
func FuzzSkip(f *testing.F) { f.Skip("no") }
func TestPar(t *testing.T) {
	for _, n := range []string{"a", "b"} {
		t.Run(n, func(t *testing.T) { t.Parallel(); t.Skip("x") })
	}
}
