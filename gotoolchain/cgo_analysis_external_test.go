package gotoolchain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
)

func TestAnalyzePackageResolvesCgoThroughTheRealCompiler(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/cgoprobe\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "value.go"), []byte("package cgoprobe\nimport \"C\"\nfunc Value() int { return int(C.int(7)) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := gomodule.ParseImportPath("example.com/cgoprobe")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []struct{ name, value string }{
		{"CGO_ENABLED", "1"}, {"GOENV", "off"}, {"GOFLAGS", "-buildvcs=false"}, {"GOTOOLCHAIN", "local"}, {"GOPROXY", "off"}, {"GOSUMDB", "off"},
	} {
		environment = compilerEnvironmentVariable(t, environment, variable.name, variable.value)
	}
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &environment})
	if err != nil {
		t.Fatal(err)
	}
	got, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: root, Package: pkg})
	if err != nil || len(got.Units) == 0 || !analysisHasDeclaration(got, "Value") {
		t.Fatalf("AnalyzePackage(cgo) = %d units/%v, want complete compiler-resolved Value declaration", len(got.Units), err)
	}
}
