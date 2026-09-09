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

func TestPlanRegistrationRequiresConstructedPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		constructed bool
		wantErr     error
	}{
		{name: "zero plan refuses an otherwise executable step", wantErr: core.ErrShutdownContract},
		{name: "constructed plan admits the same step", constructed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := new(Plan)
			if tc.constructed {
				var err error
				plan, err = NewPlan(PlanPolicy{TotalBudget: durationForTest(t, time.Hour)})
				if err != nil {
					t.Fatalf("NewPlan = %v, want nil", err)
				}
			}
			calls := 0
			step := validStep(t, 1, PhaseDrain, durationForTest(t, time.Hour))
			step.Action = func(context.Context) error { calls++; return nil }
			err := plan.Register(step)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Register = %v, want %v", err, tc.wantErr)
			}
			wantCount := uint8(0)
			if tc.constructed {
				wantCount = 1
			}
			if plan.count != wantCount || calls != 0 {
				t.Fatalf("registration = (%d,%d calls), want (%d,0)", plan.count, calls, wantCount)
			}
			report, err := plan.Run(t.Context())
			if !errors.Is(err, tc.wantErr) || report.Count() != wantCount || calls != int(wantCount) {
				t.Fatalf("Run = (%d,%v,%d calls), want (%d,%v,%d)", report.Count(), err, calls, wantCount, tc.wantErr, wantCount)
			}
		})
	}
}

type transitionClass uint8

const (
	transitionBoundary transitionClass = iota + 1
	transitionRefusal
	transitionNeutral
	transitionContradiction
)

// The callback producer has three exhaustive semantic exits: nil, error,
// panic. The classifier has three budget states: active, step expired, total
// expired. Their nine handoffs exhaust this finite domain; time boundary
// probes supplement it, rather than padding a fictitious 50-case space.
func TestShutdownCallbackProducerClassifierLayerTriad(t *testing.T) {
	t.Parallel()
	for _, clock := range []struct {
		name       string
		elapsed    time.Duration
		wantBudget StepOutcome
	}{
		{name: "no elapsed work", wantBudget: StepOutcomeCompleted},
		{name: "one nanosecond before step expiry", elapsed: 2*time.Second - 1, wantBudget: StepOutcomeCompleted},
		{name: "exact step expiry", elapsed: 2 * time.Second, wantBudget: StepOutcomeTimedOut},
		{name: "one nanosecond after step expiry", elapsed: 2*time.Second + 1, wantBudget: StepOutcomeTimedOut},
		{name: "one nanosecond before total expiry", elapsed: 3*time.Second - 1, wantBudget: StepOutcomeTimedOut},
		{name: "exact total expiry", elapsed: 3 * time.Second, wantBudget: StepOutcomeTotalBudgetExceeded},
		{name: "one nanosecond after total expiry", elapsed: 3*time.Second + 1, wantBudget: StepOutcomeTotalBudgetExceeded},
	} {
		for _, exit := range []struct {
			name    string
			failure error
			panics  bool
		}{
			{name: "nil callback result"},
			{name: "typed callback failure", failure: errHostileCleanup},
			{name: "contained typed panic", failure: errHostileCleanup, panics: true},
		} {
			want := clock.wantBudget
			primary := transitionBoundary
			if exit.failure != nil && want == StepOutcomeCompleted {
				want = StepOutcomeFailed
				primary = transitionRefusal
			}
			if exit.panics {
				want = StepOutcomePanicked
				if clock.wantBudget != StepOutcomeCompleted {
					primary = transitionContradiction
				}
			}
			t.Run(clock.name+"/"+exit.name, func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					var called, returned int
					var observed error
					var deadlinePresent bool
					report, runErr := runOneStep(t, durationForTest(t, 3*time.Second), durationForTest(t, 2*time.Second),
						func(ctx context.Context) error {
							called++
							_, deadlinePresent = ctx.Deadline()
							if err := temporal.Wait(temporal.WaitRequest{Context: t.Context(), Duration: durationForTest(t, clock.elapsed)}); err != nil {
								return err
							}
							// Wait for ready timer callbacks before observing their state at the
							// exact boundary. This is a Go synctest fact, not a wall-clock guess.
							synctest.Wait()
							observed = ctx.Err()
							returned++
							if exit.panics {
								panic(exit.failure)
							}
							return exit.failure
						})
					wantContext := error(nil)
					if clock.wantBudget != StepOutcomeCompleted {
						wantContext = context.DeadlineExceeded
					}
					if called != 1 || returned != 1 || !deadlinePresent || !errors.Is(observed, wantContext) {
						t.Fatalf("producer = (%d calls,%d returns,deadline=%t,%v), want (1,1,true,%v)", called, returned, deadlinePresent, observed, wantContext)
					}
					result, ok := report.Result(0)
					if !ok || report.Count() != 1 || result.ID() != stepIDForTest(t, 1) || result.Phase() != PhaseDrain || result.Outcome() != want {
						t.Fatalf("handoff class %d = (%+v,%t,count=%d), want ID1/drain/%v/count1", primary, result, ok, report.Count(), want)
					}
					for _, identity := range []struct {
						outcome StepOutcome
						err     error
					}{
						{StepOutcomeFailed, core.ErrShutdownStepFailure},
						{StepOutcomeTimedOut, core.ErrShutdownStepTimeout},
						{StepOutcomeTotalBudgetExceeded, core.ErrShutdownTotalTimeout},
						{StepOutcomePanicked, core.ErrShutdownStepPanic},
					} {
						wantMatch := want == identity.outcome
						if errors.Is(result.Failure(), identity.err) != wantMatch || errors.Is(runErr, identity.err) != wantMatch {
							t.Fatalf("class %d identity %v = result:%t run:%t, want %t/%t", primary, identity.err, errors.Is(result.Failure(), identity.err), errors.Is(runErr, identity.err), wantMatch, wantMatch)
						}
					}
					if exit.failure != nil && (!errors.Is(runErr, exit.failure) || !errors.Is(result.Failure(), exit.failure)) {
						t.Fatalf("callback failure retained = (%v,%v), want %v", runErr, result.Failure(), exit.failure)
					}
					if want == StepOutcomeCompleted && (runErr != nil || result.Failure() != nil) {
						t.Fatalf("completed failures = (%v,%v), want nil/nil", runErr, result.Failure())
					}
					if err := report.Validate(); err != nil {
						t.Fatalf("Report.Validate = %v, want nil", err)
					}
					if got, ok := report.Result(1); ok || got != (StepResult{}) {
						t.Fatalf("extra result = (%+v,%t), want zero/false", got, ok)
					}
				})
			})
		}
	}
	t.Run("neutral empty run seals zero observations", func(t *testing.T) {
		t.Parallel()
		primary := transitionNeutral
		plan, err := NewPlan(PlanPolicy{TotalBudget: durationForTest(t, time.Hour)})
		if err != nil {
			t.Fatalf("NewPlan = %v, want nil", err)
		}
		report, err := plan.Run(t.Context())
		if err != nil || report.Count() != 0 || report.Validate() != nil {
			t.Fatalf("class %d empty run = (%d,%v,%v), want zero/nil/nil", primary, report.Count(), err, report.Validate())
		}
		if got, ok := report.Result(0); ok || got != (StepResult{}) {
			t.Fatalf("empty result = (%+v,%t), want zero/false", got, ok)
		}
	})
}

func TestShutdownSkippedStepsKeepDeadlineAndEffectFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		expire      bool
		wantCalls   int
		wantOutcome StepOutcome
		wantErr     error
	}{
		{name: "active total budget executes later cleanup", wantCalls: 2, wantOutcome: StepOutcomeCompleted},
		{name: "expired total budget never executes later cleanup", expire: true, wantCalls: 1, wantOutcome: StepOutcomeTotalBudgetExceeded, wantErr: core.ErrShutdownTotalTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				budget := durationForTest(t, time.Second)
				plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
				if err != nil {
					t.Fatalf("NewPlan = %v, want nil", err)
				}
				calls := 0
				for _, phase := range []Phase{PhaseStopAdmission, PhaseRelease} {
					// Only the total deadline can fire: Go clips this longer step
					// budget to the parent. Equal independent timers would let a
					// child Done observation precede the root timer callback.
					step := validStep(t, uint16(phase), phase, durationForTest(t, 2*time.Second))
					step.Action = func(ctx context.Context) error {
						calls++
						if phase == PhaseStopAdmission && tc.expire {
							<-ctx.Done()
						}
						return nil
					}
					if err := plan.Register(step); err != nil {
						t.Fatalf("Register = %v, want nil", err)
					}
				}
				report, err := plan.Run(t.Context())
				if calls != tc.wantCalls || report.Count() != 2 || !errors.Is(err, tc.wantErr) {
					t.Fatalf("Run = (%d calls,%d results,%v), want (%d,2,%v)", calls, report.Count(), err, tc.wantCalls, tc.wantErr)
				}
				for i := range report.Count() {
					got, ok := report.Result(i)
					if !ok || got.Outcome() != tc.wantOutcome || errors.Is(got.Failure(), context.DeadlineExceeded) != tc.expire {
						t.Fatalf("Result(%d) = (%+v,%t), want outcome %v with deadline=%t", i, got, ok, tc.wantOutcome, tc.expire)
					}
				}
				if err := report.Validate(); err != nil {
					t.Fatalf("Report.Validate = %v, want nil", err)
				}
			})
		})
	}
}

func TestPlanReentryCannotPassByReturningAnUnexpectedNil(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		register bool
	}{
		{name: "Run reentry refuses before a second callback"},
		{name: "Register reentry refuses without retained work", register: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := durationForTest(t, time.Hour)
			plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
			if err != nil {
				t.Fatalf("NewPlan = %v, want nil", err)
			}
			var reentry error
			var second Report
			calls := 0
			step := validStep(t, 1, PhaseDrain, budget)
			step.Action = func(ctx context.Context) error {
				calls++
				if tc.register {
					reentry = plan.Register(validStep(t, 2, PhaseRelease, budget))
				} else {
					second, reentry = plan.Run(ctx)
				}
				return nil
			}
			if err := plan.Register(step); err != nil {
				t.Fatalf("Register = %v, want nil", err)
			}
			report, err := plan.Run(t.Context())
			if err != nil || report.Count() != 1 || calls != 1 || !errors.Is(reentry, core.ErrShutdownContract) || second.Count() != 0 {
				t.Fatalf("reentry = (%v,%d results,%d calls,%v,%d nested results), want nil/1/1/contract/0", err, report.Count(), calls, reentry, second.Count())
			}
		})
	}
}
