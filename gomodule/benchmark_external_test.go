package gomodule_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/gomodule"
)

func BenchmarkParsePath(b *testing.B) {
	const value = "github.com/deliri/primitive/v2026"
	fixture, err := gomodule.ParsePath(value)
	if err != nil || fixture.String() != value {
		b.Fatalf("benchmark workload = (%v, %v), want %q and nil", fixture, err, value)
	}
	b.SetBytes(int64(len(value)))
	var wantErr error
	b.ReportAllocs()
	var last gomodule.Path
	for b.Loop() {
		path, err := gomodule.ParsePath(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("gomodule.ParsePath() error = %v, want %v", err, wantErr)
		}
		last = path
	}
	if last.String() != value {
		b.Fatalf("gomodule.ParsePath() = %q, want %q", last, value)
	}
}

func BenchmarkParseImportPath(b *testing.B) {
	const value = "github.com/deliri/primitive/v2026/filestore"
	fixture, err := gomodule.ParseImportPath(value)
	if err != nil || fixture.String() != value {
		b.Fatalf("benchmark workload = (%v, %v), want %q and nil", fixture, err, value)
	}
	b.SetBytes(int64(len(value)))
	var wantErr error
	b.ReportAllocs()
	var last gomodule.ImportPath
	for b.Loop() {
		path, err := gomodule.ParseImportPath(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("gomodule.ParseImportPath() error = %v, want %v", err, wantErr)
		}
		last = path
	}
	if last.String() != value {
		b.Fatalf("gomodule.ParseImportPath() = %q, want %q", last, value)
	}
}
