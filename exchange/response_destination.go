package exchange

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
	"io"
)

// ResponseDestination supplies a fresh caller-owned sink before each HTTP
// attempt. Calls are synchronous, numbered from one, and receive the attempt
// context. The caller owns retention, admission policy, and sink cleanup.
// A retry can follow partial delivery: destinations must not publish an
// irreversible effect before the caller accepts the returned delivery.
type ResponseDestination func(context.Context, uint64) (io.Writer, error)

// ResponseDelivery records bytes acknowledged by the final attempt's sink.
// Complete means the response body reached EOF successfully before Close.
// A complete delivery may still carry a close, cancellation, or status error;
// it is not acceptance of the response or proof that a request did not commit.
type ResponseDelivery struct {
	Metadata ResponseMetadata
	Complete bool
}

// Validate checks the observed response coordinates.
func (r ResponseDelivery) Validate() error { return r.Metadata.Validate() }

// SendTo sends a retained request and streams every admitted response body,
// including unexpected-status bodies, to a fresh destination for each attempt.
// It uses the same client custody, redirects, retry and Retry-After mechanics as
// SendBounded. Primitive retains only a fixed transfer window, with no extent
// quota. Sink failures preserve their identity.
func SendTo(call BoundedCall, destination ResponseDestination) (ResponseDelivery, error) {
	if err := call.Validate(); err != nil {
		return ResponseDelivery{}, err
	}
	input, err := prepareSendBounded(call)
	if err != nil {
		return ResponseDelivery{}, err
	}
	return deliverResponse(input, destination)
}

// SendNoBodyTo is the body-absent response-streaming operation.
func SendNoBodyTo(call NoBodyBoundedCall, destination ResponseDestination) (ResponseDelivery, error) {
	if err := call.Validate(); err != nil {
		return ResponseDelivery{}, err
	}
	input, err := prepareSendNoBodyBounded(call)
	if err != nil {
		return ResponseDelivery{}, err
	}
	return deliverResponse(input, destination)
}

func deliverResponse(input aggregateCall, destination ResponseDestination) (ResponseDelivery, error) {
	if destination == nil {
		return ResponseDelivery{}, requestError(core.ErrExchangeContract)
	}
	input.destination = destination
	raw, err := executeAggregate(input)
	return ResponseDelivery{Metadata: raw.metadata, Complete: raw.complete}, err
}

func openResponseDestination(ctx context.Context, factory ResponseDestination, attempt uint64) (destination io.Writer, err error) {
	if factory == nil {
		return nil, nil
	} // Whole-value APIs own their aggregate.
	defer func() {
		if recover() != nil {
			destination, err = nil, core.ErrExchangeContract
		}
	}()
	destination, err = factory(ctx, attempt)
	if err != nil {
		return nil, err
	}
	if core.WriterIsNil(destination) {
		return nil, core.ErrExchangeContract
	}
	return destination, nil
}
