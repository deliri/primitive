package compass_test

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
)

const compassStringBatch = 64

func BenchmarkProjectNameStringBatch(b *testing.B) {
	text := strings.Repeat("n", 128)
	name, err := compass.ParseProjectName(text)
	if err != nil || name.String() != text {
		b.Fatalf("name fixture error=%v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range compassStringBatch {
			got := name.String()
			if got != text {
				b.Fatalf("String got=%q, want %q", got, text)
			}
		}
	}
	b.ReportMetric(compassStringBatch, "strings/op")
}
