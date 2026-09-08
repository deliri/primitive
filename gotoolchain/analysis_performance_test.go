package gotoolchain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Fixed production/internal-test pair sharing net/http exports. Compiler
// metadata loading is outside timing; source and export reads remain measured.
// The final typed declarations make all measured work observable.
func BenchmarkPackageVariantCheck(b *testing.B) {
	directory := b.TempDir()
	for _, file := range []struct{ path, source string }{
		{"go.mod", "module example.com/variants\n\ngo 1.27.1\n"},
		{"value.go", "package variants\nimport \"net/http\"\nfunc Header(r *http.Request) http.Header { return r.Header }\n"},
		{"value_test.go", "package variants\nimport \"net/http\"\nvar TestValue = http.MethodGet\n"},
	} {
		if err := os.WriteFile(filepath.Join(directory, file.path), []byte(file.source), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		b.Fatal(err)
	}
	pkg, err := gomodule.ParseImportPath("example.com/variants")
	if err != nil {
		b.Fatal(err)
	}
	limits, err := DefaultLimits()
	if err != nil {
		b.Fatal(err)
	}
	capability, err := Open(b.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		b.Fatal(err)
	}
	request := AnalysisRequest{WorkingDirectory: root, Package: pkg, IncludeTests: true}
	loaded, err := capability.loadAnalysisMetadata(b.Context(), root, []gomodule.ImportPath{pkg}, true)
	if err != nil {
		b.Fatal(err)
	}
	exports := collectCanonicalExports(loaded)
	var result PackageAnalysis
	b.ReportAllocs()
	for b.Loop() {
		result, err = compilePackageAnalysis(b.Context(), loaded, request, exports)
		if err != nil {
			b.Fatal(err)
		}
	}
	if err := result.Validate(); err != nil {
		b.Fatal(err)
	}
	if len(result.Units) != 2 {
		b.Fatalf("checked units = %d, want production and internal-test", len(result.Units))
	}
	declarations, tests := 0, 0
	for _, unit := range result.Units {
		if unit.Types.Scope().Lookup("Header") != nil {
			declarations++
		}
		if unit.Types.Scope().Lookup("TestValue") != nil {
			tests++
		}
	}
	if declarations != 2 || tests != 1 {
		b.Fatalf("typed declarations = %d/%d, want Header in both and TestValue in one", declarations, tests)
	}
}
