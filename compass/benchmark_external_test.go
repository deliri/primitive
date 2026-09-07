package compass_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
)

func BenchmarkCurrent(b *testing.B) {
	var wantErr error
	b.ReportAllocs()
	var last compass.Configuration
	for b.Loop() {
		configuration, err := compass.Current()
		if !errors.Is(err, wantErr) {
			b.Fatalf("compass.Current() error = %v, want %v", err, wantErr)
		}
		last = configuration
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("compass.Current() Validate() error = %v, want nil", err)
	}
}
