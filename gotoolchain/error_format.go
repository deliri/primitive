package gotoolchain

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ErrorFormatRequest probes Go's formatter with one distinct error marker in
// each native argument slot. Format is borrowed text, including malformed Go
// formats; the standard formatter owns their meaning and diagnostics.
type ErrorFormatRequest struct {
	Format        string
	ArgumentCount int
}

func (r ErrorFormatRequest) Validate() error {
	if r.ArgumentCount < 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (ErrorFormatRequest) goToolchainProtocolFact() {}

// ErrorFormatOperand is one argument selected by Go's native %w behavior under
// the marker probe. It does not claim the analyzed program's runtime argument
// implements error, nor that the program preserves a particular refusal.
type ErrorFormatOperand struct{ Index int }

func (o ErrorFormatOperand) Validate() error {
	if o.Index < 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (ErrorFormatOperand) goToolchainProtocolFact() {}

type errorFormatArgument struct{ index int }

func (errorFormatArgument) Error() string            { return "" }
func (errorFormatArgument) goToolchainInternalFlow() {}

// ErrorFormatOperands asks fmt.Errorf itself which markers it wraps. There is
// no second printf parser, directive catalog or argument-index state machine.
// Native argument materialization and formatted diagnostics allocate in
// proportion to the native request/result; this is not an O(1) formatter claim.
// The caller borrows one selected ordinal at a time and need not retain a list.
// Native formatting is synchronous; cancellation is checked around that call
// and before each argument construction and observation.
func ErrorFormatOperands(ctx context.Context, request ErrorFormatRequest) iter.Seq2[ErrorFormatOperand, error] {
	return func(yield func(ErrorFormatOperand, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), request.Validate()); err != nil {
			yield(ErrorFormatOperand{}, err)
			return
		}
		arguments := make([]any, request.ArgumentCount)
		for index := range arguments {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(ErrorFormatOperand{}, err)
				return
			}
			arguments[index] = errorFormatArgument{index: index}
		}
		native := fmt.Errorf(request.Format, arguments...)
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(ErrorFormatOperand{}, err)
			return
		}
		lend := func(wrapped error) bool {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(ErrorFormatOperand{}, err)
				return false
			}
			if wrapped == nil {
				return true
			}
			marker, ok := wrapped.(errorFormatArgument)
			if !ok || marker.index < 0 || marker.index >= request.ArgumentCount {
				yield(ErrorFormatOperand{}, core.ErrGoToolchainOutput)
				return false
			}
			return yield(ErrorFormatOperand{Index: marker.index}, nil)
		}
		switch wrapped := native.(type) {
		case interface{ Unwrap() []error }:
			for _, cause := range wrapped.Unwrap() {
				if !lend(cause) {
					return
				}
			}
		case interface{ Unwrap() error }:
			lend(wrapped.Unwrap())
		}
	}
}

var (
	_ protocolFact = ErrorFormatRequest{}
	_ protocolFact = ErrorFormatOperand{}
)
