package hostfacts_test

import (
	"context"
	"errors"
	"math"
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestGoMemoryLimitAppliesTypedIntentAndPreservesPreviousRuntimeLimit(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	// The runtime adapter is the subject: use its native API as an independent oracle.
	previous := debug.SetMemoryLimit(-1)
	defer debug.SetMemoryLimit(previous)
	for _, value := range []uint64{1 << 30, 2 << 30, math.MaxInt64} {
		limit, err := core.NewByteCount(value)
		if err != nil {
			t.Fatal(err)
		}
		before := debug.SetMemoryLimit(-1)
		got, err := hostfacts.ApplyGoMemoryLimit(t.Context(), hostfacts.GoMemoryLimitRequest{Limit: limit})
		if err != nil || got.Validate() != nil {
			t.Fatalf("ApplyGoMemoryLimit() = (%v, %v), want validated native observation", got, err)
		}
		old, oldErr := got.Previous.Uint64()
		applied, appliedErr := got.Applied.Uint64()
		actual := debug.SetMemoryLimit(-1)
		if oldErr != nil || appliedErr != nil || old != uint64(before) || applied != value || actual != int64(value) {
			t.Fatalf("native limit = previous:%d applied:%d actual:%d errors:%v/%v, want %d/%d/%d", old, applied, actual, oldErr, appliedErr, before, value, value)
		}
	}
}

func TestGoMemoryLimitRefusesInvalidIntentAndTerminalContextBeforeMutation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	limit, err := core.NewByteCount(1 << 30)
	if err != nil {
		t.Fatal(err)
	}
	overflow, err := core.NewByteCount(uint64(math.MaxInt64) + 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	cancel(context.Canceled)
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		limit core.ByteCount
		want  error
	}{
		{name: "absent context", ctx: nil, limit: limit, want: core.ErrNilContext},
		{name: "cancelled context", ctx: ctx, limit: limit, want: context.Canceled},
		{name: "zero limit", ctx: t.Context(), want: core.ErrHostFactsContract},
		{name: "unsigned limit exceeds runtime range", ctx: t.Context(), limit: overflow, want: core.ErrHostFactsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := debug.SetMemoryLimit(-1)
			got, err := hostfacts.ApplyGoMemoryLimit(tc.ctx, hostfacts.GoMemoryLimitRequest{Limit: tc.limit})
			if !errors.Is(err, tc.want) || got.Validate() == nil || got != (hostfacts.GoMemoryLimitResult{}) || debug.SetMemoryLimit(-1) != before {
				t.Fatalf("refused memory intent = (%v, %v), runtime:%d, want zero observation, %v and preserved %d", got, err, debug.SetMemoryLimit(-1), tc.want, before)
			}
		})
	}
}
