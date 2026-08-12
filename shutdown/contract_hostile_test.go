package shutdown

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// requireRejection proves the load-bearing typed class first and only then the
// operator-facing diagnostic. The diagnostic never stands alone as the
// rejection proof, so a regression to a different typed error that happens to
// render the same prose still fails.
func requireRejection(t *testing.T, got error, wantIdentity error, wantDiagnostic diagnostic) {
	t.Helper()
	if !errors.Is(got, wantIdentity) {
		t.Fatalf("error = %v, want identity %v", got, wantIdentity)
	}
	var observed diagnostic
	if !errors.As(got, &observed) || observed != wantDiagnostic {
		t.Fatalf("error diagnostic = %q, want %q", observed, wantDiagnostic)
	}
}

// TestRegisterNamesEveryRejectionClassDistinctly pins the operator contract for
// the plan trust boundary. Every rejection previously collapsed into one
// diagnostic-free core.ErrShutdownContract, so a full plan, a duplicate
// identity, and a closed plan were indistinguishable to a caller.
func TestRegisterNamesEveryRejectionClassDistinctly(t *testing.T) {
	t.Parallel()

	one := durationForTest(t, time.Nanosecond)
	two := durationForTest(t, 2*time.Nanosecond)
	maximum := durationForTest(t, time.Duration(1<<63-1))
	nearMaximum := durationForTest(t, time.Duration(1<<63-2))
	zero := durationForTest(t, 0)

	admitted := []struct {
		name string
		step Step
	}{
		{name: "minimum step budget at one nanosecond", step: validStep(t, 1, PhaseDrain, one)},
		{name: "one above the minimum step budget", step: validStep(t, 1, PhaseDrain, two)},
		{name: "maximum representable step budget", step: validStep(t, 1, PhaseDrain, maximum)},
		{name: "one below the maximum step budget", step: validStep(t, 1, PhaseDrain, nearMaximum)},
		{name: "minimum nonzero step identity", step: validStep(t, 1, PhaseDrain, one)},
		{name: "one above the minimum step identity", step: validStep(t, 2, PhaseDrain, one)},
		{name: "maximum step identity", step: validStep(t, ^uint16(0), PhaseDrain, one)},
		{name: "one below the maximum step identity", step: validStep(t, ^uint16(0)-1, PhaseDrain, one)},
		{name: "first phase is admitted", step: validStep(t, 1, PhaseStopAdmission, one)},
		{name: "drain phase is admitted", step: validStep(t, 1, PhaseDrain, one)},
		{name: "persist phase is admitted", step: validStep(t, 1, PhasePersist, one)},
		{name: "flush phase is admitted", step: validStep(t, 1, PhaseFlush, one)},
		{name: "last phase is admitted", step: validStep(t, 1, PhaseRelease, one)},
		{name: "step budget above the total budget is clipped, not rejected", step: validStep(t, 1, PhaseDrain, maximum)},
	}
	for _, tc := range admitted {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan, err := NewPlan(PlanPolicy{TotalBudget: two})
			if err != nil {
				t.Fatalf("NewPlan() error = %v, want nil", err)
			}
			if err := plan.Register(tc.step); err != nil {
				t.Fatalf("Register() error = %v, want nil", err)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("Plan.Validate() error = %v, want nil", err)
			}
		})
	}

	rejected := []struct {
		name           string
		step           Step
		wantDiagnostic diagnostic
	}{
		{
			name:           "zero step identity is rejected",
			step:           Step{ID: StepID{}, Phase: PhaseDrain, Budget: one, Action: noopAction},
			wantDiagnostic: diagnosticStepIdentityZero,
		},
		{
			name:           "unknown zero phase is rejected",
			step:           validStep(t, 1, PhaseUnknown, one),
			wantDiagnostic: diagnosticPhaseUnsupported,
		},
		{
			name:           "phase exactly at the private limit is rejected",
			step:           validStep(t, 1, phaseLimit, one),
			wantDiagnostic: diagnosticPhaseUnsupported,
		},
		{
			name:           "phase one above the private limit is rejected",
			step:           validStep(t, 1, phaseLimit+1, one),
			wantDiagnostic: diagnosticPhaseUnsupported,
		},
		{
			name:           "maximum byte phase is rejected",
			step:           validStep(t, 1, Phase(^uint8(0)), one),
			wantDiagnostic: diagnosticPhaseUnsupported,
		},
		{
			name:           "zero step budget is rejected",
			step:           validStep(t, 1, PhaseDrain, zero),
			wantDiagnostic: diagnosticStepBudgetInvalid,
		},
		{
			name:           "unset step budget is rejected",
			step:           Step{ID: stepIDForTest(t, 1), Phase: PhaseDrain, Action: noopAction},
			wantDiagnostic: diagnosticStepBudgetInvalid,
		},
		{
			name:           "nil action is rejected",
			step:           Step{ID: stepIDForTest(t, 1), Phase: PhaseDrain, Budget: one},
			wantDiagnostic: diagnosticStepActionNil,
		},
		{
			name:           "wholly unset step is rejected",
			step:           Step{},
			wantDiagnostic: diagnosticStepIdentityZero,
		},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan, err := NewPlan(PlanPolicy{TotalBudget: one})
			if err != nil {
				t.Fatalf("NewPlan() error = %v, want nil", err)
			}
			requireRejection(t, plan.Register(tc.step), core.ErrShutdownContract, tc.wantDiagnostic)
			requireRejection(t, tc.step.Validate(), core.ErrShutdownContract, tc.wantDiagnostic)
		})
	}

	t.Run("duplicate step identity names the duplicate", func(t *testing.T) {
		t.Parallel()
		plan, err := NewPlan(PlanPolicy{TotalBudget: one})
		if err != nil {
			t.Fatalf("NewPlan() error = %v, want nil", err)
		}
		if err := plan.Register(validStep(t, 7, PhaseDrain, one)); err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}
		requireRejection(t, plan.Register(validStep(t, 7, PhaseRelease, one)),
			core.ErrShutdownContract, diagnosticStepIdentityDuplicate)
	})

	t.Run("exact capacity is admitted and one above names the ceiling", func(t *testing.T) {
		t.Parallel()
		plan, err := NewPlan(PlanPolicy{TotalBudget: one})
		if err != nil {
			t.Fatalf("NewPlan() error = %v, want nil", err)
		}
		for raw := 1; raw <= MaximumSteps; raw++ {
			if err := plan.Register(validStep(t, uint16(raw), PhaseDrain, one)); err != nil {
				t.Fatalf("Register(step %d of %d) error = %v, want nil", raw, MaximumSteps, err)
			}
		}
		requireRejection(t, plan.Register(validStep(t, MaximumSteps+1, PhaseDrain, one)),
			core.ErrShutdownContract, diagnosticPlanFull)
	})

	t.Run("registration after the single run names the closed plan", func(t *testing.T) {
		t.Parallel()
		plan, err := NewPlan(PlanPolicy{TotalBudget: one})
		if err != nil {
			t.Fatalf("NewPlan() error = %v, want nil", err)
		}
		if _, err := plan.Run(t.Context()); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		requireRejection(t, plan.Register(validStep(t, 1, PhaseDrain, one)),
			core.ErrShutdownContract, diagnosticPlanRegistrationClosed)
		_, runErr := plan.Run(t.Context())
		requireRejection(t, runErr, core.ErrShutdownContract,
			diagnosticPlanAlreadyRun)
	})

	t.Run("zero total budget names the policy", func(t *testing.T) {
		t.Parallel()
		_, err := NewPlan(PlanPolicy{TotalBudget: zero})
		requireRejection(t, err, core.ErrShutdownContract,
			diagnosticTotalBudgetInvalid)
		requireRejection(t, PlanPolicy{}.Validate(), core.ErrShutdownContract,
			diagnosticTotalBudgetInvalid)
	})
}

// TestTypedNilPlanRejectsEveryEntryPoint proves the nil receiver is a named
// rejection on all three exported entry points rather than a panic.
func TestTypedNilPlanRejectsEveryEntryPoint(t *testing.T) {
	t.Parallel()

	one := durationForTest(t, time.Nanosecond)
	var plan *Plan
	requireRejection(t, plan.Register(validStep(t, 1, PhaseDrain, one)),
		core.ErrShutdownContract, diagnosticPlanNil)
	requireRejection(t, plan.Validate(), core.ErrShutdownContract, diagnosticPlanNil)
	report, runErr := plan.Run(t.Context())
	requireRejection(t, runErr, core.ErrShutdownContract, diagnosticPlanNil)
	if err := report.Validate(); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("nil Plan.Run() report validation = %v, want %v", err, core.ErrShutdownContract)
	}
	if got, ok := report.Result(0); ok || got != (StepResult{}) {
		t.Fatalf("nil Plan.Run() Result(0) = %+v present:%t, want zero/false", got, ok)
	}
}

func TestPlanRejectsAnImpossibleRetainedCountAtEveryBoundary(t *testing.T) {
	t.Parallel()

	one := durationForTest(t, time.Nanosecond)
	plan, err := NewPlan(PlanPolicy{TotalBudget: one})
	if err != nil {
		t.Fatalf("NewPlan() error = %v, want nil", err)
	}
	plan.count = MaximumSteps + 1
	requireRejection(t, plan.Validate(), core.ErrShutdownContract, diagnosticPlanCount)
	requireRejection(t, plan.Register(validStep(t, 1, PhaseDrain, one)),
		core.ErrShutdownContract, diagnosticPlanCount)
	report, runErr := plan.Run(t.Context())
	requireRejection(t, runErr, core.ErrShutdownContract, diagnosticPlanCount)
	if got, ok := report.Result(0); ok || got != (StepResult{}) {
		t.Fatalf("Run(corrupt count) Result(0) = %+v present:%t, want zero/false", got, ok)
	}
}

// TestRunRejectsAnUnusableParentContext proves the ingress gate runs before any
// step, so a caller cannot start cleanup with no observable parent.
func TestRunRejectsAnUnusableParentContext(t *testing.T) {
	t.Parallel()

	plan, err := NewPlan(PlanPolicy{TotalBudget: durationForTest(t, time.Second)})
	if err != nil {
		t.Fatalf("NewPlan() error = %v, want nil", err)
	}
	var started bool
	step := validStep(t, 1, PhaseDrain, durationForTest(t, time.Second))
	step.Action = func(context.Context) error {
		started = true
		return nil
	}
	if err := plan.Register(step); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	//lint:ignore SA1012 the nil parent is exactly the rejected ingress under test
	report, runErr := plan.Run(nil) //nolint:staticcheck
	requireRejection(t, runErr, core.ErrShutdownContract, diagnosticParentContext)
	if started {
		t.Fatalf("Run(nil parent) step started = %t, want false", started)
	}
	if err := report.Validate(); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("Run(nil parent) report validation error = %v, want errors.Is %v", err, core.ErrShutdownContract)
	}
	if err := plan.Register(validStep(t, 2, PhaseDrain, durationForTest(t, time.Second))); err != nil {
		t.Fatalf("Register(after rejected Run) error = %v, want the plan to stay open", err)
	}
}

// TestStepResultRejectsEveryOutcomeFailureMismatch exhausts the tagged contract
// between an outcome and the error identity its failure must carry. The
// validator had no direct coverage, so an outcome could have been sealed
// alongside a failure from a different class without any test noticing.
func TestStepResultRejectsEveryOutcomeFailureMismatch(t *testing.T) {
	t.Parallel()

	identities := map[StepOutcome]error{
		StepOutcomeFailed:              core.ErrShutdownStepFailure,
		StepOutcomeTimedOut:            core.ErrShutdownStepTimeout,
		StepOutcomePanicked:            core.ErrShutdownStepPanic,
		StepOutcomeTotalBudgetExceeded: core.ErrShutdownTotalTimeout,
	}
	id := stepIDForTest(t, 1)

	for outcome := StepOutcomeCompleted; outcome < stepOutcomeLimit; outcome++ {
		identity, carries := identities[outcome]
		t.Run("outcome "+outcome.String()+" admits only its own identity", func(t *testing.T) {
			t.Parallel()
			if !carries {
				clean := StepResult{id: id, phase: PhaseDrain, outcome: outcome}
				if err := clean.Validate(); err != nil {
					t.Fatalf("completed StepResult.Validate() error = %v, want nil", err)
				}
				dirty := StepResult{id: id, phase: PhaseDrain, outcome: outcome,
					failure: core.ErrShutdownStepFailure}
				requireRejection(t, dirty.Validate(), core.ErrShutdownContract,
					diagnosticCompletedStepFailure)
				return
			}
			matching := StepResult{id: id, phase: PhaseDrain, outcome: outcome, failure: identity}
			if err := matching.Validate(); err != nil {
				t.Fatalf("StepResult{%s}.Validate() error = %v, want nil", outcome, err)
			}
			absent := StepResult{id: id, phase: PhaseDrain, outcome: outcome}
			requireRejection(t, absent.Validate(), core.ErrShutdownContract,
				diagnosticStepResultIdentityMismatch)
			for other, wrong := range identities {
				if other == outcome {
					continue
				}
				mismatched := StepResult{id: id, phase: PhaseDrain, outcome: outcome, failure: wrong}
				requireRejection(t, mismatched.Validate(), core.ErrShutdownContract,
					diagnosticStepResultIdentityMismatch)
			}
		})
	}

	structural := []struct {
		name   string
		result StepResult
	}{
		{name: "zero step result is rejected", result: StepResult{}},
		{name: "unset identity is rejected", result: StepResult{phase: PhaseDrain, outcome: StepOutcomeCompleted}},
		{name: "unset phase is rejected", result: StepResult{id: id, outcome: StepOutcomeCompleted}},
		{name: "phase at the private limit is rejected", result: StepResult{id: id, phase: phaseLimit, outcome: StepOutcomeCompleted}},
		{name: "unset outcome is rejected", result: StepResult{id: id, phase: PhaseDrain}},
		{name: "outcome at the private limit is rejected", result: StepResult{id: id, phase: PhaseDrain, outcome: stepOutcomeLimit}},
		{name: "maximum byte outcome is rejected", result: StepResult{id: id, phase: PhaseDrain, outcome: StepOutcome(^uint8(0))}},
	}
	for _, tc := range structural {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.result.Validate(); !errors.Is(err, core.ErrShutdownContract) {
				t.Fatalf("StepResult.Validate() error = %v, want %v", err, core.ErrShutdownContract)
			}
		})
	}
}

// TestReportValidateProvesItsRetainedResults is the ratchet for a hollow seal.
// Validate previously checked only its own valid flag and count, so a report
// could certify observations that failed their own contract.
func TestReportValidateProvesItsRetainedResults(t *testing.T) {
	t.Parallel()

	id := stepIDForTest(t, 1)
	sound := StepResult{id: id, phase: PhaseDrain, outcome: StepOutcomeCompleted}

	t.Run("unset report is rejected", func(t *testing.T) {
		t.Parallel()
		requireRejection(t, Report{}.Validate(), core.ErrShutdownContract, diagnosticReportUnset)
	})

	t.Run("sealed empty report is admitted", func(t *testing.T) {
		t.Parallel()
		if err := (Report{valid: true}).Validate(); err != nil {
			t.Fatalf("empty Report.Validate() error = %v, want nil", err)
		}
	})

	t.Run("count above the fixed capacity is rejected", func(t *testing.T) {
		t.Parallel()
		over := Report{valid: true, count: MaximumSteps + 1}
		requireRejection(t, over.Validate(), core.ErrShutdownContract,
			diagnosticReportCount)
		if got, ok := over.Result(0); ok || got != (StepResult{}) {
			t.Fatalf("over-capacity Report.Result(0) = %+v present:%t, want zero/false", got, ok)
		}
	})

	t.Run("exact capacity of sound results is admitted", func(t *testing.T) {
		t.Parallel()
		full := Report{valid: true, count: MaximumSteps}
		for index := range full.results {
			full.results[index] = sound
		}
		if err := full.Validate(); err != nil {
			t.Fatalf("full Report.Validate() error = %v, want nil", err)
		}
	})

	t.Run("one invalid retained result is rejected", func(t *testing.T) {
		t.Parallel()
		for _, position := range []uint8{0, 1, MaximumSteps - 1} {
			forged := Report{valid: true, count: MaximumSteps}
			for index := range forged.results {
				forged.results[index] = sound
			}
			forged.results[position] = StepResult{outcome: StepOutcomePanicked}
			requireRejection(t, forged.Validate(), core.ErrShutdownContract,
				diagnosticReportResult)
		}
	})

	t.Run("results beyond the count are not proven", func(t *testing.T) {
		t.Parallel()
		partial := Report{valid: true, count: 1}
		partial.results[0] = sound
		partial.results[1] = StepResult{outcome: StepOutcomePanicked}
		if err := partial.Validate(); err != nil {
			t.Fatalf("partial Report.Validate() error = %v, want nil", err)
		}
		if got, ok := partial.Result(1); ok || got != (StepResult{}) {
			t.Fatalf("Result(1) = %+v present:%t, want zero/false beyond the count", got, ok)
		}
	})
}

// TestStartedStepKeepsItsNativeErrorUnderEveryBudget is the ratchet for the
// defect that made a step which actually ran and failed report only a clock
// fact. Core names the total identity as work stopped or skipped by expiry, so
// a step that ran must still surface what it produced.
func TestStartedStepKeepsItsNativeErrorUnderEveryBudget(t *testing.T) {
	t.Parallel()

	t.Run("total budget expiry keeps the callback failure reachable", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			report, runErr := runOneStep(t,
				durationForTest(t, 2*time.Second), durationForTest(t, 3*time.Second),
				func(ctx context.Context) error {
					<-ctx.Done()
					return errHostileCleanup
				})
			result, ok := report.Result(0)
			if !ok || result.Outcome() != StepOutcomeTotalBudgetExceeded {
				t.Fatalf("Run() result = %+v present:%t, want total-budget-exceeded", result, ok)
			}
			if !errors.Is(result.Failure(), core.ErrShutdownTotalTimeout) ||
				!errors.Is(result.Failure(), errHostileCleanup) {
				t.Fatalf("result failure = %v, want both the total identity and %v",
					result.Failure(), errHostileCleanup)
			}
			if !errors.Is(runErr, errHostileCleanup) {
				t.Fatalf("Run() error = %v, want %v to stay reachable", runErr, errHostileCleanup)
			}
		})
	})

	t.Run("step budget expiry keeps the callback failure reachable", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			report, runErr := runOneStep(t,
				durationForTest(t, 3*time.Second), durationForTest(t, 2*time.Second),
				func(ctx context.Context) error {
					<-ctx.Done()
					return errHostileCleanup
				})
			result, ok := report.Result(0)
			if !ok || result.Outcome() != StepOutcomeTimedOut {
				t.Fatalf("Run() result = %+v present:%t, want timed-out", result, ok)
			}
			if !errors.Is(result.Failure(), core.ErrShutdownStepTimeout) ||
				!errors.Is(result.Failure(), errHostileCleanup) {
				t.Fatalf("result failure = %v, want both the step identity and %v",
					result.Failure(), errHostileCleanup)
			}
			if !errors.Is(runErr, errHostileCleanup) {
				t.Fatalf("Run() error = %v, want %v to stay reachable", runErr, errHostileCleanup)
			}
		})
	})

	t.Run("a step skipped before it starts says so and carries no callback error", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			totalBudget := durationForTest(t, 2*time.Second)
			stepBudget := durationForTest(t, 3*time.Second)
			plan, err := NewPlan(PlanPolicy{TotalBudget: totalBudget})
			if err != nil {
				t.Fatalf("NewPlan() error = %v, want nil", err)
			}
			blocking := validStep(t, 1, PhaseStopAdmission, stepBudget)
			blocking.Action = func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			}
			if err := plan.Register(blocking); err != nil {
				t.Fatalf("Register(blocking) error = %v, want nil", err)
			}
			var laterStarted bool
			later := validStep(t, 2, PhaseRelease, stepBudget)
			later.Action = func(context.Context) error {
				laterStarted = true
				return nil
			}
			if err := plan.Register(later); err != nil {
				t.Fatalf("Register(later) error = %v, want nil", err)
			}
			report, _ := plan.Run(t.Context())
			skipped, ok := report.Result(1)
			if !ok || skipped.Outcome() != StepOutcomeTotalBudgetExceeded {
				t.Fatalf("skipped result = %+v present:%t, want total-budget-exceeded", skipped, ok)
			}
			if laterStarted {
				t.Fatal("a step after total expiry started = true, want false")
			}
			var observed diagnostic
			if !errors.As(skipped.Failure(), &observed) || observed != diagnosticStepSkipped {
				t.Fatalf("skipped diagnostic = %q, want %q", observed, diagnosticStepSkipped)
			}
			if err := report.Validate(); err != nil {
				t.Fatalf("Report.Validate() error = %v, want nil", err)
			}
		})
	})
}

func runOneStep(
	t *testing.T,
	total, step temporal.Duration,
	action StepAction,
) (Report, error) {
	t.Helper()
	plan, err := NewPlan(PlanPolicy{TotalBudget: total})
	if err != nil {
		t.Fatalf("NewPlan() error = %v, want nil", err)
	}
	registered := validStep(t, 1, PhaseDrain, step)
	registered.Action = action
	if err := plan.Register(registered); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	return plan.Run(t.Context())
}

func noopAction(context.Context) error { return nil }
