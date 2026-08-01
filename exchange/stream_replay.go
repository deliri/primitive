package exchange

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// StreamReplayPolicy owns the total deadline and retry schedule around fresh
// single-attempt Upload or Download calls.
type StreamReplayPolicy struct {
	OperationTimeout temporal.Duration
	Retry            RetryPolicy
}

// Validate closes the total budget and retry schedule.
func (p StreamReplayPolicy) Validate() error {
	if err := p.OperationTimeout.Validate(); err != nil || p.OperationTimeout.IsZero() {
		return core.ErrExchangeContract
	}
	return p.Retry.Validate()
}

// StreamAttempt must construct fresh destination custody or rewind source
// custody before executing exactly one streaming call.
type StreamAttempt func(context.Context, uint64) (StreamResponse, error)

// StreamReplayCall supplies one bounded replayable streaming operation.
type StreamReplayCall struct {
	Context context.Context
	Attempt StreamAttempt
	Policy  StreamReplayPolicy
}

// Validate rejects missing custody, context, or retry policy.
func (c StreamReplayCall) Validate() error {
	if err := contextstate.Validate(c.Context); err != nil {
		return err
	}
	if c.Attempt == nil {
		return core.ErrExchangeContract
	}
	return c.Policy.Validate()
}

// ReplayStream owns retry classification, exponential backoff, jitter, and
// the total deadline. The caller-owned attempt owns only reopening or
// rewinding its stream and one Upload or Download call.
func ReplayStream(call StreamReplayCall) (StreamResponse, error) {
	var zero StreamResponse
	if err := call.Validate(); err != nil {
		return zero, err
	}
	operationContext, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{
		Parent: call.Context, Duration: call.Policy.OperationTimeout,
	})
	if err != nil {
		return zero, requestError(err)
	}
	defer cancel()
	progress := retryProgress{}
	for progress.attempts < call.Policy.Retry.MaximumAttempts {
		progress.attempts++
		response, attemptErr := call.Attempt(operationContext, progress.attempts)
		if attemptErr == nil {
			response.Metadata.Attempts = progress.attempts
			attemptErr = response.Validate()
		}
		retry, decisionErr := replayStreamDecision(operationContext, attemptErr)
		if !retry {
			return response, decisionErr
		}
		if progress.attempts == call.Policy.Retry.MaximumAttempts {
			return response, RetryExhaustedError{cause: decisionErr, attempts: progress.attempts}
		}
		progress, err = waitForRetry(retryWaitRequest{
			context: operationContext, policy: call.Policy.Retry, progress: progress,
		})
		if err != nil {
			return response, RetryExhaustedError{cause: err, attempts: progress.attempts}
		}
	}
	return zero, RetryExhaustedError{cause: core.ErrExchangeTransport, attempts: progress.attempts}
}

var (
	_ core.Validatable = StreamReplayPolicy{}
	_ core.Validatable = StreamReplayCall{}
)

func replayStreamDecision(ctx context.Context, cause error) (bool, error) {
	if terminal := terminalOperationError(ctx); terminal != nil {
		return false, terminal
	}
	if cause == nil {
		return false, nil
	}
	status, ok := errors.AsType[StatusError](cause)
	if ok {
		return retryableStatus(status.Status()), cause
	}
	if errors.Is(cause, core.ErrExchangeCancelled) ||
		errors.Is(cause, core.ErrExchangeRequest) ||
		errors.Is(cause, core.ErrExchangeRedirect) ||
		errors.Is(cause, core.ErrExchangeBodyLimit) ||
		errors.Is(cause, core.ErrExchangeContentType) {
		return false, cause
	}
	return errors.Is(cause, core.ErrExchangeTransport) ||
		errors.Is(cause, core.ErrExchangeResponse), cause
}
