package a

import (
	"strings"
	"testing"
)

func BenchmarkSlow(b *testing.B) {
	for b.Loop() {
		_ = strings.Repeat("x", 64)
	}
}

func BenchmarkLong(b *testing.B) {
	b.Run(strings.Repeat("n", 1100), func(b *testing.B) {
		for b.Loop() {
		}
	})
}
