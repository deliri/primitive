package hostfacts

import (
	"context"
	"errors"
	"math"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func resourceDuration(t *testing.T, text string) temporal.Duration {
	t.Helper()
	duration, err := temporal.ParseDuration(text)
	if err != nil {
		t.Fatal(err)
	}
	return duration
}

func TestResourceUsageDurationBoundaries(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"1ns", "250ms", "1s", "2562047h47m16.854775807s"} {
		t.Run(text, func(t *testing.T) {
			r := ResourceUsageRequest{Interval: resourceDuration(t, text)}
			if err := r.Validate(); err != nil {
				t.Fatalf("positive finite interval refused: %v", err)
			}
		})
	}
	for _, value := range []int64{-1, math.MinInt64} {
		duration, err := temporal.DurationFromNanoseconds(value)
		if err == nil || !duration.IsZero() {
			t.Fatalf("negative duration=%v/%v, want zero refusal", duration, err)
		}
		if err := (ResourceUsageRequest{Interval: duration}).Validate(); !errors.Is(err, core.ErrHostFactsContract) {
			t.Fatalf("rejected duration became a request: %v", err)
		}
	}
	if err := (ResourceUsageRequest{}).Validate(); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("zero interval=%v, want contract refusal", err)
	}
	valid := ResourceUsage{window: resourceDuration(t, "1ns"), cpus: LogicalCPUCount{count: 2}, total: byteLength(t, 2), available: byteLength(t, 1)}
	if err := valid.Validate(); err != nil || valid.Window().Nanoseconds() != 1 {
		t.Fatalf("measured window=%v/%v", valid.Window(), err)
	}
	valid.window = temporal.Duration{}
	if err := valid.Validate(); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("missing measured window=%v, want refusal", err)
	}
}

func requireResourceNativePlatform(t *testing.T) {
	t.Helper()
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	if platform.OperatingSystem != core.OperatingSystemDarwin && platform.OperatingSystem != core.OperatingSystemLinux {
		t.Skip("native CPU observation requires Darwin or Linux")
	}
}

func TestResourceUsageSubsecondNative(t *testing.T) {
	requireResourceNativePlatform(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	interval := resourceDuration(t, "250ms")
	got, err := ObserveResourceUsage(ctx, ResourceUsageRequest{Interval: interval})
	if err != nil || got.Validate() != nil {
		t.Fatalf("native subsecond sample=%+v/%v", got, err)
	}
	// This is an actual measured short-window check, not a universal latency
	// guarantee: scheduler/OS delays can legitimately make this proof fail.
	if got.Window().Nanoseconds() < interval.Nanoseconds() || got.Window().Nanoseconds() >= int64(temporal.NanosecondsPerSecond) {
		t.Fatalf("native window=%dns for requested250ms, want [250ms,1s)", got.Window().Nanoseconds())
	}
	if got.TotalMemoryBytes().Uint64() == 0 || got.AvailableMemoryBytes().Uint64() > got.TotalMemoryBytes().Uint64() || got.ProcessMemoryBytes().Uint64() == 0 || got.BusyCPUMilli() > uint64(got.CPUCount().Int())*resourceCPUMilli {
		t.Fatalf("invalid end-window native facts: %+v", got)
	}
	t.Logf("native resource window_ns=%d cpu_count=%d busy_milli=%d total=%d available=%d process=%d", got.Window().Nanoseconds(), got.CPUCount().Int(), got.BusyCPUMilli(), got.TotalMemoryBytes().Uint64(), got.AvailableMemoryBytes().Uint64(), got.ProcessMemoryBytes().Uint64())
}

func TestResourceUsageWindowCancellation(t *testing.T) {
	requireResourceNativePlatform(t)
	for _, during := range []bool{false, true} {
		name := "before"
		if during {
			name = "during"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cause := errors.New("owned resource window cancellation")
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(cause)
				if !during {
					cancel(cause)
				}
				type outcome struct {
					usage ResourceUsage
					err   error
				}
				joined := make(chan outcome, 1)
				interval := resourceDuration(t, "2s")
				go func() {
					u, err := ObserveResourceUsage(ctx, ResourceUsageRequest{Interval: interval})
					joined <- outcome{u, err}
				}()
				// Native reads must finish before the synchronous observer can
				// durably block in Temporal.Wait. The owner then cancels and joins.
				synctest.Wait()
				cancel(cause)
				got := <-joined
				if !errors.Is(got.err, cause) || !errors.Is(got.err, context.Canceled) || !emptyResourceUsage(got.usage) {
					t.Fatalf("cancel/join=%+v/%v, want original cause and no sample", got.usage, got.err)
				}
			})
		})
	}
}
