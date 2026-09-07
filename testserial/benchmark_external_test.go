package testserial_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkDeclarationValidate(b *testing.B) {
	declaration := core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessOutput,
		Scope:  core.TestIsolationScopePackageProcess,
	}
	var wantErr error
	b.ReportAllocs()
	var last error
	for b.Loop() {
		last = declaration.Validate()
		if !errors.Is(last, wantErr) {
			b.Fatalf("TestIsolationDeclaration.Validate() error = %v, want %v", last, wantErr)
		}
	}
}
