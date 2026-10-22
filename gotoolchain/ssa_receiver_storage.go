package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSACallReceiverStorageRequest borrows two actual calls in one native function.
// It selects no product methods and describes no resource lifetime.
type SSACallReceiverStorageRequest struct{ Left, Right *ssa.Call }

func (r SSACallReceiverStorageRequest) Validate() error {
	if r.Left == nil || r.Right == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSACallReceiverStorageRequest) goToolchainInternalFlow() {}

// SameReceiverStorage compares native receiver operands, including the captured
// receiver of a bound method and Go's promoted-method field selection. Value
// receivers, nil pointers and independently loaded pointer fields grant no
// storage fact. Working memory follows field depth, excluding Go's compiler.
func (v SSAUnitView) SameReceiverStorage(ctx context.Context, r SSACallReceiverStorageRequest) (out SSARelationObservation, err error) {
	defer func() {
		if err == nil {
			err = temporal.Wait(temporal.WaitRequest{Context: ctx})
		}
		if err != nil {
			out = SSARelationObservation{}
		}
	}()
	if err = errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	fn := r.Left.Parent()
	if fn == nil || fn != r.Right.Parent() || fn.Prog != v.Program || fn.Pkg != v.Package {
		return out, core.ErrGoToolchainContract
	}
	for _, call := range [2]*ssa.Call{r.Left, r.Right} {
		if !v.ownsStorageValue(call) {
			return out, core.ErrGoToolchainContract
		}
		block := call.Block()
		if block.Index < 0 || block.Index >= len(fn.Blocks) || fn.Blocks[block.Index] != block {
			return out, core.ErrGoToolchainOutput
		}
		if argumentCallUntakenBlock(block) {
			return out, nil
		}
	}
	left, present, err := v.callReceiverStorage(ctx, r.Left)
	if err != nil || !present {
		return out, err
	}
	right, present, err := v.callReceiverStorage(ctx, r.Right)
	if err != nil || !present {
		return out, err
	}
	out.Proven, err = nativeProjectedReceiverStorage(ctx, left, right)
	return out, err
}

// fields borrows Go's selection path; no receiver or alias population is copied.
type ssaReceiverStorage struct {
	value  ssa.Value
	fields []int
}

func (ssaReceiverStorage) goToolchainInternalFlow() {}

func (v SSAUnitView) callReceiverStorage(ctx context.Context, call *ssa.Call) (ssaReceiverStorage, bool, error) {
	common := call.Common()
	if !nativeReceiverValuePresent(common.Value) {
		return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
	}
	if closure, bound := common.Value.(*ssa.MakeClosure); bound {
		if target, ok := closure.Fn.(*ssa.Function); !ok || target == nil {
			return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
		}
	}
	var operand ssa.Value
	var fields []int
	if common.IsInvoke() {
		operand = common.Value
	} else {
		target := common.StaticCallee()
		if target == nil {
			return ssaReceiverStorage{}, false, nil
		}
		if target.Signature == nil {
			return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
		}
		object, ok := target.Object().(*types.Func)
		if !ok || object.Signature().Recv() == nil {
			return ssaReceiverStorage{}, false, nil
		}
		receiverType := object.Signature().Recv().Type()
		if _, pointer := types.Unalias(receiverType).(*types.Pointer); !pointer && !types.IsInterface(receiverType) {
			return ssaReceiverStorage{}, false, nil
		}
		if closure, bound := common.Value.(*ssa.MakeClosure); bound {
			if closure.Fn != target || len(closure.Bindings) != 1 || len(target.FreeVars) != 1 || target.Signature.Recv() != nil {
				return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
			}
			operand = closure.Bindings[0]
			if !nativeReceiverValuePresent(operand) || target.FreeVars[0] == nil || !types.AssignableTo(operand.Type(), target.FreeVars[0].Type()) {
				return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
			}
		} else {
			if len(common.Args) == 0 {
				return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
			}
			operand = common.Args[0]
		}
		if !nativeReceiverValuePresent(operand) {
			return ssaReceiverStorage{}, false, core.ErrGoToolchainOutput
		}
		if !types.Identical(operand.Type(), receiverType) {
			selection, found := types.LookupSelection(operand.Type(), true, object.Pkg(), object.Name())
			selected, method := selection.Obj().(*types.Func)
			if !found || !method || selected.Origin() != object.Origin() {
				return ssaReceiverStorage{}, false, nil
			}
			indices := selection.Index()
			fields = indices[:len(indices)-1]
			if !nativeReceiverAddressFields(operand.Type(), fields) {
				return ssaReceiverStorage{}, false, nil
			}
		}
	}
	if !v.ownsStorageValue(operand) {
		return ssaReceiverStorage{}, false, core.ErrGoToolchainContract
	}
	value, err := v.storedPointer(ctx, nativeSSARepresentation(operand))
	if err != nil {
		return ssaReceiverStorage{}, false, err
	}
	return ssaReceiverStorage{value: value, fields: fields}, true, nil
}

func nativeReceiverValuePresent(value ssa.Value) bool {
	if value == nil {
		return false
	}
	native := reflect.ValueOf(value)
	return native.Kind() == reflect.Pointer && !native.IsNil() && value.Type() != nil
}

// Pointer or interface members require a load rather than a stable field
// address. Independent loads remain opaque under the native storage contract.
func nativeReceiverAddressFields(value types.Type, fields []int) bool {
	for _, index := range fields {
		if pointer, ok := types.Unalias(value).(*types.Pointer); ok {
			value = pointer.Elem()
		}
		shape, ok := value.Underlying().(*types.Struct)
		if !ok || index < 0 || index >= shape.NumFields() {
			return false
		}
		value = shape.Field(index).Type()
		if _, pointer := types.Unalias(value).(*types.Pointer); pointer || types.IsInterface(value) {
			return false
		}
	}
	return true
}

func nativeProjectedReceiverStorage(ctx context.Context, left, right ssaReceiverStorage) (bool, error) {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return false, err
	}
	left.value, right.value = nativeSSARepresentation(left.value), nativeSSARepresentation(right.value)
	if len(left.fields) == 0 && len(right.fields) == 0 {
		return nativeSSAStorage(left.value, right.value), nil
	}
	leftField, rightField := -1, -1
	if len(left.fields) > 0 {
		leftField = left.fields[len(left.fields)-1]
		left.fields = left.fields[:len(left.fields)-1]
	} else if address, ok := left.value.(*ssa.FieldAddr); ok {
		leftField, left.value = address.Field, address.X
	}
	if len(right.fields) > 0 {
		rightField = right.fields[len(right.fields)-1]
		right.fields = right.fields[:len(right.fields)-1]
	} else if address, ok := right.value.(*ssa.FieldAddr); ok {
		rightField, right.value = address.Field, address.X
	}
	if leftField < 0 || leftField != rightField {
		return false, nil
	}
	return nativeProjectedReceiverStorage(ctx, left, right)
}

var _ internalFlow = SSACallReceiverStorageRequest{}
var _ internalFlow = ssaReceiverStorage{}
