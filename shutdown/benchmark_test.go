package shutdown

import (
	"context"
	"testing"
	"time"
)

// BenchmarkPlanLifecycle includes construction, registration, callbacks and
// sealed accounting. Immutable step fixtures are prepared outside timing.
func BenchmarkPlanLifecycle(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		count int
		fails bool
	}{
		{name: "empty"}, {name: "one", count: 1},
		{name: "maximum", count: MaximumSteps},
		{name: "maximum-failures", count: MaximumSteps, fails: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			budget := durationForTest(b, time.Hour)
			var calls uint64
			action := StepAction(func(context.Context) error { calls++; return nil })
			if tc.fails {
				action = func(context.Context) error { calls++; return errHostileCleanup }
			}
			var steps [MaximumSteps]Step
			for i := range tc.count {
				steps[i] = validStep(b, uint16(i+1), Phase(i%int(phaseLimit-1)+1), budget)
				steps[i].Action = action
				if err := steps[i].Validate(); err != nil {
					b.Fatalf("fixture validation = %v, want nil", err)
				}
			}
			var got Report
			for b.Loop() {
				plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
				if err != nil {
					b.Fatalf("NewPlan = %v, want nil", err)
				}
				for _, step := range steps[:tc.count] {
					if err := plan.Register(step); err != nil {
						b.Fatalf("Register = %v, want nil", err)
					}
				}
				got, err = plan.Run(context.Background())
				if (err != nil) != tc.fails || int(got.Count()) != tc.count {
					b.Fatalf("Run = (%d,%v), want (%d,failure=%t)", got.Count(), err, tc.count, tc.fails)
				}
			}
			if want := uint64(b.N) * uint64(tc.count); calls != want {
				b.Fatalf("callbacks = %d, want %d", calls, want)
			}
			if err := got.Validate(); err != nil {
				b.Fatalf("report validation = %v, want nil", err)
			}
		})
	}
}

func BenchmarkReportWalkMaximum(b *testing.B) {
	b.ReportAllocs()
	budget := durationForTest(b, time.Hour)
	plan, err := NewPlan(PlanPolicy{TotalBudget: budget})
	if err != nil {
		b.Fatalf("NewPlan = %v, want nil", err)
	}
	for i := range MaximumSteps {
		if err := plan.Register(validStep(b, uint16(i+1), PhaseDrain, budget)); err != nil {
			b.Fatalf("Register = %v, want nil", err)
		}
	}
	report, err := plan.Run(context.Background())
	if err != nil || report.Count() != MaximumSteps {
		b.Fatalf("fixture = (%d,%v), want maximum/nil", report.Count(), err)
	}
	var sum uint64
	for b.Loop() {
		for i := range report.Count() {
			result, ok := report.Result(i)
			if !ok {
				b.Fatalf("Result(%d) present = %t, want true", i, ok)
			}
			sum += uint64(result.id.value)
		}
	}
	if want := uint64(b.N) * MaximumSteps * (MaximumSteps + 1) / 2; sum != want {
		b.Fatalf("identity sum = %d, want %d", sum, want)
	}
}

// BenchmarkWatchClose measures the actual os/signal subscription and its join.
func BenchmarkWatchClose(b *testing.B) {
	b.ReportAllocs()
	request := WatchRequest{Parent: context.Background(), Policy: defaultSignalPolicy(), Set: SignalSetStandard}
	if err := request.Validate(); err != nil {
		b.Fatalf("fixture validation = %v, want nil", err)
	}
	for b.Loop() {
		controller, err := Watch(request)
		if err != nil {
			b.Fatalf("Watch = %v, want nil", err)
		}
		if err := controller.Close(); err != nil {
			b.Fatalf("Close = %v, want nil", err)
		}
		select {
		case <-controller.Done():
		default:
			b.Fatal("controller joined = false, want true")
		}
	}
}
