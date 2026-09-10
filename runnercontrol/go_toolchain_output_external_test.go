package runnercontrol_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoTestObservationCompilerAcceptsRealToolchainOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":         "module outputfixture\n\ngo 1.27.0\n",
		"output_test.go": "package outputfixture\nimport \"testing\"\nfunc TestOutput(t *testing.T) { t.Error(\"first error line\\nsecond error line\") }\n",
	} {
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
		t.Fatalf("go test fixture error = %v/stderr %s, want exit 1", executionErr, &stderr)
	}
	if !bytes.Contains(data, []byte(`"OutputType":"frame"`)) || !bytes.Contains(data, []byte(`"OutputType":"error"`)) {
		t.Fatalf("go test fixture output = %s, want framing and error metadata", data)
	}
	compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
	if err != nil {
		t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
	}
	if n, err := compiler.Write(data); err != nil || n != len(data) {
		t.Fatalf("Write(real output) = %d/%v, want %d/nil", n, err, len(data))
	}
	got, err := compiler.Seal(executionErr)
	if err != nil || len(got.Accounting.Attempts) != 1 {
		t.Fatalf("Seal(real output) = %+v/%v, want one accounted attempt/nil", got, err)
	}
	if attempt := got.Accounting.Attempts[0]; attempt.Failed != 1 || attempt.Passed != 0 || attempt.Unavailable != 0 || len(got.Benchmarks) != 0 {
		t.Fatalf("Seal(real output) = %+v, want one failed package and no benchmarks", got)
	}
}

func TestGoTestObservationOutputMetadataDomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, action, outputType string
		wantErr                  bool
	}{
		{name: "ordinary output has no classification metadata", action: "output"},
		{name: "framing output is admitted", action: "output", outputType: "frame"},
		{name: "error output is admitted", action: "output", outputType: "error"},
		{name: "continued error output is admitted", action: "output", outputType: "error-continue"},
		{name: "unknown output metadata is refused", action: "output", outputType: "future-output-kind", wantErr: true},
		{name: "output metadata on terminal action is refused", action: "pass", outputType: "frame", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			event := struct {
				Action     string
				Package    string
				Output     string
				OutputType string
			}{Action: tc.action, Package: "fixture", Output: "diagnostic\n", OutputType: tc.outputType}
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("Marshal(event) error = %v, want nil", err)
			}
			compiler, err := runnercontrol.NewGoTestObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1})
			if err != nil {
				t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
			}
			if _, err := compiler.Write(data); err != nil {
				t.Fatalf("Write() error = %v, want nil", err)
			}
			got, err := compiler.Seal(context.Canceled)
			if tc.wantErr {
				if !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("Seal(unknown metadata) error = %v, want %v", err, core.ErrJSONContract)
				}
				return
			}
			if err != nil || len(got.Accounting.Attempts) != 1 || got.Accounting.Attempts[0].Cancelled != 1 || len(got.Benchmarks) != 0 {
				t.Fatalf("Seal(diagnostic) = %+v/%v, want one cancelled package and no benchmarks/nil", got, err)
			}
		})
	}
}
