package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"iter"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSANilBoxedReturnRequest selects a native function and the interface whose
// assignable return slots are observed. The interface carries Go's type relation,
// without a product rule, error spelling or copied result population.
type SSANilBoxedReturnRequest struct {
	Function  *ssa.Function
	Interface *types.Interface
}

func (r SSANilBoxedReturnRequest) Validate() error {
	if r.Function == nil || r.Interface == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSANilBoxedReturnRequest) goToolchainInternalFlow() {}

// NilBoxedReturns lends each actual return once if a selected interface slot
// boxes a compiler-proven nil value. A nil interface, concrete nil return or
// opaque runtime value grants no boxing fact. Go owns assignability, constant
// values and representation conversions. Nested bodies are outside this stream.
func (v SSAUnitView) NilBoxedReturns(ctx context.Context, r SSANilBoxedReturnRequest) iter.Seq2[*ssa.Return, error] {
	return func(yield func(*ssa.Return, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		request := SSAInstructionRequest{Function: r.Function, Extent: SSAInstructionExtentFunction}
		for instruction, err := range v.Instructions(ctx, request) {
			if err != nil {
				yield(nil, err)
				return
			}
			returned, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			for _, value := range returned.Results {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(nil, err)
					return
				}
				if value == nil || reflect.ValueOf(value).IsNil() || value.Type() == nil {
					yield(nil, core.ErrGoToolchainOutput)
					return
				}
				if !types.IsInterface(value.Type()) || !types.AssignableTo(value.Type(), r.Interface) {
					continue
				}
				for {
					if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
						yield(nil, err)
						return
					}
					conversion, ok := value.(*ssa.ChangeInterface)
					if !ok {
						break
					}
					value = conversion.X
					if value == nil || reflect.ValueOf(value).IsNil() {
						yield(nil, core.ErrGoToolchainOutput)
						return
					}
				}
				box, ok := value.(*ssa.MakeInterface)
				if !ok {
					continue
				}
				if box.X == nil || reflect.ValueOf(box.X).IsNil() {
					yield(nil, core.ErrGoToolchainOutput)
					return
				}
				zero, ok := box.X.(*ssa.Const)
				if ok && zero.IsNil() {
					if !yield(returned, nil) {
						return
					}
					break
				}
			}
		}
	}
}

var _ internalFlow = SSANilBoxedReturnRequest{}
