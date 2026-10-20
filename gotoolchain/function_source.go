package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type FunctionSourcePresence uint8

const (
	FunctionSourceUnknown FunctionSourcePresence = iota
	FunctionSourceAbsent
	FunctionSourcePresent
)

func (p FunctionSourcePresence) Validate() error {
	if p != FunctionSourceAbsent && p != FunctionSourcePresent {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (p FunctionSourcePresence) IsValid() bool { return p.Validate() == nil }
func (FunctionSourcePresence) OffWireEnum()    {}
func (p FunctionSourcePresence) String() string {
	switch p {
	case FunctionSourceAbsent:
		return "absent"
	case FunctionSourcePresent:
		return "present"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// FunctionSourceRequest borrows an actual Go function object. Instantiations
// resolve to Go's Origin declaration; a same-spelled foreign function cannot
// borrow source from the selected analysis.
type FunctionSourceRequest struct{ Function *types.Func }

func (r FunctionSourceRequest) Validate() error {
	if r.Function == nil || r.Function.Pkg() == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (FunctionSourceRequest) goToolchainProtocolFact() {}

// FunctionSourceObservation is a loan from the existing native compiler graph.
// Present can include an assembly declaration whose Body is nil. Absent means
// the function belongs to another package, not that its behavior is harmless.
type FunctionSourceObservation struct {
	Declaration *ast.FuncDecl
	Types       *types.Info
	Positions   *token.FileSet
	Presence    FunctionSourcePresence
}

func (o FunctionSourceObservation) Validate() error {
	if err := o.Presence.Validate(); err != nil {
		return err
	}
	if o.Presence == FunctionSourceAbsent {
		if o.Declaration != nil || o.Types != nil || o.Positions != nil {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	if o.Declaration == nil || o.Declaration.Name == nil || o.Types == nil || o.Positions == nil {
		return core.ErrGoToolchainContract
	}
	function, ok := o.Types.Defs[o.Declaration.Name].(*types.Func)
	if !ok || function.Pos() != o.Declaration.Name.Pos() {
		return core.ErrGoToolchainContract
	}
	position := o.Positions.PositionFor(function.Pos(), false)
	if !position.IsValid() {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (FunctionSourceObservation) goToolchainProtocolFact() {}

// FunctionSource uses native package ownership and token-file membership, then
// Go's binary search over source-ordered declarations. It retains no function
// index and does not scan every body for each lookup. Memory excludes the
// already-owned compiler graph and remains constant for each lookup.
func (a PackageAnalysis) FunctionSource(ctx context.Context, request FunctionSourceRequest) (FunctionSourceObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate(), request.Validate()); err != nil {
		return FunctionSourceObservation{}, err
	}
	if a.Incomplete {
		return FunctionSourceObservation{}, core.ErrGoToolchainOutput
	}
	function := request.Function.Origin()
	for _, unit := range a.Units {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return FunctionSourceObservation{}, err
		}
		if unit.Types != function.Pkg() {
			continue
		}
		position := function.Pos()
		file := unit.Fset.File(position)
		if file == nil {
			return FunctionSourceObservation{}, core.ErrGoToolchainOutput
		}
		for _, source := range unit.Syntax {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				return FunctionSourceObservation{}, err
			}
			if source == nil {
				return FunctionSourceObservation{}, core.ErrGoToolchainOutput
			}
			if unit.Fset.File(source.Pos()) != file {
				continue
			}
			index := sort.Search(len(source.Decls), func(index int) bool { return source.Decls[index].End() > position })
			if index == len(source.Decls) {
				return FunctionSourceObservation{}, core.ErrGoToolchainOutput
			}
			declaration, ok := source.Decls[index].(*ast.FuncDecl)
			if !ok {
				return FunctionSourceObservation{}, core.ErrGoToolchainOutput
			}
			binding, ok := unit.TypesInfo.Defs[declaration.Name].(*types.Func)
			if !ok || binding.Origin() != function {
				return FunctionSourceObservation{}, core.ErrGoToolchainOutput
			}
			observation := FunctionSourceObservation{Declaration: declaration, Types: unit.TypesInfo, Positions: unit.Fset, Presence: FunctionSourcePresent}
			if err := errors.Join(observation.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
				return FunctionSourceObservation{}, err
			}
			return observation, nil
		}
		return FunctionSourceObservation{}, core.ErrGoToolchainOutput
	}
	return FunctionSourceObservation{Presence: FunctionSourceAbsent}, nil
}

var (
	_ protocolFact     = FunctionSourceRequest{}
	_ protocolFact     = FunctionSourceObservation{}
	_ core.OffWireEnum = FunctionSourceUnknown
)
