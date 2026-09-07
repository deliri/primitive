package gotoolchain_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkParseToolchainVersion(b *testing.B) {
	const value = "go1.27.1"
	var wantErr error
	b.ReportAllocs()
	var last gotoolchain.ToolchainVersion
	for b.Loop() {
		got, err := gotoolchain.ParseToolchainVersion(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("gotoolchain.ParseToolchainVersion() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("gotoolchain.ParseToolchainVersion() = %q, want %q", last, value)
	}
}

func BenchmarkParsePackageName(b *testing.B) {
	const value = "filestore"
	var wantErr error
	b.ReportAllocs()
	var last gotoolchain.PackageName
	for b.Loop() {
		got, err := gotoolchain.ParsePackageName(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("gotoolchain.ParsePackageName() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("gotoolchain.ParsePackageName() = %q, want %q", last, value)
	}
}
