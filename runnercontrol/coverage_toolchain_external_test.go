package runnercontrol_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoCoverageCompilerConsumesToolchainProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	profile := toolchainCoverageFixture(t, dir)
	compiler := runnercontrol.NewGoCoverageCompiler()
	for _, value := range profile {
		if n, err := compiler.Write([]byte{value}); n != 1 || err != nil {
			t.Fatalf("Write(profile byte) = %d/%v, want 1/nil", n, err)
		}
	}
	got, err := compiler.Seal()
	want := runnercontrol.GoCoverageObservation{Mode: runnercontrol.CoverageSet, Statements: 1, Covered: 1, BasisPoints: 10000}
	if err != nil || got != want {
		t.Fatalf("Seal(toolchain profile) = %+v/%v, want %+v/nil", got, err, want)
	}
}

// The installed Go compiler emits the canonical seed. No profile bytes are
// handwritten or pre-populated; the fixed source has one executed statement.
func toolchainCoverageFixture(t testing.TB, dir string) []byte {
	t.Helper()
	for name, content := range map[string]string{
		"go.mod":        "module coveragefixture\n\ngo 1.27.0\n",
		"value.go":      "package coveragefixture\nfunc Value() int { return 1 }\n",
		"value_test.go": "package coveragefixture\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"value differs\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-count=1", "-covermode=set", "-coverprofile=coverage.out", ".")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go test -coverprofile error = %v/output %s, want nil", err, output)
	}
	profile, err := os.ReadFile(filepath.Join(dir, "coverage.out"))
	if err != nil {
		t.Fatalf("ReadFile(generated profile) error = %v, want nil", err)
	}
	return profile
}
