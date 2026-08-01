package exchange

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReplayStreamOwnsFreshAttemptRetryAndFinalMetadata(t *testing.T) {
	t.Parallel()

	attempts := uint64(0)
	response, err := ReplayStream(StreamReplayCall{
		Context: t.Context(), Policy: replayStreamPolicy(t, 3),
		Attempt: func(_ context.Context, attempt uint64) (StreamResponse, error) {
			attempts = attempt
			if attempt < 3 {
				return StreamResponse{}, responseError(errors.New("transient stream response"))
			}
			return validReplayStreamResponse(t), nil
		},
	})
	if err != nil || attempts != 3 || response.Metadata.Attempts != 3 {
		t.Fatalf("ReplayStream() = attempts %d/metadata %d/error %v, want 3/3/nil", attempts, response.Metadata.Attempts, err)
	}
}

func TestReplayStreamDoesNotReplayRequestContractFailure(t *testing.T) {
	t.Parallel()

	attempts := 0
	_, err := ReplayStream(StreamReplayCall{
		Context: t.Context(), Policy: replayStreamPolicy(t, 3),
		Attempt: func(context.Context, uint64) (StreamResponse, error) {
			attempts++
			return StreamResponse{}, requestError(core.ErrExchangeContract)
		},
	})
	if attempts != 1 || !errors.Is(err, core.ErrExchangeRequest) {
		t.Fatalf("ReplayStream(request failure) = attempts %d/error %v, want 1 and %v", attempts, err, core.ErrExchangeRequest)
	}
}

func TestReplayStreamExhaustionPreservesTypedAttemptCount(t *testing.T) {
	t.Parallel()

	_, err := ReplayStream(StreamReplayCall{
		Context: t.Context(), Policy: replayStreamPolicy(t, 2),
		Attempt: func(context.Context, uint64) (StreamResponse, error) {
			return StreamResponse{}, transportError(errors.New("transient transport"))
		},
	})
	exhausted, ok := errors.AsType[RetryExhaustedError](err)
	if !ok || exhausted.Attempts() != 2 || !errors.Is(err, core.ErrExchangeRetryExhausted) || !errors.Is(err, core.ErrExchangeTransport) {
		t.Fatalf("ReplayStream(exhausted) error = %v, want typed two-attempt transport exhaustion", err)
	}
}

func TestReplayStreamRejectsMissingFreshCustodyFunction(t *testing.T) {
	t.Parallel()

	_, err := ReplayStream(StreamReplayCall{Context: t.Context(), Policy: replayStreamPolicy(t, 1)})
	if !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("ReplayStream(nil attempt) error = %v, want %v", err, core.ErrExchangeContract)
	}
}

func replayStreamPolicy(t testing.TB, attempts uint64) StreamReplayPolicy {
	t.Helper()
	zero := replayDuration(t, 0)
	if attempts == 1 {
		return StreamReplayPolicy{
			OperationTimeout: replayDuration(t, int64(temporal.NanosecondsPerSecond)),
			Retry:            RetryPolicy{MaximumAttempts: 1, BaseDelay: zero, MaximumDelay: zero, MaximumJitter: zero, MaximumRetryAfter: zero, MaximumWait: zero},
		}
	}
	oneMillisecond := replayDuration(t, int64(temporal.NanosecondsPerMillisecond))
	return StreamReplayPolicy{
		OperationTimeout: replayDuration(t, int64(temporal.NanosecondsPerSecond)),
		Retry: RetryPolicy{
			BaseDelay: oneMillisecond, MaximumDelay: oneMillisecond,
			MaximumJitter: oneMillisecond, MaximumRetryAfter: oneMillisecond,
			MaximumWait: replayDuration(t, 20*int64(temporal.NanosecondsPerMillisecond)), MaximumAttempts: attempts,
		},
	}
}

func replayDuration(t testing.TB, nanoseconds int64) temporal.Duration {
	t.Helper()
	duration, err := temporal.DurationFromNanoseconds(nanoseconds)
	if err != nil {
		t.Fatal(err)
	}
	return duration
}

func validReplayStreamResponse(t testing.TB) StreamResponse {
	t.Helper()
	status, err := core.NewHTTPStatusCode(200)
	if err != nil {
		t.Fatal(err)
	}
	return StreamResponse{Metadata: ResponseMetadata{Status: status, Attempts: 1}}
}
