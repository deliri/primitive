package contextstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
)

func BenchmarkValidateLiveContext(b *testing.B) {
	ctx := context.Background()
	var wantErr error
	b.ReportAllocs()
	for b.Loop() {
		err := contextstate.Validate(ctx)
		if !errors.Is(err, wantErr) {
			b.Fatalf("contextstate.Validate() error = %v, want %v", err, wantErr)
		}
	}
}

func BenchmarkObserveLiveContext(b *testing.B) {
	ctx := context.Background()
	var wantErr error
	b.ReportAllocs()
	var last contextstate.State
	for b.Loop() {
		state, err := contextstate.Observe(ctx)
		if !errors.Is(err, wantErr) {
			b.Fatalf("contextstate.Observe() error = %v, want %v", err, wantErr)
		}
		last = state
	}
	if last != contextstate.StateNone {
		b.Fatalf("contextstate.Observe() = %v, want StateNone", last)
	}
}
