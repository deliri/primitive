package shutdown

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzStepIDNominalIngress(f *testing.F) {
	for _, raw := range []uint16{0, 1, 2, ^uint16(0) - 1, ^uint16(0)} {
		if raw != 0 {
			_ = stepIDForTest(f, raw)
		}
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw uint16) {
		got, err := NewStepID(raw)
		if raw == 0 {
			if !errors.Is(err, core.ErrShutdownContract) || got != (StepID{}) {
				t.Fatalf("NewStepID = (%v,%v), want zero/contract", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != strconv.FormatUint(uint64(raw), 10) {
			t.Fatalf("NewStepID(%d) = (%v,%v), want exact valid identity", raw, got, err)
		}
		other, err := NewStepID(raw - 1)
		if raw > 1 && (err != nil || other == got) {
			t.Fatalf("adjacent identities = (%v,%v,%v), want distinct", got, other, err)
		}
	})
}

// Bytes are fuzz controls for typed registrations, not a shutdown wire format.
// Accepted steps are independently sorted by phase then descending admission
// coordinate. This oracle does not call runPhases or its classification helper.
func FuzzPlanRegistrationAndExecution(f *testing.F) {
	for _, count := range []int{0, 1, MaximumSteps - 1, MaximumSteps, MaximumSteps + 1} {
		seed := make([]byte, 0, count*3)
		for i := range count {
			seed = append(seed, byte(i+1), byte(Phase(i%int(phaseLimit-1)+1)), byte(i%3))
		}
		f.Add(seed)
	}
	f.Add([]byte{1, byte(PhaseDrain), 0, 1, byte(PhaseRelease), 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		data = data[:min(len(data), (MaximumSteps+1)*3)]
		budget := durationForTest(t, time.Hour)
		plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
		if err != nil {
			t.Fatalf("NewPlan = %v, want nil", err)
		}
		type admittedStep struct {
			id       StepID
			phase    Phase
			position int
			outcome  StepOutcome
		}
		admitted := make([]admittedStep, 0, MaximumSteps)
		calls := make([]StepID, 0, MaximumSteps)
		for i := 0; i+2 < len(data); i += 3 {
			id, idErr := NewStepID(uint16(data[i]))
			if (idErr != nil) != (data[i] == 0) {
				t.Fatalf("ID(%d) = %v, want zero-only rejection", data[i], idErr)
			}
			phase := Phase(data[i+1])
			mode := data[i+2] % 3
			outcome := []StepOutcome{StepOutcomeCompleted, StepOutcomeFailed, StepOutcomePanicked}[mode]
			step := Step{ID: id, Phase: phase, Budget: budget, Action: func(context.Context) error {
				calls = append(calls, id)
				if mode == 2 {
					panic(errHostileCleanup)
				}
				if mode == 1 {
					return errHostileCleanup
				}
				return nil
			}}
			duplicate := slices.ContainsFunc(admitted, func(s admittedStep) bool { return s.id == id })
			wantReject := data[i] == 0 || !slices.Contains([]Phase{PhaseStopAdmission, PhaseDrain, PhasePersist, PhaseFlush, PhaseRelease}, phase) || duplicate || len(admitted) == MaximumSteps
			err := plan.Register(step)
			if wantReject {
				if !errors.Is(err, core.ErrShutdownContract) {
					t.Fatalf("Register(%d,%d) = %v, want contract", data[i], phase, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("Register(%d,%d) = %v, want nil", data[i], phase, err)
			}
			admitted = append(admitted, admittedStep{id: id, phase: phase, position: i, outcome: outcome})
			// The registered value owns its function and coordinates after return.
			step.ID = StepID{}
			step.Action = nil
		}
		if len(calls) != 0 {
			t.Fatalf("callbacks during admission = %d, want zero", len(calls))
		}
		slices.SortFunc(admitted, func(a, b admittedStep) int {
			if a.phase != b.phase {
				return int(a.phase) - int(b.phase)
			}
			return b.position - a.position
		})
		report, runErr := plan.Run(t.Context())
		if int(report.Count()) != len(admitted) || len(calls) != len(admitted) {
			t.Fatalf("run counts = (%d,%d), want %d/%d", report.Count(), len(calls), len(admitted), len(admitted))
		}
		wantFailure, wantPanic := false, false
		for i, want := range admitted {
			got, ok := report.Result(uint8(i))
			if !ok || got.ID() != want.id || calls[i] != want.id || got.Phase() != want.phase || got.Outcome() != want.outcome || got.Validate() != nil {
				t.Fatalf("result[%d] = (%+v,%t,call=%v), want %+v", i, got, ok, calls[i], want)
			}
			wantFailure = wantFailure || want.outcome == StepOutcomeFailed
			wantPanic = wantPanic || want.outcome == StepOutcomePanicked
			if want.outcome != StepOutcomeCompleted && !errors.Is(got.Failure(), errHostileCleanup) {
				t.Fatalf("failure = %v, want retained callback error", got.Failure())
			}
		}
		if errors.Is(runErr, core.ErrShutdownStepFailure) != wantFailure || errors.Is(runErr, core.ErrShutdownStepPanic) != wantPanic || (runErr != nil) != (wantFailure || wantPanic) {
			t.Fatalf("aggregate failure = %v, want failure=%t panic=%t", runErr, wantFailure, wantPanic)
		}
		if err := report.Validate(); err != nil {
			t.Fatalf("Report.Validate = %v, want nil", err)
		}
		if got, ok := report.Result(report.Count()); ok || got != (StepResult{}) {
			t.Fatalf("extra result = (%+v,%t), want zero/false", got, ok)
		}
		second, err := plan.Run(t.Context())
		if !errors.Is(err, core.ErrShutdownContract) || second.Count() != 0 || len(calls) != len(admitted) {
			t.Fatalf("replay = (%d,%v,%d calls), want zero/contract/no new calls", second.Count(), err, len(calls))
		}
	})
}

func FuzzCallbackPanicDiagnosticIngress(f *testing.F) {
	for _, seed := range [][]byte{nil, []byte("panic"), {0xff, 0xfe}, []byte("雪"), make([]byte, panicDiagnosticMaximumRunes-1), make([]byte, panicDiagnosticMaximumRunes), make([]byte, panicDiagnosticMaximumRunes+1)} {
		f.Add(seed, false)
		f.Add(seed, true)
	}
	f.Fuzz(func(t *testing.T, data []byte, asBytes bool) {
		data = data[:min(len(data), 4096)]
		budget := durationForTest(t, time.Hour)
		report, err := runOneStep(t, budget, budget, func(context.Context) error {
			if asBytes {
				panic(data)
			}
			panic(string(data))
		})
		result, ok := report.Result(0)
		var got StepPanicError
		if !ok || result.Outcome() != StepOutcomePanicked || !errors.Is(err, core.ErrShutdownStepPanic) || !errors.As(result.Failure(), &got) {
			t.Fatalf("panic result = (%+v,%v), want contained typed panic", result, err)
		}
		runes := []rune(string(data))
		runes = runes[:min(len(runes), panicDiagnosticMaximumRunes)]
		want := string(runes)
		if len(runes) == 0 {
			want = emptyPanicDiagnostic
		}
		if got.Diagnostic() != want || !utf8.ValidString(got.Diagnostic()) || got.Validate() != nil {
			t.Fatalf("diagnostic = %q, want %q and valid", got.Diagnostic(), want)
		}
	})
}
