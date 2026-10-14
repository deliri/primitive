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
	if err := p.OperationTimeout.Validate(); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	if p.OperationTimeout.IsZero() {
		return errors.Join(core.ErrExchangeContract, core.ErrTemporalContract)
	}
	if err := p.Retry.Validate(); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

// StreamAttempt must construct fresh destination custody or rewind source
// custody before executing exactly one streaming call.
type StreamAttempt func(context.Context, uint64) (StreamResponse, error)

// StreamReplayCall supplies one bounded replayable streaming operation.
type StreamReplayCall struct {
	// Semantics owns replay authority for the logical operation. Attempt must
	// use the same method and idempotency key in each fresh native call. Its
	// individual streaming call remains structurally single-attempt. The
	// replay owner refuses an unbound request and cannot amplify single-attempt
	// semantics, including a single attempt that carries an idempotency key.
	Semantics RequestSemantics
	Context   context.Context
	Attempt   StreamAttempt
	Policy    StreamReplayPolicy
}

// Validate rejects missing custody, context, or retry policy.
func (c StreamReplayCall) Validate() error {
	if err := contextstate.Validate(c.Context); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	if c.Attempt == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := c.Policy.Validate(); err != nil {
		return err
	}
	allowed, err := c.Semantics.AllowsRetry()
	if err != nil || !allowed && c.Policy.Retry.MaximumAttempts > 1 {
		return requestError(errors.Join(core.ErrExchangeContract, err))
	}
	return nil
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
		response, attemptErr = admitStreamAttemptResponse(response, attemptErr, progress.attempts)
		retry, decisionErr := replayStreamDecision(operationContext, attemptErr)
		if !retry {
			return response, decisionErr
		}
		if progress.attempts == call.Policy.Retry.MaximumAttempts {
			return response, RetryExhaustedError{cause: decisionErr, attempts: progress.attempts}
		}
		progress, err = waitForRetry(retryWaitRequest{
			context: operationContext, policy: call.Policy.Retry, progress: progress,
			retryAfter: response.Metadata.RetryAfter,
		})
		if err != nil {
			if errors.Is(err, core.ErrExchangeCancelled) {
				return response, errors.Join(err, decisionErr)
			}
			return response, RetryExhaustedError{cause: errors.Join(decisionErr, err), attempts: progress.attempts}
		}
	}
	return zero, RetryExhaustedError{cause: core.ErrExchangeTransport, attempts: progress.attempts}
}

// The callback supplies a single-attempt observation. Validate it before adding
// the replay owner's attempt count, including on a failed transfer. A refusal
// without an observation remains absent, rather than acquiring invented facts.
func admitStreamAttemptResponse(response StreamResponse, cause error, attempts uint64) (StreamResponse, error) {
	if cause != nil && absentStreamObservation(response) {
		return response, cause
	}
	if response.Metadata.Attempts != 1 {
		return StreamResponse{}, errors.Join(cause, requestError(core.ErrExchangeContract))
	}
	if err := response.Validate(); err != nil {
		return StreamResponse{}, errors.Join(cause, requestError(errors.Join(core.ErrExchangeContract, err)))
	}
	response.Metadata.Attempts = attempts
	return response, cause
}

func absentStreamObservation(response StreamResponse) bool {
	return response.Metadata.Attempts == 0 &&
		response.Metadata.Status == (core.HTTPStatusCode{}) &&
		response.Metadata.Bytes == (core.ByteLength{}) &&
		response.Metadata.RetryAfter.IsZero() &&
		response.DeclaredRequestBytes == (core.ByteLength{}) &&
		!response.RequestLengthKnown &&
		response.Metadata.Headers.Values == nil
}

var (
	_ core.Validatable = StreamReplayPolicy{}
	_ core.Validatable = StreamReplayCall{}
)

func replayStreamDecision(ctx context.Context, cause error) (bool, error) {
	if terminal := terminalOperationError(ctx); terminal != nil {
		// Cancellation stops future effects; it cannot erase the failure
		// already observed by the completed attempt.
		return false, errors.Join(terminal, cause)
	}
	if cause == nil {
		return false, nil
	}
	if terminalStreamReplayCause(cause) {
		return false, cause
	}
	status, ok := errors.AsType[StatusError](cause)
	if ok {
		return retryableStatus(status.Status()), cause
	}
	return errors.Is(cause, core.ErrExchangeTransport) ||
		errors.Is(cause, core.ErrExchangeResponse), cause
}

func terminalStreamReplayCause(cause error) bool {
	return errors.Is(cause, core.ErrExchangeCancelled) ||
		errors.Is(cause, core.ErrExchangeRequest) ||
		errors.Is(cause, core.ErrExchangeRedirect) ||
		errors.Is(cause, core.ErrExchangeBodyLimit) ||
		errors.Is(cause, core.ErrExchangeContentType) ||
		errors.Is(cause, core.ErrExchangeResponseHeaders)
}
