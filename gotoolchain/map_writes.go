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

// MapValueOrigin records only the actual Go SSA operand form. Other never
// certifies a non-nil runtime value: parameters, global loads and merged paths
// may be nil. There is no additional nilness analysis or application flow model.
type MapValueOrigin uint8

const (
	MapValueOriginUnknown MapValueOrigin = iota
	MapValueOriginNilConstant
	MapValueOriginOther
)

func (v MapValueOrigin) Validate() error {
	if v != MapValueOriginNilConstant && v != MapValueOriginOther {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (v MapValueOrigin) IsValid() bool { return v.Validate() == nil }
func (MapValueOrigin) OffWireEnum()    {}
func (v MapValueOrigin) String() string {
	switch v {
	case MapValueOriginNilConstant:
		return "nil constant"
	case MapValueOriginOther:
		return "other native operand"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// MapWriteObservation is one Go tools SSA MapUpdate, with physical source
// coordinates and the native map operand's constant form. It is not a verdict
// about all execution paths or map initialization elsewhere in the program.
type MapWriteObservation struct {
	File   core.AbsolutePath
	Origin MapValueOrigin
	Offset uint64
	Line   uint64
	Column uint64
}

func (o MapWriteObservation) Validate() error {
	if err := errors.Join(o.File.Validate(), o.Origin.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if o.Line == 0 || o.Column == 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (MapWriteObservation) goToolchainProtocolFact() {}

var _ protocolFact = MapWriteObservation{}

// MapWrites builds Go tools' native SSA for one selected compilation unit at
// a time and lends its instructions without collecting functions or findings.
// The native SSA package graph is an explicit whole-unit compiler allocation;
// this API does not claim O(1) compiler memory. Products need no shadow variable
// inventory or flow simulator. The native builder is synchronous; cancellation
// is observed before and after construction and before each instruction loan.
// Order is native CFG/block order, not a manufactured source-order transcript.
func (a PackageAnalysis) MapWrites(ctx context.Context) iter.Seq2[MapWriteObservation, error] {
	return func(yield func(MapWriteObservation, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate()); err != nil {
			yield(MapWriteObservation{}, err)
			return
		}
		if a.Incomplete {
			yield(MapWriteObservation{}, core.ErrGoToolchainOutput)
			return
		}
		for _, unit := range a.Units {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(MapWriteObservation{}, err)
				return
			}
			program := ssa.NewProgram(unit.Fset, ssa.BuilderMode(0))
			for _, imported := range unit.Types.Imports() {
				program.CreatePackage(imported, nil, nil, true)
			}
			compiled := program.CreatePackage(unit.Types, unit.Syntax, unit.TypesInfo, false)
			compiled.Build()
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(MapWriteObservation{}, err)
				return
			}
			// The synthetic initializer owns literals in package variable
			// initializers. Source init functions are separate native functions.
			for got, err := range nativeFunctionMapWrites(ctx, unit.Fset, compiled.Func("init")) {
				if !yield(got, err) || err != nil {
					return
				}
			}
			for _, file := range unit.Syntax {
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if !ok || function.Body == nil {
						continue
					}
					object, ok := unit.TypesInfo.Defs[function.Name].(*types.Func)
					if !ok {
						yield(MapWriteObservation{}, core.ErrGoToolchainOutput)
						return
					}
					body := program.FuncValue(object)
					if body == nil {
						yield(MapWriteObservation{}, core.ErrGoToolchainOutput)
						return
					}
					for got, err := range nativeFunctionMapWrites(ctx, unit.Fset, body) {
						if !yield(got, err) || err != nil {
							return
						}
					}
				}
			}
		}
	}
}

func nativeFunctionMapWrites(ctx context.Context, positions *token.FileSet, function *ssa.Function) iter.Seq2[MapWriteObservation, error] {
	return func(yield func(MapWriteObservation, error) bool) {
		if function == nil {
			yield(MapWriteObservation{}, core.ErrGoToolchainOutput)
			return
		}
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
					yield(MapWriteObservation{}, err)
					return
				}
				update, ok := instruction.(*ssa.MapUpdate)
				if !ok {
					continue
				}
				position := positions.PositionFor(update.Pos(), false)
				file, err := core.ParseAbsolutePath(position.Filename)
				if err != nil || !position.IsValid() || position.Offset < 0 {
					yield(MapWriteObservation{}, errors.Join(core.ErrGoToolchainOutput, err))
					return
				}
				origin := MapValueOriginOther
				if value, ok := update.Map.(*ssa.Const); ok && value.IsNil() {
					origin = MapValueOriginNilConstant
				}
				observation := MapWriteObservation{File: file, Origin: origin, Offset: uint64(position.Offset), Line: uint64(position.Line), Column: uint64(position.Column)}
				if !yield(observation, nil) {
					return
				}
			}
		}
		for _, anonymous := range function.AnonFuncs {
			for got, err := range nativeFunctionMapWrites(ctx, positions, anonymous) {
				if !yield(got, err) || err != nil {
					return
				}
			}
		}
	}
}
