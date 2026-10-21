package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// MapIterationObservation is one native map iterator's physical source
// coordinate. Go owns aliases, generic constraint intersections and lowering.
// This observation applies no policy to the map or its iteration order.
type MapIterationObservation struct {
	File   core.AbsolutePath
	Offset uint64
	Line   uint64
	Column uint64
}

func (o MapIterationObservation) Validate() error {
	if err := o.File.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if o.Line == 0 || o.Column == 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (MapIterationObservation) goToolchainProtocolFact() {}

// MapIterations owns one native SSA construction at a time. Compiler memory
// scales with the current unit. No second instruction or finding population is
// retained; the consumer completes each observation before the scan advances.
func (a PackageAnalysis) MapIterations(ctx context.Context) iter.Seq2[MapIterationObservation, error] {
	return func(yield func(MapIterationObservation, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate()); err != nil {
			yield(MapIterationObservation{}, err)
			return
		}
		if a.Incomplete {
			yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
			return
		}
		for _, unit := range a.Units {
			accepting := true
			err := a.WithSSAUnit(ctx, SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view SSAUnitView) error {
				for observation, err := range view.MapIterations(ctx) {
					if err != nil {
						return err
					}
					if !yield(observation, nil) {
						accepting = false
						return nil
					}
				}
				return nil
			}})
			if !accepting {
				return
			}
			if err != nil {
				yield(MapIterationObservation{}, err)
				return
			}
		}
	}
}

// MapIterations borrows an existing native unit. It does not rebuild SSA.
// Native block order is retained; presentation sorting belongs to the caller.
// Go emits no iterator for code removed during its native lowering.
func (v SSAUnitView) MapIterations(ctx context.Context) iter.Seq2[MapIterationObservation, error] {
	return func(yield func(MapIterationObservation, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate()); err != nil {
			yield(MapIterationObservation{}, err)
			return
		}
		if len(v.Source.Syntax) == 0 {
			yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
			return
		}
		for observation, err := range nativeFunctionMapIterations(ctx, v.Source.Fset, v.Package.Func("init")) {
			if !yield(observation, err) || err != nil {
				return
			}
		}
		for _, file := range v.Source.Syntax {
			if file == nil {
				yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
				return
			}
			for _, declaration := range file.Decls {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(MapIterationObservation{}, err)
					return
				}
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Body == nil {
					continue
				}
				object, ok := v.Source.TypesInfo.Defs[function.Name].(*types.Func)
				if !ok {
					yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
					return
				}
				for observation, err := range nativeFunctionMapIterations(ctx, v.Source.Fset, v.Program.FuncValue(object)) {
					if !yield(observation, err) || err != nil {
						return
					}
				}
			}
		}
	}
}

func nativeFunctionMapIterations(ctx context.Context, positions *token.FileSet, function *ssa.Function) iter.Seq2[MapIterationObservation, error] {
	return func(yield func(MapIterationObservation, error) bool) {
		if function == nil {
			yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
			return
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(MapIterationObservation{}, err)
					return
				}
				next, ok := instruction.(*ssa.Next)
				if !ok || next.IsString {
					continue
				}
				iterator, ok := next.Iter.(*ssa.Range)
				if !ok {
					yield(MapIterationObservation{}, core.ErrGoToolchainOutput)
					return
				}
				physical := positions.PositionFor(iterator.Pos(), false)
				file, err := core.ParseAbsolutePath(physical.Filename)
				if err != nil || !physical.IsValid() || physical.Offset < 0 {
					yield(MapIterationObservation{}, errors.Join(core.ErrGoToolchainOutput, err))
					return
				}
				observation := MapIterationObservation{File: file, Offset: uint64(physical.Offset), Line: uint64(physical.Line), Column: uint64(physical.Column)}
				if err := observation.Validate(); err != nil {
					yield(MapIterationObservation{}, err)
					return
				}
				if !yield(observation, nil) {
					return
				}
			}
		}
		for _, anonymous := range function.AnonFuncs {
			for observation, err := range nativeFunctionMapIterations(ctx, positions, anonymous) {
				if !yield(observation, err) || err != nil {
					return
				}
			}
		}
	}
}
