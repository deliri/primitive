package sourceobservation_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/sourceobservation"
)

func BenchmarkNewLanguage(b *testing.B) {
	const value = "go"
	var wantErr error
	b.ReportAllocs()
	var last sourceobservation.Language
	for b.Loop() {
		got, err := sourceobservation.NewLanguage(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("sourceobservation.NewLanguage() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("sourceobservation.NewLanguage() = %q, want %q", last, value)
	}
}
