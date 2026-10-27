package gotoolchain

import (
	"context"
	"go/token"
	"go/types"

	pc "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

func (v SSAUnitView) ownsStorageValue(value ssa.Value) bool {
	if value.Type() == nil {
		return false
	}
	if _, ok := value.(*ssa.Const); ok {
		return true
	}
	if global, ok := value.(*ssa.Global); ok {
		return global.Pkg != nil && global.Pkg.Prog == v.Program && global.Pkg.Members[global.Name()] == global
	}
	owner := value.Parent()
	if owner == nil || owner.Prog != v.Program || owner.Pkg != v.Package {
		return false
	}
	switch native := value.(type) {
	case *ssa.Parameter:
		for _, parameter := range owner.Params {
			if parameter == native {
				return true
			}
		}
	case *ssa.FreeVar:
		for _, capture := range owner.FreeVars {
			if capture == native {
				return true
			}
		}
	case ssa.Instruction:
		for _, block := range owner.Blocks {
			for _, instruction := range block.Instrs {
				if instruction == native {
					return true
				}
			}
		}
	}
	return false
}

// A source capture binds one existing native cell. The lexical parent chain
// strictly decreases; no alias registry or invented closure environment exists.
func nativeCaptureCell(address ssa.Value) *ssa.Alloc {
	for {
		if cell, ok := address.(*ssa.Alloc); ok {
			return cell
		}
		capture, ok := address.(*ssa.FreeVar)
		if !ok {
			return nil
		}
		frame := capture.Parent()
		parent := frame.Parent()
		if parent == nil {
			return nil
		}
		index := -1
		for ordinal, candidate := range frame.FreeVars {
			if candidate == capture {
				index = ordinal
				break
			}
		}
		if index < 0 {
			return nil
		}
		var binding ssa.Value
		for _, block := range parent.Blocks {
			for _, instruction := range block.Instrs {
				closure, ok := instruction.(*ssa.MakeClosure)
				if !ok || closure.Fn != frame {
					continue
				}
				if binding != nil || index >= len(closure.Bindings) {
					return nil
				}
				binding = closure.Bindings[index]
			}
		}
		if binding == nil {
			return nil
		}
		address = binding
	}
}

// Go explicitly stores the value once before every native read or capture.
// Writes through a child, an escaped cell and multiple stores remain opaque.
func (v SSAUnitView) storedValue(ctx context.Context, value ssa.Value) (ssa.Value, error) {
	load, ok := value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return value, nil
	}
	switch value.Type().Underlying().(type) {
	case *types.Pointer, *types.Signature:
	default:
		return value, nil
	}
	cell := nativeCaptureCell(load.X)
	if cell == nil || cell.Referrers() == nil {
		return value, nil
	}
	var store *ssa.Store
	for _, instruction := range *cell.Referrers() {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return nil, err
		}
		if candidate, ok := instruction.(*ssa.Store); ok && candidate.Addr == cell {
			if store != nil {
				return value, nil
			}
			store = candidate
		}
	}
	if store == nil {
		return value, nil
	}
	stored := nativeSSARepresentation(store.Val)
	if !nativeReceiverValuePresent(stored) {
		return nil, pc.ErrGoToolchainOutput
	}
	if _, loaded := stored.(*ssa.UnOp); loaded {
		return value, nil
	}
	if _, function := stored.Type().Underlying().(*types.Signature); function {
		if err := v.functionArgumentBefore(ctx, stored, store); err != nil {
			return nil, err
		}
	} else if !v.ownsStorageValue(stored) {
		return nil, pc.ErrGoToolchainContract
	}
	for _, instruction := range *cell.Referrers() {
		if instruction == store {
			continue
		}
		switch user := instruction.(type) {
		case *ssa.DebugRef:
			continue
		case *ssa.UnOp:
			if user.Op != token.MUL || user.X != cell {
				return value, nil
			}
		case *ssa.MakeClosure:
			safe, err := nativeReadOnlyCaptures(ctx, user, cell)
			if err != nil {
				return nil, err
			}
			if !safe {
				return value, nil
			}
		default:
			return value, nil
		}
		before, err := v.InstructionDominates(ctx, SSAInstructionOrderRequest{Before: store, After: instruction})
		if err != nil {
			return nil, err
		}
		if !before.Proven {
			return value, nil
		}
	}
	return stored, nil
}

func nativeReadOnlyCaptures(ctx context.Context, closure *ssa.MakeClosure, cell ssa.Value) (bool, error) {
	frame, ok := closure.Fn.(*ssa.Function)
	if !ok || frame.Parent() != cell.Parent() || len(closure.Bindings) != len(frame.FreeVars) {
		return false, nil
	}
	for index, binding := range closure.Bindings {
		if binding != cell {
			continue
		}
		safe, err := nativeReadOnlyCell(ctx, frame.FreeVars[index])
		if err != nil || !safe {
			return false, err
		}
	}
	return true, nil
}

func nativeReadOnlyCell(ctx context.Context, cell *ssa.FreeVar) (bool, error) {
	references := cell.Referrers()
	if references == nil {
		return false, nil
	}
	for _, instruction := range *references {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return false, err
		}
		switch user := instruction.(type) {
		case *ssa.DebugRef:
		case *ssa.UnOp:
			if user.Op != token.MUL || user.X != cell {
				return false, nil
			}
		case *ssa.MakeClosure:
			safe, err := nativeReadOnlyCaptures(ctx, user, cell)
			if err != nil || !safe {
				return false, err
			}
		default:
			return false, nil
		}
	}
	return true, nil
}
