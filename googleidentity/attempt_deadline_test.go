package googleidentity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type googleDeadlineTransport struct {
	deadline temporal.Instant
	calls    int
}

func (t *googleDeadlineTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls++
	deadline, ok := r.Context().Deadline()
	if !ok {
		return nil, core.ErrGoogleIdentityContract
	}
	instant, err := temporal.NewInstant(deadline)
	if err != nil {
		return nil, err
	}
	t.deadline = instant
	return nil, io.ErrClosedPipe
}

// This transport observer proves the context handed to the real SDK request.
// Real local HTTP acquisition is covered separately.
func TestGoogleServiceAccountAttemptDeadlineLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		attemptSeconds uint64
		parentSeconds  uint64
		cancel         bool
		wantSeconds    uint64
		wantCalls      int
		wantErr        error
	}{
		{name: "shorter_attempt_deadline_reaches_sdk", attemptSeconds: 1, wantSeconds: 1, wantCalls: 1, wantErr: io.ErrClosedPipe},
		{name: "equal_operation_attempt", attemptSeconds: 5, wantSeconds: 5, wantCalls: 1, wantErr: io.ErrClosedPipe},
		{name: "parent_deadline_remains_stricter", attemptSeconds: 3, parentSeconds: 2, wantSeconds: 2, wantCalls: 1, wantErr: io.ErrClosedPipe},
		{name: "cancelled_request_never_reaches_sdk", attemptSeconds: 1, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			synctest.Test(t, func(t *testing.T) {
				source := serviceAccountFixtureSource(t, dir, "https://unused.invalid/token")
				transport := &googleDeadlineTransport{}
				client, err := exchange.NewClient(&http.Client{Transport: transport})
				if err != nil {
					t.Fatal(err)
				}
				source.client = client
				request := serviceAccountFixtureRequest(t)
				request.Policy.AttemptTimeout, err = temporal.DurationFromSeconds(tc.attemptSeconds)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.parentSeconds != 0 {
					duration, err := temporal.DurationFromSeconds(tc.parentSeconds)
					if err != nil {
						t.Fatal(err)
					}
					parent, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: duration})
					if err != nil {
						t.Fatal(err)
					}
					defer stop()
					ctx = parent
				}
				if tc.cancel {
					cancel()
				}
				observed, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				start, err := observed.Instant()
				if err != nil {
					t.Fatal(err)
				}
				got, err := source.Acquire(ctx, request)
				if got != (Token{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrGoogleIdentityContract) || transport.calls != tc.wantCalls {
					t.Fatalf("token=%v error=%v calls=%d, want zero, %v, %d", got, err, transport.calls, tc.wantErr, tc.wantCalls)
				}
				want := temporal.Instant{}
				if tc.wantCalls != 0 {
					duration, err := temporal.DurationFromSeconds(tc.wantSeconds)
					if err != nil {
						t.Fatal(err)
					}
					want, err = start.Add(duration)
					if err != nil {
						t.Fatal(err)
					}
				}
				if transport.deadline != want {
					t.Fatalf("SDK deadline=%v, want exact policy deadline %v", transport.deadline, want)
				}
			})
		})
	}
}
