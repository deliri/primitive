package nm

import (
	"fmt"
	"testing"
)

func TestPass(t *testing.T) { t.Run("sub", func(t *testing.T) {}) }
func TestSkip(t *testing.T) { t.Skip("skip") }
func BenchmarkB(b *testing.B) {
	for b.Loop() {
	}
}
func FuzzF(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, n int) {}) }
func ExampleE() {
	fmt.Println("x") // Output: x
}
