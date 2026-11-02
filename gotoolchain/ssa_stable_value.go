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

// SSAStableValueRequest selects one value borrowed from this native SSA unit.
// Constants and owned native values are accepted; no runtime value is read.
type SSAStableValueRequest struct{ Value ssa.Value }

func (r SSAStableValueRequest) Validate() error {
	if !nativeReceiverValuePresent(r.Value) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAStableValueRequest) goToolchainInternalFlow() {}

// SSAStableValueObservation lends a compiler value for the current SSA loan.
// A nil Value means the selected load has no single-store, read-only capture
// proof. A non-load lends itself. A proved load lends its stored value, with
// native representation changes removed. This is not a runtime alias proof.
type SSAStableValueObservation struct{ Value ssa.Value }

func (o SSAStableValueObservation) Validate() error {
	if o.Value != nil && !nativeReceiverValuePresent(o.Value) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSAStableValueObservation) goToolchainInternalFlow() {}

// StableValue observes Go's existing single-store cell and read-only capture
// relations for every value type. Multiple stores, writes through any child
// closure, escapes and non-dominating stores lend no proof. It follows only
// native capture cells; it constructs no alias registry or assignment history.
// Returned values remain borrowed until the owning SSAUnitView loan ends.
func (v SSAUnitView) StableValue(ctx context.Context, r SSAStableValueRequest) (out SSAStableValueObservation, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = core.ErrGoToolchainOutput
			if cause, ok := failure.(error); ok {
				err = errors.Join(err, cause)
			}
		}
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSAStableValueObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	if !v.ownsStorageValue(r.Value) {
		return out, core.ErrGoToolchainContract
	}
	if load, loaded := r.Value.(*ssa.UnOp); loaded && load.Op == token.MUL {
		if !nativeReceiverValuePresent(load.X) || !v.ownsStorageValue(load.X) {
			return out, core.ErrGoToolchainOutput
		}
		address, pointer := load.X.Type().Underlying().(*types.Pointer)
		if !pointer || !types.Identical(address.Elem(), load.Type()) {
			return out, core.ErrGoToolchainOutput
		}
	}
	value, queryErr := v.stableStoredValue(ctx, r.Value)
	if queryErr != nil {
		return out, queryErr
	}
	if load, loaded := r.Value.(*ssa.UnOp); loaded && load.Op == token.MUL && value == r.Value {
		return out, nil
	}
	out = SSAStableValueObservation{Value: value}
	return out, out.Validate()
}

var _ internalFlow = SSAStableValueRequest{}
var _ internalFlow = SSAStableValueObservation{}
