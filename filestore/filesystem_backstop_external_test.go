package filestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/temporal"
)

// Setup only: assertions about completion and effects stay at each call site.
// Duration units remain Go's; Temporal owns the actual clock effect.
func newFilesystemBackstop(parent context.Context, t testing.TB, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	duration, err := temporal.NewDuration(timeout)
	if err != nil {
		t.Fatalf("backstop duration = %v, want admitted Go duration", err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: parent, Duration: duration})
	if err != nil {
		t.Fatalf("backstop construction = %v, want usable Temporal context", err)
	}
	return ctx, cancel
}

// Setup only: a deadline at Unix second one is already terminal, so no timer
// or scheduler race decides the outcome. Cause is reported by context.Cause.
func newExpiredDeadline(parent context.Context, t testing.TB, cause error) (context.Context, context.CancelFunc) {
	t.Helper()
	expired, err := temporal.InstantFromUnixSeconds(1)
	if err != nil {
		t.Fatalf("expired deadline instant = %v, want admitted Unix second", err)
	}
	ctx, cancel, err := temporal.WithDeadline(temporal.DeadlineRequest{Parent: parent, Cause: cause, Deadline: expired})
	if err != nil {
		t.Fatalf("expired deadline construction = %v, want usable Temporal context", err)
	}
	return ctx, cancel
}
