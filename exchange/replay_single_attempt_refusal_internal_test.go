package exchange

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestReplayStreamRefusesUnboundRetryBeforeCallingUpload(t *testing.T) {
	t.Parallel()
	calls := 0
	_, err := ReplayStream(StreamReplayCall{Context: t.Context(), Policy: replayStreamPolicy(t, 2), Attempt: func(context.Context, uint64) (StreamResponse, error) {
		calls++
		return StreamResponse{}, transportError(core.ErrExchangeTransport)
	}})
	if !errors.Is(err, core.ErrExchangeRequest) || calls != 0 {
		t.Fatalf("unbound replay = %v with %d callbacks, want request refusal before any possible upload", err, calls)
	}
}

func TestReplayStreamSingleAttemptAuthorityCannotAmplifyEffects(t *testing.T) {
	t.Parallel()
	key, err := ParseIdempotencyKey("owned-single-operation")
	if err != nil {
		t.Fatal(err)
	}
	for _, semantics := range []RequestSemantics{
		{Method: MethodGet, Replay: ReplaySingleAttempt},
		{Method: MethodPost, Replay: ReplaySingleAttempt},
		{Method: MethodPut, Replay: ReplaySingleAttempt},
		{Method: MethodPost, Replay: ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: key},
		{Method: MethodPatch, Replay: ReplaySingleAttemptWithIdempotencyKey, IdempotencyKey: key},
	} {
		for _, attempts := range []uint64{1, 2, 3} {
			calls := 0
			_, err := ReplayStream(StreamReplayCall{Context: t.Context(), Semantics: semantics, Policy: replayStreamPolicy(t, attempts), Attempt: func(context.Context, uint64) (StreamResponse, error) {
				calls++
				return StreamResponse{}, transportError(core.ErrExchangeTransport)
			}})
			if attempts > 1 {
				if calls != 0 || !errors.Is(err, core.ErrExchangeRequest) {
					t.Fatalf("single authority %v with %d attempts = %v/%d calls, want refusal before effects", semantics, attempts, err, calls)
				}
			} else if calls != 1 || !errors.Is(err, core.ErrExchangeTransport) {
				t.Fatalf("single authority = %v/%d calls, want exactly one native cause", err, calls)
			}
		}
	}
}
