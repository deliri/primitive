package gotoolchain

import (
	"context"
	"errors"
	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"go/constant"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"reflect"
)

// SSAInstructionOrderRequest borrows two instructions from one active native
// function. It does not describe a product control-flow graph.
type SSAInstructionOrderRequest struct{ Before, After ssa.Instruction }

func (r SSAInstructionOrderRequest) Validate() error {
	if r.Before == nil || r.After == nil || reflect.ValueOf(r.Before).IsNil() || reflect.ValueOf(r.After).IsNil() {
		return pc.ErrGoToolchainContract
	}
	return nil
}
func (SSAInstructionOrderRequest) goToolchainInternalFlow() {}

// SSAStorageRequest borrows native values. Unknown or independently loaded
// pointers are not evidence that two receivers address the same storage. Native
// single-store cells with read-only captures may preserve that identity.
type SSAStorageRequest struct{ Left, Right ssa.Value }

func (r SSAStorageRequest) Validate() error {
	if r.Left == nil || r.Right == nil || reflect.ValueOf(r.Left).IsNil() || reflect.ValueOf(r.Right).IsNil() {
		return pc.ErrGoToolchainContract
	}
	return nil
}
func (SSAStorageRequest) goToolchainInternalFlow() {}

// SSARelationObservation is one compiler-grounded relation. False includes
// absence of proof; it never certifies a runtime value or resource lifetime.
type SSARelationObservation struct{ Proven bool }

func (SSARelationObservation) Validate() error          { return nil }
func (SSARelationObservation) goToolchainProtocolFact() {}

// InstructionDominates uses Go's dominator tree and native instruction order.
// The product retains neither block sets nor a private path interpreter.
func (v SSAUnitView) InstructionDominates(ctx context.Context, r SSAInstructionOrderRequest) (SSARelationObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return SSARelationObservation{}, err
	}
	before, after := r.Before, r.After
	fn := before.Parent()
	if fn == nil || fn != after.Parent() || fn.Prog != v.Program || fn.Pkg != v.Package {
		return SSARelationObservation{}, pc.ErrGoToolchainContract
	}
	beforeBlock, afterBlock := before.Block(), after.Block()
	if beforeBlock == nil || afterBlock == nil || beforeBlock.Parent() != fn || afterBlock.Parent() != fn {
		return SSARelationObservation{}, pc.ErrGoToolchainContract
	}
	ownsBefore, ownsAfter := false, false
	for _, block := range fn.Blocks {
		ownsBefore = ownsBefore || block == beforeBlock
		ownsAfter = ownsAfter || block == afterBlock
	}
	if !ownsBefore || !ownsAfter {
		return SSARelationObservation{}, pc.ErrGoToolchainContract
	}
	beforeIndex, afterIndex := -1, -1
	for index, instruction := range beforeBlock.Instrs {
		if instruction == before {
			beforeIndex = index
			break
		}
	}
	for index, instruction := range afterBlock.Instrs {
		if instruction == after {
			afterIndex = index
			break
		}
	}
	if beforeIndex < 0 || afterIndex < 0 {
		return SSARelationObservation{}, pc.ErrGoToolchainContract
	}
	proven := beforeBlock.Dominates(afterBlock)
	if beforeBlock == afterBlock {
		proven = beforeIndex <= afterIndex
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return SSARelationObservation{}, err
	}
	return SSARelationObservation{Proven: proven}, nil
}

// SameStorage reads native value identity and explicit address operands. Go
// owns field indices, representation changes and constant index values. Merges
// and independent loads stay opaque. A native single-store cell with read-only
// captures preserves its stored pointer, callable or interface value; no assignment or alias history is built.
func (v SSAUnitView) SameStorage(ctx context.Context, r SSAStorageRequest) (SSARelationObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return SSARelationObservation{}, err
	}
	for _, value := range [2]ssa.Value{r.Left, r.Right} {
		if !v.ownsStorageValue(value) {
			return SSARelationObservation{}, pc.ErrGoToolchainContract
		}
	}
	left, err := v.storedValue(ctx, nativeSSARepresentation(r.Left))
	if err != nil {
		return SSARelationObservation{}, err
	}
	right, err := v.storedValue(ctx, nativeSSARepresentation(r.Right))
	if err != nil {
		return SSARelationObservation{}, err
	}
	proven := nativeSSAStorage(left, right)
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return SSARelationObservation{}, err
	}
	return SSARelationObservation{Proven: proven}, nil
}
func nativeSSARepresentation(value ssa.Value) ssa.Value {
	for {
		switch converted := value.(type) {
		case *ssa.ChangeType:
			value = converted.X
		case *ssa.ChangeInterface:
			value = converted.X
		case *ssa.MakeInterface:
			value = converted.X
		default:
			return value
		}
	}
}
func nativeSSAStorage(left, right ssa.Value) bool {
	left, right = nativeSSARepresentation(left), nativeSSARepresentation(right)
	for _, value := range [2]ssa.Value{left, right} {
		switch value.Type().Underlying().(type) {
		case *types.Pointer, *types.Interface:
		default:
			return false
		}
	}
	if _, ok := left.(*ssa.Const); ok {
		return false
	}
	if _, ok := right.(*ssa.Const); ok {
		return false
	}
	if left == right {
		return left != nil
	}
	switch address := left.(type) {
	case *ssa.FieldAddr:
		other, ok := right.(*ssa.FieldAddr)
		return ok && address.Field == other.Field && nativeSSAStorage(address.X, other.X)
	case *ssa.IndexAddr:
		other, ok := right.(*ssa.IndexAddr)
		if !ok {
			return false
		}
		sameIndex := address.Index == other.Index
		first, firstOK := address.Index.(*ssa.Const)
		second, secondOK := other.Index.(*ssa.Const)
		if firstOK && secondOK && first.Value != nil && second.Value != nil && first.Value.Kind() == constant.Int && second.Value.Kind() == constant.Int {
			sameIndex = constant.Compare(first.Value, token.EQL, second.Value)
		}
		return sameIndex && nativeSSAStorage(address.X, other.X)
	}
	return false
}
