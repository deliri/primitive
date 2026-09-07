package capabilities_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
)

func BenchmarkParseIdentity(b *testing.B) {
	const value = "filesystem"
	var wantErr error
	b.ReportAllocs()
	var last capabilities.Identity
	for b.Loop() {
		identity, err := capabilities.ParseIdentity(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("capabilities.ParseIdentity() error = %v, want %v", err, wantErr)
		}
		last = identity
	}
	if last.String() != value {
		b.Fatalf("capabilities.ParseIdentity() = %q, want %q", last, value)
	}
}

func BenchmarkCatalogAll(b *testing.B) {
	var wantErr error
	b.ReportAllocs()
	var last capabilities.Catalog
	for b.Loop() {
		catalog, err := capabilities.All()
		if !errors.Is(err, wantErr) {
			b.Fatalf("capabilities.All() error = %v, want %v", err, wantErr)
		}
		last = catalog
	}
	if err := last.Validate(); err != nil {
		b.Fatalf("capabilities.All() Validate() error = %v, want nil", err)
	}
}
