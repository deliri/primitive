package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// PipeTransferRequest lends the two directions of one native pipe to callers.
// The producer must return when cancellation or native pipe closure stops its
// work. The consumer runs synchronously; neither callback owns an endpoint.
// The OS supplies bounded backpressure, without a Go buffering inventory.
type PipeTransferRequest struct {
	Produce func(context.Context, io.Writer) error
	Consume func(context.Context, io.Reader) error
}

func (r PipeTransferRequest) Validate() error {
	if r.Produce == nil || r.Consume == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// PipeTransferResult records callback and cleanup outcomes after the producer
// has joined. Joining is mechanical lifetime completion, not product success.
// Its zero value is invalid: it proves no acquired or joined native lifetime.
type PipeTransferResult struct {
	ProducerError error
	ConsumerError error
	CleanupError  error
	joined        bool
}

func (r PipeTransferResult) Validate() error {
	if !r.joined {
		return core.ErrFilestoreContract
	}
	return nil
}

type pipeProducerResult struct {
	callbackError error
	closeError    error
}

// TransferPipe owns both native endpoints, one producer, cancellation and join.
// Acquisition failures return an invalid zero observation and an error. Once
// acquired, callback failures remain separate typed causes in the observation;
// a caller must inspect them before deciding what its operation means.
func TransferPipe(ctx context.Context, request PipeTransferRequest) (result PipeTransferResult, resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return PipeTransferResult{}, err
	}
	if err := request.Validate(); err != nil {
		return PipeTransferResult{}, err
	}
	lifetime, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	pipe, err := OpenPipe(lifetime)
	if err != nil {
		return PipeTransferResult{}, err
	}
	done := make(chan pipeProducerResult, 1)
	go func() {
		done <- pipeProducerResult{callbackError: producePipe(lifetime, request.Produce, pipe.Writer), closeError: pipe.Writer.Close()}
	}()
	defer func() {
		cancel(result.ConsumerError)
		result.CleanupError = pipe.Reader.Close()
		produced := <-done
		result.ProducerError = produced.callbackError
		result.CleanupError = errors.Join(result.CleanupError, produced.closeError)
		result.joined = true
		resultErr = result.Validate()
	}()
	result.ConsumerError = consumePipe(lifetime, request.Consume, pipe.Reader)
	return result, nil
}

func producePipe(ctx context.Context, produce func(context.Context, io.Writer) error, destination io.Writer) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = fmt.Errorf("pipe producer callback panicked: %w", core.ErrFilestoreContract)
		}
	}()
	return produce(ctx, destination)
}

func consumePipe(ctx context.Context, consume func(context.Context, io.Reader) error, source io.Reader) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = fmt.Errorf("pipe consumer callback panicked: %w", core.ErrFilestoreContract)
		}
	}()
	return consume(ctx, source)
}

var _ core.Validatable = PipeTransferRequest{}
var _ core.Validatable = PipeTransferResult{}
