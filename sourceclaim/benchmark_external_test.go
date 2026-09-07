package sourceclaim_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/sourceclaim"
)

func BenchmarkNewID(b *testing.B) {
	const value = "exchange-bounds-http"
	var wantErr error
	b.ReportAllocs()
	var last sourceclaim.ID
	for b.Loop() {
		id, err := sourceclaim.NewID(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("sourceclaim.NewID() error = %v, want %v", err, wantErr)
		}
		last = id
	}
	if last.String() != value {
		b.Fatalf("sourceclaim.NewID() = %q, want %q", last, value)
	}
}
