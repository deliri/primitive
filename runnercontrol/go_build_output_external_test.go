package runnercontrol_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoBuildFailureCountsSelectedPackageOnce(t *testing.T) {
	t.Parallel()
	for _, dependency := range []bool{false, true} {
		name := "selected package compilation fails"
		if dependency {
			name = "dependency failure does not add a selected package"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			data, executionErr := goBuildFailureFixture(t, dir, dependency)
			compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
			}
			if n, err := compiler.Write(data); n != len(data) || err != nil {
				t.Fatalf("Write(toolchain output) = %d/%v, want %d/nil", n, err, len(data))
			}
			got, err := compiler.Seal(executionErr)
			if err != nil || len(got.Accounting.Attempts) != 1 {
				t.Fatalf("Seal(build failure) = %+v/%v, want one attempt/nil", got, err)
			}
			attempt := got.Accounting.Attempts[0]
			if attempt.Planned != 1 || attempt.Failed != 1 || attempt.Passed != 0 || attempt.Unavailable != 0 || attempt.NotRun != 0 || len(got.Benchmarks) != 0 {
				t.Fatalf("Seal(build failure) = %+v, want exactly one failed selected package and no benchmarks", got)
			}
		})
	}
}

func goBuildFailureFixture(t testing.TB, dir string, dependency bool) ([]byte, error) {
	t.Helper()
	files := map[string]string{"go.mod": "module buildfixture\n\ngo 1.27.0\n", "broken.go": "package buildfixture\nvar Value = missingName\n"}
	if dependency {
		if err := os.Mkdir(filepath.Join(dir, "dependency"), 0700); err != nil {
			t.Fatalf("Mkdir(dependency) error = %v, want nil", err)
		}
		files["broken.go"] = "package buildfixture\nimport _ \"buildfixture/dependency\"\n"
		files["dependency/broken.go"] = "package dependency\nvar Value = missingName\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", ".")
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	data, executionErr := command.Output()
	var exit *exec.ExitError
	if !errors.As(executionErr, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("go test error = %v/stderr %s, want exit 1", executionErr, &stderr)
	}
	if !bytes.Contains(data, []byte(`"Action":"build-fail"`)) || !bytes.Contains(data, []byte(`"ImportPath":`)) {
		t.Fatalf("go test output = %s, want build-fail and ImportPath members", data)
	}
	return data, executionErr
}
