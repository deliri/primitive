package controlplane_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
)

func BenchmarkParseProductStatus(b *testing.B) {
	const value = controlplane.ProductStatusActiveToken
	var wantErr error
	b.ReportAllocs()
	var last controlplane.ProductStatus
	for b.Loop() {
		got, err := controlplane.ParseProductStatus(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("controlplane.ParseProductStatus() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("controlplane.ParseProductStatus() = %q, want %q", last, value)
	}
}
