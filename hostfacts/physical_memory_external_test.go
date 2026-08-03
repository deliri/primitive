//go:build darwin || linux

package hostfacts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
)

type physicalMemoryContextOutcome uint8

const (
	physicalMemoryContextOutcomeSuccess physicalMemoryContextOutcome = iota + 1
	physicalMemoryContextOutcomeNil
	physicalMemoryContextOutcomeCancelled
)

func TestObservePhysicalMemoryLayerTriad(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		makeContext func() context.Context
		name        string
		wantOutcome physicalMemoryContextOutcome
	}{
		{name: "positive active context observes nonzero physical memory", makeContext: context.Background, wantOutcome: physicalMemoryContextOutcomeSuccess},
		{name: "negative nil context is refused before observation", makeContext: func() context.Context { return nil }, wantOutcome: physicalMemoryContextOutcomeNil},
		{name: "neutral cancelled context is refused before observation", makeContext: func() context.Context { return cancelled }, wantOutcome: physicalMemoryContextOutcomeCancelled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := hostfacts.ObservePhysicalMemory(tc.makeContext())
			if tc.wantOutcome != physicalMemoryContextOutcomeSuccess {
				var wantErr error = core.ErrNilContext
				if tc.wantOutcome == physicalMemoryContextOutcomeCancelled {
					wantErr = context.Canceled
				}
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("ObservePhysicalMemory() error = %v, want %v", gotErr, wantErr)
				}
				if got != (hostfacts.PhysicalMemory{}) {
					t.Fatalf("ObservePhysicalMemory() = %v on refusal, want zero value", got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ObservePhysicalMemory() error = %v, want nil", gotErr)
			}
			if got.Validate() != nil || got.TotalBytes().Uint64() == 0 {
				t.Fatalf("ObservePhysicalMemory() = %v, want valid nonzero physical-memory fact", got)
			}
		})
	}
}
