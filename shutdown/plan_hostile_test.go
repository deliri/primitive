package shutdown

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

var errHostileCleanup = errors.New("hostile cleanup failure")

type panicOnError struct{}

func (*panicOnError) Error() string { panic("Error must not be called") }

type panicOnString struct{}

func (panicOnString) String() string { panic("String must not be called") }

// TestStepIDExhaustsItsBoundaryAndDiagnosticLabel owns the identity contract.
// The wider step, policy, and registration boundary surface is proven by
// TestRegisterNamesEveryRejectionClassDistinctly against the real plan path.
func TestStepIDExhaustsItsBoundaryAndDiagnosticLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		wantLabel string
		raw       uint16
		wantErr   bool
	}{
		{name: "zero identity is the rejected sentinel", raw: 0, wantLabel: core.UnknownEnumDiagnostic, wantErr: true},
		{name: "one is the minimum admitted identity", raw: 1, wantLabel: "1"},
		{name: "one above the minimum is admitted", raw: 2, wantLabel: "2"},
		{name: "a mid-domain identity is admitted", raw: 1 << 8, wantLabel: "256"},
		{name: "one below the maximum is admitted", raw: ^uint16(0) - 1, wantLabel: "65534"},
		{name: "the maximum identity is admitted", raw: ^uint16(0), wantLabel: "65535"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewStepID(tc.raw)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrShutdownContract) {
					t.Fatalf("NewStepID(%d) error = %v, want %v", tc.raw, gotErr, core.ErrShutdownContract)
				}
				if got != (StepID{}) {
					t.Fatalf("NewStepID(%d) = %v, want the zero identity on rejection", tc.raw, got)
				}
			} else if gotErr != nil {
				t.Fatalf("NewStepID(%d) error = %v, want nil", tc.raw, gotErr)
			}
			if (got.Validate() != nil) != tc.wantErr {
				t.Fatalf("StepID(%d).Validate() = %v, want rejected:%t", tc.raw, got.Validate(), tc.wantErr)
			}
			if label := got.String(); label != tc.wantLabel {
				t.Fatalf("StepID(%d).String() = %q, want %q", tc.raw, label, tc.wantLabel)
			}
		})
	}

	// Distinct raw values must stay distinct identities, because the plan
	// rejects duplicates by comparing whole StepID values.
	seen := make(map[StepID]uint16, len(cases))
	for _, tc := range cases {
		if tc.wantErr {
			continue
		}
		id := stepIDForTest(t, tc.raw)
		if prior, exists := seen[id]; exists {
			t.Fatalf("StepID(%d) collides with StepID(%d)", tc.raw, prior)
		}
		seen[id] = tc.raw
	}
}

func TestPlanExecutesPhasesInOrderAndLIFOWithinPhase(t *testing.T) {
	t.Parallel()

	budget := durationForTest(t, time.Second)
	plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
	if err != nil {
		t.Fatal(err)
	}
	var order []StepID
	register := func(raw uint16, phase Phase, failure error) {
		t.Helper()
		id := stepIDForTest(t, raw)
		err := plan.Register(Step{
			ID: id, Phase: phase, Budget: budget,
			Action: func(context.Context) error {
				order = append(order, id)
				return failure
			},
		})
		if err != nil {
			t.Fatalf("Register(%s) error = %v", id, err)
		}
	}
	register(1, PhaseDrain, nil)
	register(2, PhaseStopAdmission, nil)
	register(3, PhaseDrain, errHostileCleanup)
	register(4, PhaseRelease, nil)
	register(5, PhasePersist, nil)
	register(6, PhaseFlush, nil)

	report, runErr := plan.Run(t.Context())
	wantOrder := [...]StepID{
		stepIDForTest(t, 2),
		stepIDForTest(t, 3),
		stepIDForTest(t, 1),
		stepIDForTest(t, 5),
		stepIDForTest(t, 6),
		stepIDForTest(t, 4),
	}
	wantPhases := [...]Phase{
		PhaseStopAdmission, PhaseDrain, PhaseDrain,
		PhasePersist, PhaseFlush, PhaseRelease,
	}
	if len(order) != len(wantOrder) {
		t.Fatalf("execution count = %d, want %d", len(order), len(wantOrder))
	}
	for index, want := range wantOrder {
		result, ok := report.Result(uint8(index))
		if order[index] != want || !ok || result.ID() != want ||
			result.Phase() != wantPhases[index] || result.Validate() != nil {
			t.Fatalf("execution[%d] = order:%s result:%+v present:%t, want id:%s phase:%s",
				index, order[index], result, ok, want, wantPhases[index])
		}
	}
	if report.Count() != uint8(len(wantOrder)) ||
		!errors.Is(runErr, core.ErrShutdownStepFailure) ||
		!errors.Is(runErr, errHostileCleanup) {
		t.Fatalf("Run() = count:%d error:%v, want six and joined failure",
			report.Count(), runErr)
	}
	if _, ok := report.Result(report.Count()); ok {
		t.Fatal("Result(count) present = true, want false")
	}
}

func TestPlanMaximumRegistrationConcurrentAndOneShot(t *testing.T) {
	t.Parallel()

	budget := durationForTest(t, time.Second)
	plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
	if err != nil {
		t.Fatal(err)
	}
	var accepted atomic.Uint32
	var group sync.WaitGroup
	for raw := 1; raw <= MaximumSteps*2; raw++ {
		id := stepIDForTest(t, uint16(raw))
		group.Go(func() {
			err := plan.Register(Step{
				ID: id, Phase: PhaseDrain, Budget: budget,
				Action: func(context.Context) error { return nil },
			})
			if err == nil {
				accepted.Add(1)
				return
			}
			if !errors.Is(err, core.ErrShutdownContract) {
				t.Errorf("Register(%s) error = %v, want contract", id, err)
			}
		})
	}
	group.Wait()
	if got := accepted.Load(); got != MaximumSteps {
		t.Fatalf("accepted registrations = %d, want %d", got, MaximumSteps)
	}
	report, runErr := plan.Run(t.Context())
	if runErr != nil || report.Count() != MaximumSteps {
		t.Fatalf("Run(maximum) = count:%d error:%v, want %d/nil",
			report.Count(), runErr, MaximumSteps)
	}
	if _, err := plan.Run(t.Context()); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("second Run() error = %v, want contract", err)
	}
	if err := plan.Register(validStep(t, 500, PhaseDrain, budget)); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("Register(after Run) error = %v, want contract", err)
	}
}

func TestPlanDuplicateAndCallbackReentryAreRejected(t *testing.T) {
	t.Parallel()

	budget := durationForTest(t, time.Second)
	plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Register(validStep(t, 1, PhaseDrain, budget)); err != nil {
		t.Fatal(err)
	}
	if err := plan.Register(validStep(t, 1, PhaseRelease, budget)); !errors.Is(err, core.ErrShutdownContract) {
		t.Fatalf("duplicate Register() error = %v, want contract", err)
	}
	if err := plan.Register(Step{
		ID: stepIDForTest(t, 2), Phase: PhaseRelease, Budget: budget,
		Action: func(ctx context.Context) error {
			if err := plan.Register(validStep(t, 3, PhaseRelease, budget)); !errors.Is(err, core.ErrShutdownContract) {
				return err
			}
			if _, err := plan.Run(ctx); !errors.Is(err, core.ErrShutdownContract) {
				return err
			}
			return plan.Validate()
		},
	}); err != nil {
		t.Fatal(err)
	}
	report, runErr := plan.Run(t.Context())
	if runErr != nil || report.Count() != 2 {
		t.Fatalf("Run(reentrant callback) = count:%d error:%v, want 2/nil",
			report.Count(), runErr)
	}
}

func TestShutdownRunAccountingLayerTriadAccountsForStartedAndSkippedSteps(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		stepBudget := durationForTest(t, 2*time.Second)
		totalBudget := durationForTest(t, 3*time.Second)
		plan, err := NewPlan(PlanPolicy{TotalBudget: totalBudget})
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Register(validStep(t, 1, PhaseDrain, stepBudget)); err != nil {
			t.Fatal(err)
		}
		blocking := validStep(t, 2, PhaseStopAdmission, stepBudget)
		blocking.Action = func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}
		if err := plan.Register(blocking); err != nil {
			t.Fatal(err)
		}
		report, runErr := plan.Run(t.Context())
		first, firstOK := report.Result(0)
		second, secondOK := report.Result(1)
		if !firstOK || !secondOK ||
			first.Outcome() != StepOutcomeTimedOut ||
			second.Outcome() != StepOutcomeCompleted ||
			!errors.Is(runErr, core.ErrShutdownStepTimeout) {
			t.Fatalf("Run(step timeout) = first:%s second:%s error:%v",
				first.Outcome(), second.Outcome(), runErr)
		}
	})

	synctest.Test(t, func(t *testing.T) {
		totalBudget := durationForTest(t, 2*time.Second)
		stepBudget := durationForTest(t, 3*time.Second)
		plan, err := NewPlan(PlanPolicy{TotalBudget: totalBudget})
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Register(validStep(t, 1, PhaseDrain, stepBudget)); err != nil {
			t.Fatal(err)
		}
		blocking := validStep(t, 2, PhaseStopAdmission, stepBudget)
		blocking.Action = func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}
		if err := plan.Register(blocking); err != nil {
			t.Fatal(err)
		}
		report, runErr := plan.Run(t.Context())
		for index := uint8(0); index < report.Count(); index++ {
			result, ok := report.Result(index)
			if !ok || result.Outcome() != StepOutcomeTotalBudgetExceeded ||
				!errors.Is(result.Failure(), core.ErrShutdownTotalTimeout) {
				t.Fatalf("result %d = %+v present:%t, want total timeout", index, result, ok)
			}
		}
		if !errors.Is(runErr, core.ErrShutdownTotalTimeout) {
			t.Fatalf("Run(total timeout) error = %v, want total timeout", runErr)
		}
	})
}

// TestPlanContainsPanicsAndKeepsThePanicValueReachable proves containment and
// diagnostic preservation. The panic value was previously discarded outright,
// so a cleanup callback that panicked told an operator nothing but "panicked".
func TestPlanContainsPanicsAndKeepsThePanicValueReachable(t *testing.T) {
	t.Parallel()

	hostileError := &panicOnError{}
	longDiagnostic := strings.Repeat("x", panicDiagnosticMaximumRunes+1)
	panicValues := []struct {
		value     any
		wantError error
		name      string
		wantRunes int
	}{
		{name: "nil panic value", value: nil},
		{name: "string panic value", value: "hostile"},
		{name: "byte slice panic value", value: []byte("ab")},
		{name: "error panic value stays typed", value: errHostileCleanup, wantError: errHostileCleanup},
		{name: "hostile error formatter is never called", value: hostileError, wantError: hostileError},
		{name: "maximum integer panic value", value: uint64(^uint64(0))},
		{name: "structure panic value", value: struct{ Value string }{Value: "hostile"}},
		{name: "hostile stringer is never called", value: panicOnString{}},
		{name: "empty string panic value", value: ""},
		{
			name: "string diagnostic is bounded", value: longDiagnostic,
			wantRunes: panicDiagnosticMaximumRunes,
		},
		{
			name: "byte diagnostic is bounded", value: []byte(longDiagnostic),
			wantRunes: panicDiagnosticMaximumRunes,
		},
		{name: "invalid UTF-8 is normalized", value: []byte{0xff, 0xfe}},
	}
	for index, tc := range panicValues {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := durationForTest(t, time.Second)
			plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
			if err != nil {
				t.Fatal(err)
			}
			step := validStep(t, uint16(index+1), PhaseRelease, budget)
			step.Action = func(context.Context) error { panic(tc.value) }
			if err := plan.Register(step); err != nil {
				t.Fatal(err)
			}
			report, runErr := plan.Run(t.Context())
			result, ok := report.Result(0)
			if !ok || result.Outcome() != StepOutcomePanicked {
				t.Fatalf("Run(panic %T) = result:%+v present:%t, want a contained panic",
					tc.value, result, ok)
			}
			if !errors.Is(runErr, core.ErrShutdownStepPanic) ||
				!errors.Is(result.Failure(), core.ErrShutdownStepPanic) {
				t.Fatalf("Run(panic %T) carries step-panic identity = run:%t result:%t, want true/true",
					tc.value, errors.Is(runErr, core.ErrShutdownStepPanic),
					errors.Is(result.Failure(), core.ErrShutdownStepPanic))
			}
			var panicError StepPanicError
			if !errors.As(result.Failure(), &panicError) {
				t.Fatalf("Run(panic %T) exposes StepPanicError = false, want true", tc.value)
			}
			if err := panicError.Validate(); err != nil {
				t.Fatalf("StepPanicError.Validate() error = %v, want nil", err)
			}
			diagnosticRunes := utf8.RuneCountInString(panicError.Diagnostic())
			if diagnosticRunes < 1 || diagnosticRunes > panicDiagnosticMaximumRunes {
				t.Fatalf("Run(panic %T) diagnostic runes = %d, want [1,%d]",
					tc.value, diagnosticRunes, panicDiagnosticMaximumRunes)
			}
			if !utf8.ValidString(panicError.Diagnostic()) {
				t.Fatalf("Run(panic %T) diagnostic UTF-8 valid = false, want true", tc.value)
			}
			if tc.wantRunes != 0 && diagnosticRunes != tc.wantRunes {
				t.Fatalf("long panic diagnostic runes = %d, want exact bound %d",
					diagnosticRunes, tc.wantRunes)
			}
			if tc.wantError != nil && !errors.Is(result.Failure(), tc.wantError) {
				t.Fatalf("Run(panic %T) native error reachable = false, want true", tc.value)
			}
			if err := report.Validate(); err != nil {
				t.Fatalf("Report.Validate() after a contained panic error = %v, want nil", err)
			}
		})
	}
}

func TestStepPanicErrorRejectsEveryBrokenSeal(t *testing.T) {
	t.Parallel()

	overlong := strings.Repeat("x", panicDiagnosticMaximumRunes+1)
	cases := []struct {
		name  string
		value StepPanicError
	}{
		{name: "zero value", value: StepPanicError{}},
		{
			name:  "missing panic identity",
			value: StepPanicError{cause: errHostileCleanup, diagnostic: "present"},
		},
		{
			name: "empty diagnostic",
			value: StepPanicError{
				cause: core.ErrShutdownStepPanic,
			},
		},
		{
			name: "overlong diagnostic",
			value: StepPanicError{
				cause: core.ErrShutdownStepPanic, diagnostic: overlong,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireRejection(t, tc.value.Validate(), core.ErrShutdownContract,
				diagnosticPanicErrorInvalid)
		})
	}
}

func TestPlanDetachesTerminalParentAndPreservesValues(t *testing.T) {
	t.Parallel()

	type contextKey uint8
	const key contextKey = 1
	parent, cancel := context.WithCancel(context.WithValue(t.Context(), key, "kept"))
	cancel()
	budget := durationForTest(t, time.Second)
	plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
	if err != nil {
		t.Fatal(err)
	}
	step := validStep(t, 1, PhasePersist, budget)
	step.Action = func(ctx context.Context) error {
		if ctx.Value(key) != "kept" {
			return errHostileCleanup
		}
		return contextstateForTest(ctx)
	}
	if err := plan.Register(step); err != nil {
		t.Fatal(err)
	}
	report, runErr := plan.Run(parent)
	result, ok := report.Result(0)
	if !ok || result.Outcome() != StepOutcomeCompleted || runErr != nil {
		t.Fatalf("Run(terminal parent) = result:%+v present:%t error:%v",
			result, ok, runErr)
	}
}

func contextstateForTest(ctx context.Context) error {
	if ctx.Err() != nil {
		return errHostileCleanup
	}
	return nil
}

func validStep(
	t testing.TB,
	raw uint16,
	phase Phase,
	budget temporal.Duration,
) Step {
	t.Helper()
	return Step{
		ID: stepIDForTest(t, raw), Phase: phase, Budget: budget,
		Action: func(context.Context) error { return nil },
	}
}

func stepIDForTest(t testing.TB, raw uint16) StepID {
	t.Helper()
	id, err := NewStepID(raw)
	if err != nil {
		t.Fatalf("NewStepID(%d) error = %v", raw, err)
	}
	return id
}

func durationForTest(t testing.TB, value time.Duration) temporal.Duration {
	t.Helper()
	duration, err := temporal.NewDuration(value)
	if err != nil {
		t.Fatalf("temporal.NewDuration(%s) error = %v", value, err)
	}
	return duration
}

func BenchmarkPlanRunMaximumNoop(b *testing.B) {
	b.ReportAllocs()
	budget := durationForTest(b, time.Second)
	for b.Loop() {
		plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
		if err != nil {
			b.Fatal(err)
		}
		for index := range MaximumSteps {
			err := plan.Register(validStep(
				b, uint16(index+1), Phase(index%int(phaseLimit-1)+1), budget,
			))
			if err != nil {
				b.Fatal(err)
			}
		}
		report, err := plan.Run(context.Background())
		if err != nil || report.Count() != MaximumSteps {
			b.Fatalf("Run() = count:%d error:%v", report.Count(), err)
		}
	}
}
