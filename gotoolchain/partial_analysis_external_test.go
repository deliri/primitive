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

// Each mutation changes one independently compiled test unit. The production
// type and declaration must survive; failed units remain explicitly partial.
func TestAnalysisUnitFailureIsolationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		testSource      string
		wantErr         error
		wantTest        bool
		breakProduction bool
	}{
		{name: "valid test retains its declaration", testSource: "package probe\nconst TestValue = 2\n", wantTest: true},
		{name: "no tests retains production alone"},
		{name: "internal test type failure retains production", testSource: "package probe\nconst TestValue = missing\n", wantErr: core.ErrGoToolchainOutput},
		{name: "external test type failure retains production", testSource: "package probe_test\nconst TestValue = missing\n", wantErr: core.ErrGoToolchainOutput},
		{name: "test syntax failure retains production", testSource: "package probe\nfunc TestValue(\n", wantErr: core.ErrGoToolchainOutput},
		{name: "test dependency failure retains production", testSource: "package probe\nimport _ \"example.invalid/absent\"\n", wantErr: core.ErrGoToolchainOutput},
		{name: "production failure retains independent external test", testSource: "package probe_test\nconst TestValue = 2\n", breakProduction: true, wantTest: true, wantErr: core.ErrGoToolchainOutput},
		{name: "no surviving units retains metadata and explicit gap", breakProduction: true, wantErr: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range []struct{ name, data string }{
				{"go.mod", "module example.com/probe\n\ngo 1.27.1\n"},
				{"probe.go", "package probe\nconst Value = 1\n"},
			} {
				if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.data), 0o600); err != nil {
					t.Fatal(err)
				}
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
			request := gotoolchain.AnalysisRequest{WorkingDirectory: root, Package: pkg, IncludeTests: true}
			before, err := capability.AnalyzePackage(t.Context(), request)
			if err != nil || len(before.Units) != 1 || !analysisHasDeclaration(before, "Value") {
				t.Fatalf("baseline = %d units/%v, want production declaration", len(before.Units), err)
			}
			if tc.testSource != "" {
				if err := os.WriteFile(filepath.Join(directory, "probe_test.go"), []byte(tc.testSource), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.breakProduction {
				if err := os.WriteFile(filepath.Join(directory, "probe.go"), []byte("package probe\nconst Value = missing\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := capability.AnalyzePackage(t.Context(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("analysis = %v, want %v", err, tc.wantErr)
			}
			foundTest, foundProduction := false, false
			for _, unit := range got.Units {
				if unit.IllTyped {
					continue
				}
				foundTest = foundTest || unit.Types.Scope().Lookup("TestValue") != nil
				foundProduction = foundProduction || unit.Types.Scope().Lookup("Value") != nil
			}
			if foundProduction == tc.breakProduction || foundTest != tc.wantTest || got.Incomplete != (tc.wantErr != nil) {
				t.Fatalf("retained declarations = %+v, want production and valid tests only", got.Units)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("retained compiler units invalid: %v", err)
			}
			for _, unit := range got.Units {
				if unit.IllTyped != (len(unit.Errors) != 0) {
					t.Fatalf("compiler unit lost its partial/error distinction: %s", unit.ID)
				}
			}
		})
	}
}
