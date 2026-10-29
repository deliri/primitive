package gotoolchain

import (
	"context"
	"errors"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TypeMethodSetRequest borrows an actual Go type. Go owns method promotion,
// pointer receiver rules, ambiguity and generic substitution. The caller owns
// the meaning of the selected methods and the compiler type's lifetime.
type TypeMethodSetRequest struct{ Type types.Type }

func (r TypeMethodSetRequest) Validate() error {
	return validateBorrowedType(r.Type)
}
func (TypeMethodSetRequest) goToolchainInternalFlow() {}

// ObserveTypeMethodSet computes Go's exact method set without constructing a
// product registry or copying selected methods. A successful empty native set
// is neutral. Cancellation or invalid native types return no set.
func ObserveTypeMethodSet(ctx context.Context, request TypeMethodSetRequest) (*types.MethodSet, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), request.Validate()); err != nil {
		return nil, err
	}
	methods := types.NewMethodSet(request.Type)
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return nil, err
	}
	return methods, nil
}

var _ core.Validatable = TypeMethodSetRequest{}
