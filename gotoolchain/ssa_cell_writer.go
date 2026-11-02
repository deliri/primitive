package gotoolchain

import (
	"context"
	"errors"
	"go/token"
	"go/types"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSACellWriterRequest selects a dereference borrowed from the active SSA unit.
type SSACellWriterRequest struct{ Load *ssa.UnOp }

func (r SSACellWriterRequest) Validate() error {
	if r.Load == nil || r.Load.Op != token.MUL || r.Load.Type() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSACellWriterRequest) goToolchainInternalFlow() {}

// SSACellWriterObservation lends the sole synchronous call receiving a local
// cell's address before the selected load. Nil means no proof. This observes
// caller instructions, not callee behavior or runtime alias freedom. The call
// remains borrowed only for the owning SSAUnitView loan.
type SSACellWriterObservation struct{ Call *ssa.Call }

func (o SSACellWriterObservation) Validate() error {
	if o.Call != nil && (o.Call.Parent() == nil || !nativeReceiverValuePresent(o.Call.Common().Value)) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSACellWriterObservation) goToolchainInternalFlow() {}

// CellWriter observes a zero-store native local cell with one call escape.
// Native representation changes may lead to that call. Stores, captured cells,
// field addresses, competing uses and non-dominating calls lend no proof. The
// caller determines what the observed callee means; no vendor policy is owned.
func (v SSAUnitView) CellWriter(ctx context.Context, r SSACellWriterRequest) (out SSACellWriterObservation, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, core.ErrGoToolchainOutput)
			if cause, ok := failure.(error); ok {
				err = errors.Join(err, cause)
			}
		}
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSACellWriterObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	load := r.Load
	if !v.ownsStorageValue(load) {
		return out, core.ErrGoToolchainContract
	}
	if !nativeReceiverValuePresent(load.X) || !v.ownsStorageValue(load.X) {
		return out, core.ErrGoToolchainOutput
	}
	pointer, pointerType := load.X.Type().Underlying().(*types.Pointer)
	if !pointerType || !types.Identical(pointer.Elem(), load.Type()) {
		return out, core.ErrGoToolchainOutput
	}
	cell, local := load.X.(*ssa.Alloc)
	if !local || cell.Parent() != load.Parent() {
		return out, nil
	}
	call, queryErr := v.soleCellCall(ctx, cell, load)
	if queryErr != nil || call == nil {
		return out, queryErr
	}
	before, orderErr := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: call, After: load})
	if orderErr != nil || !before.Proven {
		return out, orderErr
	}
	out.Call = call
	return out, out.Validate()
}

// Follow at most one representation at each native value. No alias index,
// work queue, reference population or assignment history is retained.
func (v SSAUnitView) soleCellCall(ctx context.Context, cell *ssa.Alloc, load *ssa.UnOp) (*ssa.Call, error) {
	var writer *ssa.Call
	for value := ssa.Value(cell); value != nil; {
		references := value.Referrers()
		if references == nil {
			return nil, nil
		}
		var next ssa.Value
		for _, use := range *references {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return nil, err
			}
			if err := v.referrerInstructionMembership(use); err != nil {
				return nil, errors.Join(core.ErrGoToolchainOutput, err)
			}
			if use.Parent() != load.Parent() {
				return nil, core.ErrGoToolchainOutput
			}
			var representation ssa.Value
			switch operation := use.(type) {
			case *ssa.DebugRef:
				continue
			case *ssa.UnOp:
				if operation.Op != token.MUL || operation.X != value {
					return nil, nil
				}
				continue
			case *ssa.Call:
				if writer != nil {
					return nil, nil
				}
				bound, err := nativeCellCallArgument(operation, value)
				if err != nil || !bound {
					return nil, err
				}
				writer = operation
				continue
			case *ssa.MakeInterface:
				if operation.X != value {
					return nil, core.ErrGoToolchainOutput
				}
				representation = operation
			case *ssa.ChangeInterface:
				if operation.X != value {
					return nil, core.ErrGoToolchainOutput
				}
				representation = operation
			case *ssa.ChangeType:
				if operation.X != value {
					return nil, core.ErrGoToolchainOutput
				}
				representation = operation
			default:
				return nil, nil
			}
			if next != nil {
				return nil, nil
			}
			next = representation
			if !v.ownsStorageValue(next) {
				return nil, core.ErrGoToolchainOutput
			}
			origin, instruction := value.(ssa.Instruction)
			if !instruction {
				return nil, core.ErrGoToolchainOutput
			}
			before, err := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: origin, After: use})
			if err != nil || !before.Proven {
				return nil, err
			}
		}
		value = next
	}
	return writer, nil
}

func nativeCellCallArgument(call *ssa.Call, value ssa.Value) (bool, error) {
	if call.Common().Value == value {
		return false, nil
	}
	count := 0
	for _, argument := range call.Common().Args {
		if argument == value {
			count++
		}
	}
	if count == 0 {
		return false, core.ErrGoToolchainOutput
	}
	return count == 1, nil
}

var _ internalFlow = SSACellWriterRequest{}
var _ internalFlow = SSACellWriterObservation{}
