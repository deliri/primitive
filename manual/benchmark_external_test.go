package manual_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/manual"
)

func BenchmarkParseLine(b *testing.B) {
	const value = "Run the published command."
	var wantErr error
	b.ReportAllocs()
	var last manual.Line
	for b.Loop() {
		got, err := manual.ParseLine(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("manual.ParseLine() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if string(last) != value {
		b.Fatalf("manual.ParseLine() = %q, want %q", last, value)
	}
}
