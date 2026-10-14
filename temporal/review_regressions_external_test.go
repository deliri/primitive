package temporal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestTimeLayoutRefusesLiteralMisspellings(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"YYYY-MM-DD", "yyyy/mm/dd", "literal 東京", "RFC3339", "   ", "15:04:05", "MST", "-0700", ".999"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.NewTimeLayout(text)
			if got != "" || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("NewTimeLayout(%q) = (%q, %v), want empty and temporal contract refusal", text, got, err)
			}
		})
	}
}

func TestTimeLayoutAdmitsEachNativeTokenFamily(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"2006", "06", "January", "Jan", "1", "01", "Monday", "Mon", "2", "02", "2006-01-02 15:04:05", "Mon PM", "Jan MST", "2006.999999999"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			got, err := temporal.NewTimeLayout(text)
			if got != temporal.TimeLayout(text) || err != nil {
				t.Fatalf("NewTimeLayout(%q) = (%q, %v), want preserved native token", text, got, err)
			}
		})
	}
}

func TestRFC3339RefusesFractionalInformationLoss(t *testing.T) {
	t.Parallel()
	for _, nanos := range []int64{-1, 1, 999999999, -999999999} {
		t.Run(time.Duration(nanos).String(), func(t *testing.T) {
			t.Parallel()
			got, err := temporal.InstantFromNanoseconds(nanos).RFC3339()
			if got != "" || !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("RFC3339(%d) = (%q, %v), want empty and loss refusal", nanos, got, err)
			}
		})
	}
}

func TestWaitPreservesNativeCancellationCause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	cancel(core.ErrContextObservation)
	duration, err := temporal.DurationFromSeconds(1)
	if err != nil {
		t.Fatalf("duration = %v, want nil", err)
	}
	got := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: duration})
	if !errors.Is(got, context.Canceled) || !errors.Is(got, core.ErrContextObservation) {
		t.Fatalf("Wait() = %v, want cancellation and original native cause", got)
	}
}
