package gotoolchain_test

import (
	"errors"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

// A real compiler failure in one file must not erase resolved facts from its
// sibling file. Partial facts retain compiler diagnostics and cannot validate
// as a successful complete analysis. No compiler process is mocked.
func TestPartialFileAnalysisRetainsResolvedSiblingFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, file  string
		broken, foreignName bool
		minimumTypeErrors   int
	}{
		{name: "healthy control", source: "package probe\nconst Broken = 1\n"},
		{name: "type error", source: "package probe\nvar Broken = Missing\n", broken: true, minimumTypeErrors: 1},
		{name: "multiple type errors", source: "package probe\nvar Broken = Missing\nvar Another = AlsoMissing\n", broken: true, minimumTypeErrors: 2},
		{name: "parser error", source: "package probe\nfunc Broken( {\n", broken: true},
		{name: "missing package clause", source: "ABSENT\n", broken: true},
		{name: "missing dependency", source: "package probe\nimport _ \"example.invalid/missing\"\n", broken: true},
		{name: "conflicting first clause preserves compiler name", source: "package alternate\nconst Broken = 1\n", broken: true, foreignName: true},
		{name: "conflicting later clause preserves selected sibling", source: "package alternate\nconst Broken = 1\n", file: "zzbroken.go", broken: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			filename := tc.file
			if filename == "" {
				filename = "broken.go"
			}
			for _, file := range []struct{ name, body string }{
				{"go.mod", "module example.com/probe\n\ngo 1.27.1\n"},
				{"healthy.go", "package probe\nfunc Target(v int) int { return v }\nfunc Healthy() int { return Target(7) }\n"},
				{filename, tc.source},
			} {
				if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.body), 0o600); err != nil {
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
			result, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: root, Package: pkg})
			if tc.broken {
				if !errors.Is(err, core.ErrGoToolchainOutput) || !result.Incomplete {
					t.Fatalf("invalid file became complete: incomplete=%t error=%v", result.Incomplete, err)
				}
			} else if err != nil || result.Incomplete {
				t.Fatalf("healthy control failed: incomplete=%t error=%v", result.Incomplete, err)
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("retained partial compiler contract invalid: %v", err)
			}
			if len(result.Units) != 1 {
				t.Fatalf("one failed file erased the package's resolved compiler unit: %d units", len(result.Units))
			}
			unit := result.Units[0]
			if unit.IllTyped != tc.broken || (len(unit.Errors) != 0) != tc.broken {
				t.Fatalf("compiler error state not retained: illTyped=%t diagnostics=%v", unit.IllTyped, unit.Errors)
			}
			var resolved bool
			for identifier, object := range unit.TypesInfo.Uses {
				if identifier.Name != "Target" || object == nil {
					continue
				}
				if _, ok := object.Type().(*types.Signature); !ok {
					t.Fatalf("resolved target lost compiler signature: %T", object.Type())
				}
				if object.Pkg() == nil || object.Pkg().Path() != pkg.String() {
					t.Fatalf("resolved target owner = %v, want %s", object.Pkg(), pkg.String())
				}
				resolved = true
			}
			// Conflicting package clauses have no single valid package. Preserve
			// cmd/go's chosen name; do not invent which clause the author intended.
			wantedName := "probe"
			if tc.foreignName {
				wantedName = "alternate"
			}
			if unit.Name != wantedName || unit.Name != result.Metadata[0].Name || resolved == tc.foreignName {
				t.Fatalf("compiler ownership or resolved sibling facts changed: name=%s resolved=%t", unit.Name, resolved)
			}
			if len(unit.TypeErrors) < tc.minimumTypeErrors {
				t.Fatalf("checker stopped before all independent errors: %v", unit.TypeErrors)
			}
			for _, original := range result.Metadata[0].Errors {
				if !errors.Is(err, original) {
					t.Errorf("original load diagnostic lost: %+v", original)
				}
			}
			if tc.broken {
				result.Incomplete = false
				if gotErr := result.Validate(); !errors.Is(gotErr, core.ErrGoToolchainContract) {
					t.Fatalf("partial analysis validation = %v, want %v", gotErr, core.ErrGoToolchainContract)
				}
				copy := *unit
				copy.IllTyped, copy.Errors, copy.TypeErrors = false, nil, nil
				result.Units = append(result.Units[:0:0], &copy)
				if len(result.Metadata[0].Errors) == 0 {
					t.Fatalf("metadata diagnostic count = %d, want positive", len(result.Metadata[0].Errors))
				}
				if gotErr := result.Validate(); !errors.Is(gotErr, core.ErrGoToolchainContract) {
					t.Fatalf("cleared unit error validation = %v, want %v from metadata", gotErr, core.ErrGoToolchainContract)
				}
			}
		})
	}
}
