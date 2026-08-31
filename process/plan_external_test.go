package process_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestExecutionPlanJSONAndBindingLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical plan round trips and binds only runner streams", func(t *testing.T) {
		t.Parallel()
		request := processRequest(t, "silent", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		})
		plan := planFromRequest(request)
		encoded, encodeErr := plan.MarshalJSON()
		if encodeErr != nil || len(encoded) == 0 {
			t.Fatalf("Plan.MarshalJSON() = (%d bytes, %v), want nonzero and nil", len(encoded), encodeErr)
		}
		var decoded process.Plan
		if err := decoded.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("Plan.UnmarshalJSON() error = %v, want nil", err)
		}
		second, secondErr := decoded.MarshalJSON()
		if secondErr != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("Plan canonical closure = (%q, %v), want (%q, nil)", second, secondErr, encoded)
		}
		bound, bindErr := decoded.Bind(request.Streams)
		if bindErr != nil || bound.Command != request.Command || bound.WorkingDirectory != request.WorkingDirectory || len(bound.Arguments) != len(request.Arguments) {
			t.Fatalf("Plan.Bind() = (%+v, %v), want exact authority facts and nil", bound, bindErr)
		}
	})

	t.Run("negative inherited environment cannot cross the execution capability boundary", func(t *testing.T) {
		t.Parallel()
		request := processRequest(t, "silent", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		})
		plan := planFromRequest(request)
		plan.Environment = process.Environment{Mode: process.EnvironmentModeInherit}
		if gotErr := plan.Validate(); !errors.Is(gotErr, core.ErrProcessContract) {
			t.Fatalf("Plan.Validate(inherited environment) error = %v, want errors.Is(..., %v)", gotErr, core.ErrProcessContract)
		}
	})

	t.Run("neutral empty exact environment and silent arguments remain explicit", func(t *testing.T) {
		t.Parallel()
		request := processRequest(t, "silent", process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		})
		plan := planFromRequest(request)
		plan.Environment = process.Environment{Mode: process.EnvironmentModeExact, Variables: []process.EnvironmentVariable{}}
		encoded, gotErr := plan.MarshalJSON()
		if gotErr != nil || !bytes.Contains(encoded, []byte(`"environment":[]`)) {
			t.Fatalf("Plan.MarshalJSON(empty exact environment) = (%q, %v), want explicit empty environment and nil", encoded, gotErr)
		}
	})
}

func planFromRequest(request process.Request) process.Plan {
	containment := request.Containment
	if containment == (process.Containment{}) {
		containment = process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill}
	}
	environment := request.Environment
	if environment.Mode == process.EnvironmentModeInherit {
		environment = process.Environment{Mode: process.EnvironmentModeExact, Variables: []process.EnvironmentVariable{}}
	}
	return process.Plan{
		SchemaVersion: process.ExecutionPlanSchemaVersion,
		Command:       request.Command, WorkingDirectory: request.WorkingDirectory,
		Arguments: request.Arguments, Environment: environment,
		OutputLimit: request.OutputLimit, WaitDelay: request.WaitDelay,
		Containment: containment,
	}
}
