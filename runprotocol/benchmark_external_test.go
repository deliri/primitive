package runprotocol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/runprotocol"
)

func BenchmarkParseSourcePath(b *testing.B) {
	const value = "filestore/walk.go"
	var wantErr error
	b.ReportAllocs()
	var last runprotocol.SourcePath
	for b.Loop() {
		got, err := runprotocol.ParseSourcePath(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("runprotocol.ParseSourcePath() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("runprotocol.ParseSourcePath() = %q, want %q", last, value)
	}
}
