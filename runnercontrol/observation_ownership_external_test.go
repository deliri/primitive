package runnercontrol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestExperimentObservationOwnsAccountingSnapshot(t *testing.T) {
	t.Parallel()
	request := experimentObservationRequestFixture(t)
	request.Capability.Execution.Observation = runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1}
	request.Measurements.Accounting = &runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{{Sequence: 1, Planned: 1, Passed: 1, Cache: runprotocol.CacheDisabled}}}
	got, err := runnercontrol.CompileExperimentObservation(request)
	if err != nil {
		t.Fatalf("CompileExperimentObservation() error = %v, want nil", err)
	}
	request.Measurements.Accounting.Attempts[0].Passed = 0
	request.Measurements.Accounting.Attempts[0].Failed = 1
	if got.Measurements.Accounting.Attempts[0].Passed != 1 || got.Measurements.Accounting.Attempts[0].Failed != 0 {
		t.Fatalf("retained accounting = %+v, want original passed fact", got.Measurements.Accounting)
	}
	got.Measurements.Accounting.Attempts[0].Skipped = 1
	if request.Measurements.Accounting.Attempts[0].Skipped != 0 {
		t.Fatalf("request accounting = %+v, want no mutation from returned observation", request.Measurements.Accounting)
	}
}

func TestExperimentObservationAccountingContradictionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		attempt runprotocol.ExecutionAttempt
		wantErr error
	}{
		{name: "passed accounting agrees with zero exit", attempt: runprotocol.ExecutionAttempt{Passed: 1}},
		{name: "skipped accounting remains an explicit skip", attempt: runprotocol.ExecutionAttempt{Skipped: 1}},
		{name: "failed accounting cannot become passed observation", attempt: runprotocol.ExecutionAttempt{Failed: 1}, wantErr: core.ErrPrimitiveContract},
		{name: "unavailable accounting cannot become passed observation", attempt: runprotocol.ExecutionAttempt{Unavailable: 1}, wantErr: core.ErrPrimitiveContract},
		{name: "cancelled accounting cannot become passed observation", attempt: runprotocol.ExecutionAttempt{Cancelled: 1}, wantErr: core.ErrPrimitiveContract},
		{name: "expired accounting cannot become passed observation", attempt: runprotocol.ExecutionAttempt{Expired: 1}, wantErr: core.ErrPrimitiveContract},
		{name: "not run accounting cannot become passed observation", attempt: runprotocol.ExecutionAttempt{NotRun: 1}, wantErr: core.ErrPrimitiveContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := experimentObservationRequestFixture(t)
			request.Capability.Execution.Observation = runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1}
			attempt := tc.attempt
			attempt.Sequence, attempt.Planned, attempt.Cache = 1, 1, runprotocol.CacheDisabled
			request.Measurements.Accounting = &runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{attempt}}
			got, err := runnercontrol.CompileExperimentObservation(request)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got.Measurements.Accounting != nil {
					t.Fatalf("CompileExperimentObservation() = %+v/%v, want zero observation/%v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Measurements.Accounting.Attempts[0] != attempt {
				t.Fatalf("CompileExperimentObservation() = %+v/%v, want exact accounting/nil", got, err)
			}
		})
	}
}
