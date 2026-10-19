package background_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/temporal"
)

// BenchmarkPullScopeItems measures the per-item cost of the native pull loan
// over a fixed 1024-item forward source, including scope setup and cleanup.
func BenchmarkPullScopeItems(b *testing.B) {
	const items = 1024
	source := func(ctx context.Context, yield func(int) bool) error {
		for index := range items {
			if !yield(index) {
				return nil
			}
		}
		return nil
	}
	var sum int
	use := func(ctx context.Context, pull *background.PullSource[int]) error {
		sum = 0
		for {
			value, present, err := pull.Next()
			if err != nil || !present {
				return err
			}
			sum += value
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := background.WithPullScope(b.Context(), background.PullScopeRequest[int]{Source: source, Use: use}); err != nil {
			b.Fatal(err)
		}
	}
	if sum != items*(items-1)/2 {
		b.Fatalf("consumed sum = %d, want %d", sum, items*(items-1)/2)
	}
}

// BenchmarkExecutorSubmitComplete measures one accepted callback's full
// round trip: admission, worker execution under its timeout, and completion.
func BenchmarkExecutorSubmitComplete(b *testing.B) {
	timeout, err := temporal.DurationFromSeconds(60)
	if err != nil {
		b.Fatal(err)
	}
	executor, err := background.Open(background.OpenRequest{Parent: b.Context(), Policy: background.Policy{Concurrent: 1, Waiting: 1, Timeout: timeout}})
	if err != nil {
		b.Fatal(err)
	}
	completed := make(chan error, 1)
	work := background.Work{
		Run:      func(context.Context) error { return nil },
		Complete: func(err error) { completed <- err },
	}
	b.ReportAllocs()
	finished := 0
	for b.Loop() {
		if err := executor.Submit(work); err != nil {
			b.Fatal(err)
		}
		if err := <-completed; err != nil {
			b.Fatal(err)
		}
		finished++
	}
	if err := executor.Stop(context.Background()); err != nil || finished == 0 {
		b.Fatalf("Stop() = %v after %d completions, want nil after at least one", err, finished)
	}
}
