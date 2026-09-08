package gotoolchain_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestAnalyzeTestOnlyPackageThroughTheRealCompiler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		source          string
		includeTests    bool
		wantErr         error
		wantDeclaration bool
	}{
		{name: "test only package retains test declarations", source: "package probe\nconst TestValue = 1\n", includeTests: true, wantDeclaration: true},
		{name: "test only package has an empty production unit", source: "package probe\nconst TestValue = 1\n"},
		{name: "invalid test source still refuses", source: "package probe\nconst TestValue = missing\n", includeTests: true, wantErr: core.ErrGoToolchainOutput},
		{name: "excluded invalid tests do not poison production", source: "package probe\nconst TestValue = missing\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/probe\n\ngo 1.27.1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "probe_test.go"), []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := gomodule.ParseImportPath("example.com/probe")
			if err != nil {
				t.Fatal(err)
			}
			limits, err := gotoolchain.DefaultLimits()
			if err != nil {
				t.Fatal(err)
			}
			capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits})
			if err != nil {
				t.Fatal(err)
			}
			got, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: root, Package: pkg, IncludeTests: tc.includeTests})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || !got.Incomplete || len(got.Units) != 2 {
					t.Fatalf("AnalyzePackage() = %d units/%v, want complete production and partial test units with %v", len(got.Units), err, tc.wantErr)
				}
				if got.Units[0].IllTyped || !got.Units[1].IllTyped || len(got.Units[1].Errors) == 0 {
					t.Fatalf("production/test IllTyped = %t/%t, test diagnostics = %d; want false/true and positive", got.Units[0].IllTyped, got.Units[1].IllTyped, len(got.Units[1].Errors))
				}
				if err := got.Validate(); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if declaration := analysisHasDeclaration(got, "TestValue"); declaration != tc.wantDeclaration {
				t.Fatalf("test declaration = %t, want %t", declaration, tc.wantDeclaration)
			}
			if got.Units[0].Types.Name() != "probe" || len(got.Units[0].Syntax) != 0 || !got.Units[0].Types.Complete() {
				t.Fatalf("production unit = %+v, want complete empty probe package", got.Units[0])
			}
		})
	}
}
