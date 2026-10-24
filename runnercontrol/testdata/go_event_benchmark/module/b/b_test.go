package b

import "testing"

func BenchmarkOther(b *testing.B) {
	for b.Loop() {
	}
}
