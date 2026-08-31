package runnercontrol_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

var benchmarkExperimentObservationSink projectstandards.ExperimentObservation

func BenchmarkCompileExperimentObservation(b *testing.B) {
	request := experimentObservationRequestFixture(b)
	if err := request.Validate(); err != nil {
		b.Fatalf("ExperimentObservationRequest.Validate(benchmark workload) error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := runnercontrol.CompileExperimentObservation(request)
		if err != nil {
			b.Fatalf("CompileExperimentObservation(benchmark workload) error = %v, want nil", err)
		}
		benchmarkExperimentObservationSink = got
	}
	if err := benchmarkExperimentObservationSink.Validate(); err != nil {
		b.Fatalf("CompileExperimentObservation(benchmark result).Validate() error = %v, want nil", err)
	}
}

func TestExperimentObservationClassifierExhaustsExecutionStateDomain(t *testing.T) {
	t.Parallel()

	base := experimentObservationRequestFixture(t)
	nonzeroExit := *base.Process
	nonzeroExit.ExitCode = 7
	cases := []struct {
		name        string
		process     *process.ResultObservation
		failure     error
		wantOutcome projectstandards.Outcome
		wantStarted bool
	}{
		{name: "reaped zero exit is passed", process: base.Process, wantOutcome: projectstandards.OutcomePassed, wantStarted: true},
		{name: "reaped nonzero exit is failed", process: &nonzeroExit, failure: core.ErrProcessWait, wantOutcome: projectstandards.OutcomeFailed, wantStarted: true},
		{name: "reaped zero exit with evidence refusal is failed", process: base.Process, failure: core.ErrPrimitiveContract, wantOutcome: projectstandards.OutcomeFailed, wantStarted: true},
		{name: "pre-start cancellation is cancelled", failure: context.Canceled, wantOutcome: projectstandards.OutcomeCancelled},
		{name: "post-start cancellation retains started evidence", process: base.Process, failure: context.Canceled, wantOutcome: projectstandards.OutcomeCancelled, wantStarted: true},
		{name: "pre-start deadline is timed out", failure: context.DeadlineExceeded, wantOutcome: projectstandards.OutcomeTimedOut},
		{name: "post-start deadline retains started evidence", process: base.Process, failure: context.DeadlineExceeded, wantOutcome: projectstandards.OutcomeTimedOut, wantStarted: true},
		{name: "other pre-start failure is setup failed", failure: core.ErrProcessStart, wantOutcome: projectstandards.OutcomeSetupFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := base
			request.Process = tc.process
			request.Failure = tc.failure
			got, gotErr := runnercontrol.CompileExperimentObservation(request)
			if gotErr != nil {
				t.Fatalf("CompileExperimentObservation(%s) error = %v, want nil", tc.name, gotErr)
			}
			if got.Outcome != tc.wantOutcome || got.Started != tc.wantStarted {
				t.Fatalf("CompileExperimentObservation(%s) = outcome %s/started %t, want %s/%t", tc.name, got.Outcome.String(), got.Started, tc.wantOutcome.String(), tc.wantStarted)
			}
			if tc.wantStarted && got.Measurements.DurationNs != 1_000_000 {
				t.Fatalf("CompileExperimentObservation(%s) duration = %d, want 1000000", tc.name, got.Measurements.DurationNs)
			}
			if !tc.wantStarted && got.Measurements.DurationNs != 0 {
				t.Fatalf("CompileExperimentObservation(%s) duration = %d, want 0", tc.name, got.Measurements.DurationNs)
			}
		})
	}
}

func TestExperimentObservationClassifierLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive capability digest becomes the exact execution fingerprint", func(t *testing.T) {
		t.Parallel()
		request := experimentObservationRequestFixture(t)
		want, err := request.Capability.Digest()
		if err != nil {
			t.Fatalf("ExperimentCapability.Digest() setup error = %v, want nil", err)
		}
		got, gotErr := runnercontrol.CompileExperimentObservation(request)
		if gotErr != nil || got.ExecutionFingerprint != want {
			t.Fatalf("CompileExperimentObservation() = fingerprint %v/error %v, want %v/nil", got.ExecutionFingerprint, gotErr, want)
		}
	})

	t.Run("negative reversed execution interval fails with typed contract identity", func(t *testing.T) {
		t.Parallel()
		request := experimentObservationRequestFixture(t)
		request.BeganAt, request.CompletedAt = request.CompletedAt, request.BeganAt
		got, gotErr := runnercontrol.CompileExperimentObservation(request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Validate() == nil {
			t.Fatalf("CompileExperimentObservation(reversed interval) = (%+v, %v), want zero invalid observation and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral absent process requires an explicit execution refusal", func(t *testing.T) {
		t.Parallel()
		request := experimentObservationRequestFixture(t)
		request.Process = nil
		request.Failure = nil
		got, gotErr := runnercontrol.CompileExperimentObservation(request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Validate() == nil {
			t.Fatalf("CompileExperimentObservation(no process and no refusal) = (%+v, %v), want zero invalid observation and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})
}

func experimentObservationRequestFixture(t testing.TB) runnercontrol.ExperimentObservationRequest {
	t.Helper()
	payload := experimentCompletionPayloadFixture(t, true)
	command := mustProfileAbsolutePath(t, "/bin/sh")
	working := mustProfileAbsolutePath(t, "/tmp")
	arguments, argumentsErr := process.ParseArguments([]string{"-c", "true"})
	workspace := observationWorkspaceFixture(t)
	environment, environmentErr := process.ParseExactEnvironment([]string{
		core.EnvironmentHomeName + "=" + workspace.Home.String(), core.EnvironmentTemporaryName + "=" + workspace.Temporary.String(), core.EnvironmentCacheName + "=" + workspace.Cache.String(),
	})
	wait, waitErr := temporal.DurationFromNanoseconds(1)
	output, outputErr := core.NewByteCount(1024)
	if err := errors.Join(argumentsErr, environmentErr, waitErr, outputErr); err != nil {
		t.Fatalf("experiment observation process plan setup error = %v, want nil", err)
	}
	budget, budgetErr := runnercontrol.NewExecutionBudget(wait, 1, 1)
	if budgetErr != nil {
		t.Fatalf("runnercontrol.NewExecutionBudget(observation fixture) error = %v, want nil", budgetErr)
	}
	plan := process.Plan{SchemaVersion: process.ExecutionPlanSchemaVersion, Command: command, WorkingDirectory: working, Arguments: arguments, Environment: environment, OutputLimit: output, WaitDelay: wait, Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate}}
	capability := runnercontrol.ExperimentCapability{
		SchemaVersion: runnercontrol.SchemaVersion, MemberCapabilityDigest: core.SHA256Of([]byte("member")), Fence: payload.Fence,
		Run: payload.Run, Experiment: payload.Observation.Experiment, Probe: payload.Probe, Source: payload.Probe.Source,
		Execution: runnercontrol.ExperimentExecution{Process: plan, Workspace: workspace, Subject: subjectExecutionFixture(t, working), Artifacts: []runnercontrol.ArtifactExpectation{}, Observation: runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationOpaque}, Budget: budget}, Resources: resourceRequirement(t, 1, 1024, 2, 2, false), BuildContextDigest: core.SHA256Of([]byte("execution-context")),
		ExpiresAt: temporal.InstantFromNanoseconds(4_000_000),
	}
	if err := capability.Validate(); err != nil {
		t.Fatalf("ExperimentCapability.Validate(observation fixture) error = %v, want nil", err)
	}
	return runnercontrol.ExperimentObservationRequest{
		Capability: capability, BeganAt: *payload.StartedAt, CompletedAt: payload.CompletedAt,
		Process: payload.Process, Artifacts: []projectstandards.ArtifactReference{},
	}
}

func observationWorkspaceFixture(t testing.TB) runnercontrol.WritableWorkspace {
	t.Helper()
	return runnercontrol.WritableWorkspace{
		Root: mustProfileAbsolutePath(t, "/workspace"), Home: mustProfileAbsolutePath(t, "/workspace/home"),
		Output: mustProfileAbsolutePath(t, "/workspace/output"), Cache: mustProfileAbsolutePath(t, "/workspace/cache"), Temporary: mustProfileAbsolutePath(t, "/workspace/tmp"),
	}
}
