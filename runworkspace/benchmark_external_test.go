package runworkspace_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/runworkspace"
)

func BenchmarkParseGoDeclarations(b *testing.B) {
	source := []byte("package p\nimport \"testing\"\nfunc TestAlpha(t *testing.T) {}\nfunc BenchmarkEncode(b *testing.B) {}\n")
	kinds := []runprotocol.ProbeKind{runprotocol.ProbeKindGoTest, runprotocol.ProbeKindGoBenchmark}
	var wantErr error
	b.ReportAllocs()
	var last []runworkspace.GoDeclaration
	for b.Loop() {
		got, err := runworkspace.ParseGoDeclarations(source, kinds)
		if !errors.Is(err, wantErr) {
			b.Fatalf("runworkspace.ParseGoDeclarations() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if len(last) != 2 {
		b.Fatalf("runworkspace.ParseGoDeclarations() count = %d, want 2", len(last))
	}
}

func BenchmarkParseResidueCount(b *testing.B) {
	const value = "42\n"
	var wantErr error
	b.ReportAllocs()
	var last uint32
	for b.Loop() {
		got, err := runworkspace.ParseResidueCount(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("runworkspace.ParseResidueCount() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last != 42 {
		b.Fatalf("runworkspace.ParseResidueCount() = %d, want 42", last)
	}
}
