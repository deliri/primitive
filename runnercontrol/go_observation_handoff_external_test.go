package runnercontrol_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

// Exhaust the one-package state handoff: absent, active, and every terminal
// state crossed with success, failure, cancellation, and deadline. Multi-package
// cardinality and wire grammar have their own hostile producer tables.
func TestGoObservationProducerClassifierExhaustiveStateLayerTriad(t *testing.T) {
	t.Parallel()
	for _, source := range []struct {
		name, action            string
		passed, failed, skipped uint32
	}{
		{name: "absent"}, {name: "active", action: "start"},
		{name: "passed", action: "pass", passed: 1},
		{name: "failed", action: "fail", failed: 1},
		{name: "skipped", action: "skip", skipped: 1},
	} {
		for _, ending := range []struct {
			name                       string
			err                        error
			outcome                    runprotocol.Outcome
			failed, cancelled, expired uint32
		}{
			{name: "zero exit", outcome: runprotocol.OutcomePassed},
			{name: "process refusal", err: core.ErrProcessWait, outcome: runprotocol.OutcomeFailed, failed: 1},
			{name: "cancellation", err: context.Canceled, outcome: runprotocol.OutcomeCancelled, cancelled: 1},
			{name: "deadline", err: context.DeadlineExceeded, outcome: runprotocol.OutcomeTimedOut, expired: 1},
		} {
			t.Run(source.name+" then "+ending.name, func(t *testing.T) {
				t.Parallel()
				request := experimentObservationRequestFixture(t)
				policy := runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1}
				request.Capability.Execution.Observation = policy
				compiler, err := runnercontrol.NewGoTestObservationCompiler(policy)
				if err != nil {
					t.Fatal(err)
				}
				if source.action != "" {
					for _, chunk := range events(event(source.action, "selected", "", ""))(t) {
						if _, err = compiler.Write(chunk); err != nil {
							t.Fatal(err)
						}
					}
				}
				produced, producerErr := compiler.Seal(ending.err)
				want := runprotocol.ExecutionAttempt{Sequence: 1, Planned: 1, Cache: runprotocol.CacheDisabled, Passed: source.passed, Failed: source.failed, Skipped: source.skipped}
				incomplete := source.name == "absent" || source.name == "active"
				contradiction := ending.err == nil && (incomplete || source.failed == 1)
				primary := "boundary"
				if contradiction {
					primary = "contradiction"
				} else if source.skipped == 1 {
					primary = "neutral"
				}
				if incomplete {
					if ending.err == nil {
						want.Unavailable = 1
					} else {
						want.Failed, want.Cancelled, want.Expired = ending.failed, ending.cancelled, ending.expired
					}
				}
				attempt, ok := produced.Accounting.Latest()
				if !ok || attempt != want || len(produced.Benchmarks) != 0 {
					t.Fatalf("%s producer = %+v, want %+v and no benchmarks", primary, produced, want)
				}
				if contradiction {
					if !errors.Is(producerErr, core.ErrPrimitiveContract) {
						t.Fatalf("contradiction error = %v, want %v", producerErr, core.ErrPrimitiveContract)
					}
				} else if producerErr != nil {
					t.Fatalf("producer error = %v, want nil", producerErr)
				}
				request.Measurements.Accounting = &produced.Accounting
				request.Failure = errors.Join(ending.err, producerErr)
				got, classifierErr := runnercontrol.CompileExperimentObservation(request)
				wantOutcome := ending.outcome
				if contradiction {
					wantOutcome = runprotocol.OutcomeFailed
				}
				if classifierErr != nil || got.Outcome != wantOutcome || got.Measurements.Accounting == nil || got.Measurements.Accounting.Attempts[0] != want || len(got.Artifacts) != 0 || len(got.Measurements.Benchmarks) != 0 {
					t.Fatalf("%s handoff = %+v/%v, want outcome %v, exact producer accounting, no extra evidence/nil", primary, got, classifierErr, wantOutcome)
				}
				// Removing exactly the refusal must never certify contradictory counters.
				if contradiction {
					request.Failure = nil
					refused, err := runnercontrol.CompileExperimentObservation(request)
					if !errors.Is(err, core.ErrPrimitiveContract) || refused.Measurements.Accounting != nil {
						t.Fatalf("erased refusal = %+v/%v, want zero/%v", refused, err, core.ErrPrimitiveContract)
					}
				}
			})
		}
	}
}
