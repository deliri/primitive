package gotoolchain_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/packages"
)

// Exercise real cmd/go errors, then compare their typed position and message
// with the compiler's retained metadata. Flattening an error into prose must
// not destroy the caller's ability to identify its compiler-owned contract.
func TestAnalysisRetainsCompilerDiagnosticIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, file, source string }{
		{"direct missing import", "a/value.go", "package a\nimport _ \"example.invalid/missing\"\n"},
		{"transitive missing import", "b/value.go", "package b\nimport _ \"example.invalid/missing\"\n"},
		{"test missing import", "a/value_test.go", "package a\nimport _ \"example.invalid/missing\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capability, batch := analysisBatchFixture(t, t.TempDir())
			request := gotoolchain.AnalysisRequest{WorkingDirectory: batch.WorkingDirectory, Package: batch.Packages[0], IncludeTests: true}
			if _, err := capability.AnalyzePackage(t.Context(), request); err != nil {
				t.Fatalf("healthy compiler control: %v", err)
			}
			if tc.file == "b/value.go" {
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), "a/value.go"), []byte("package a\nimport _ \"example.com/batch/b\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), tc.file), []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			analysis, err := capability.AnalyzePackage(t.Context(), request)
			if !errors.Is(err, core.ErrGoToolchainOutput) || !analysis.Incomplete {
				t.Fatalf("broken compiler outcome: incomplete=%t err=%v", analysis.Incomplete, err)
			}
			var diagnostic packages.Error
			if !errors.As(err, &diagnostic) {
				t.Fatalf("original compiler error type lost: %v", err)
			}
			if diagnostic.Kind != packages.ListError || diagnostic.Msg == "" || diagnostic.Pos == "" {
				t.Fatalf("compiler diagnostic lost kind, message or import position: %+v", diagnostic)
			}
			found := false
			for _, unit := range analysis.Metadata {
				for _, original := range unit.Errors {
					if original == diagnostic {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("diagnostic did not retain compiler metadata identity: %+v", diagnostic)
			}
		})
	}
}

// A first diagnostic is not the compiler's complete failure census. Each
// original cmd/go error must remain reachable through the public typed chain,
// including failures reached through a local dependency.
func TestAnalysisRetainsEveryCompilerLoadDiagnostic(t *testing.T) {
	t.Parallel()
	for _, transitive := range []bool{false, true} {
		name := "direct imports"
		if transitive {
			name = "transitive imports"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			capability, batch := analysisBatchFixture(t, t.TempDir())
			request := gotoolchain.AnalysisRequest{WorkingDirectory: batch.WorkingDirectory, Package: batch.Packages[0]}
			if _, err := capability.AnalyzePackage(t.Context(), request); err != nil {
				t.Fatalf("healthy control: %v", err)
			}
			directory, packageName := "a", "a"
			if transitive {
				directory, packageName = "b", "b"
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), "a/value.go"), []byte("package a\nimport _ \"example.com/batch/b\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, fixture := range []struct{ filename, source string }{
				{"value.go", "package " + packageName + "\nimport _ \"example.invalid/first\"\n"},
				{"second.go", "package " + packageName + "\nimport _ \"example.invalid/second\"\n"},
			} {
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), directory, fixture.filename), []byte(fixture.source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			analysis, err := capability.AnalyzePackage(t.Context(), request)
			if !errors.Is(err, core.ErrGoToolchainOutput) || !analysis.Incomplete {
				t.Fatalf("broken compiler outcome: incomplete=%t err=%v", analysis.Incomplete, err)
			}
			var originals []packages.Error
			for _, unit := range analysis.Metadata {
				originals = append(originals, unit.Errors...)
			}
			if len(originals) < 2 {
				t.Fatalf("fault failed to produce multiple compiler diagnostics: %+v", originals)
			}
			for _, original := range originals {
				if !errors.Is(err, original) {
					t.Errorf("public error chain lost compiler diagnostic %+v: %v", original, err)
				}
			}
		})
	}
}
