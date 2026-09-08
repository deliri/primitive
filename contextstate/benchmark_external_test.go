package contextstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/temporal"
)

func BenchmarkValidateContext(b *testing.B) {
	cases := []struct {
		name     string
		terminal error
	}{
		{name: "Live"},
		{name: "Cancelled", terminal: context.Canceled},
		{name: "DeadlineExceeded", terminal: context.DeadlineExceeded},
	}
	b.ReportAllocs()
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			ctx, cancel, err := benchmarkContext(tc.terminal)
			if err != nil {
				b.Fatal(err)
			}
			defer cancel()
			var got error
			b.ReportAllocs()
			for b.Loop() {
				got = contextstate.Validate(ctx)
			}
			if !errors.Is(got, tc.terminal) || got != tc.terminal {
				b.Fatalf("Validate=%v; want exact %v", got, tc.terminal)
			}
		})
	}
}

func BenchmarkObserveContext(b *testing.B) {
	cases := []struct {
		name     string
		terminal error
		want     contextstate.State
	}{
		{name: "Live", want: contextstate.StateNone},
		{name: "Cancelled", terminal: context.Canceled, want: contextstate.StateCancelled},
		{name: "DeadlineExceeded", terminal: context.DeadlineExceeded, want: contextstate.StateDeadlineExceeded},
	}
	b.ReportAllocs()
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			ctx, cancel, err := benchmarkContext(tc.terminal)
			if err != nil {
				b.Fatal(err)
			}
			defer cancel()
			var got contextstate.State
			b.ReportAllocs()
			for b.Loop() {
				got, err = contextstate.Observe(ctx)
			}
			if err != nil || got != tc.want {
				b.Fatalf("Observe=%v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func BenchmarkObserveAfterDoneContext(b *testing.B) {
	cases := []struct {
		name     string
		terminal error
		want     contextstate.State
	}{
		{name: "Cancelled", terminal: context.Canceled, want: contextstate.StateCancelled},
		{name: "DeadlineExceeded", terminal: context.DeadlineExceeded, want: contextstate.StateDeadlineExceeded},
	}
	b.ReportAllocs()
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			ctx, cancel, err := benchmarkContext(tc.terminal)
			if err != nil {
				b.Fatal(err)
			}
			defer cancel()
			<-ctx.Done()
			var got contextstate.State
			b.ReportAllocs()
			for b.Loop() {
				got, err = contextstate.ObserveAfterDone(ctx)
			}
			if err != nil || got != tc.want {
				b.Fatalf("ObserveAfterDone=%v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

// Fixture construction stays outside the measurement. Temporal delegates the
// expired deadline to Go; cancellation uses Go's real cancel context.
func benchmarkContext(terminal error) (context.Context, context.CancelFunc, error) {
	if errors.Is(terminal, context.DeadlineExceeded) {
		return temporal.WithDeadline(temporal.DeadlineRequest{Parent: context.Background(), Deadline: temporal.InstantFromNanoseconds(0)})
	}
	ctx, cancel := context.WithCancel(context.Background())
	if errors.Is(terminal, context.Canceled) {
		cancel()
	}
	return ctx, cancel, nil
}
