package textrepair

import (
	"strings"
	"testing"
)

var prefixBenchmarkSink string

func BenchmarkPrefixMalformedPrefix(b *testing.B) {
	source := strings.Repeat("\xffaé界😀", 128)
	request := Request{Source: source, MaximumBytes: prefixBudget(b, 1024)}
	got, err := Prefix(request)
	if err != nil || len(got) == 0 || len(got) > 1024 {
		b.Fatalf("setup prefix = %q/error %v, want nonempty bounded/nil", got, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := Prefix(request)
		if err != nil {
			b.Fatalf("Prefix error = %v, want nil", err)
		}
		prefixBenchmarkSink = result
	}
}
