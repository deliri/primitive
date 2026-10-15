package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// SymbolReferenceKind preserves Go's distinction between a package function,
// a method on a named receiver, and an anonymous interface method. An anonymous
// receiver supplies no concrete capability identity and must remain unresolved.
type SymbolReferenceKind uint8

const (
	SymbolReferenceUnknown SymbolReferenceKind = iota
	SymbolReferenceFunction
	SymbolReferenceNamedMethod
	SymbolReferenceAnonymousMethod
)

func (k SymbolReferenceKind) Validate() error {
	if k < SymbolReferenceFunction || k > SymbolReferenceAnonymousMethod {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (k SymbolReferenceKind) IsValid() bool { return k.Validate() == nil }
func (SymbolReferenceKind) OffWireEnum()    {}
func (k SymbolReferenceKind) String() string {
	switch k {
	case SymbolReferenceFunction:
		return "function"
	case SymbolReferenceNamedMethod:
		return "named method"
	case SymbolReferenceAnonymousMethod:
		return "anonymous interface method"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// SymbolReference is one compiler-resolved function or method reference. The
// declaring receiver comes from Go's signature, including aliases, promotion,
// method expressions and method values. Source coordinates are physical: a
// //line directive cannot redirect the observation to another source file.
type SymbolReference struct {
	Symbol capabilities.StandardSymbol
	Kind   SymbolReferenceKind
	File   core.AbsolutePath
	Offset uint64
	Line   uint64
	Column uint64
}

func (r SymbolReference) Validate() error {
	if err := errors.Join(r.Symbol.Validate(), r.File.Validate(), r.Kind.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if r.Line == 0 || r.Column == 0 || (r.Kind == SymbolReferenceNamedMethod) != (r.Symbol.Receiver != nil) {
		return core.ErrGoToolchainContract
	}
	return nil
}

// SymbolReferences lends references in source order from this one native Go
// analysis. It retains no reference collection or ownership classification.
// Function values are observed at their origin as well as direct calls, so
// assigning an effectful method to a local variable cannot erase its identity.
func (a PackageAnalysis) SymbolReferences(ctx context.Context) iter.Seq2[SymbolReference, error] {
	return func(yield func(SymbolReference, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), a.Validate()); err != nil {
			yield(SymbolReference{}, err)
			return
		}
		if a.Incomplete {
			yield(SymbolReference{}, core.ErrGoToolchainOutput)
			return
		}
		for _, unit := range a.Units {
			for _, source := range unit.Syntax {
				stopped := false
				ast.Inspect(source, func(node ast.Node) bool {
					if stopped {
						return false
					}
					identifier, ok := node.(*ast.Ident)
					if !ok {
						return true
					}
					function, ok := unit.TypesInfo.Uses[identifier].(*types.Func)
					if !ok || function.Pkg() == nil {
						return true
					}
					symbol, kind, err := observedFunctionSymbol(function)
					position := unit.Fset.PositionFor(identifier.Pos(), false)
					file, pathErr := core.ParseAbsolutePath(position.Filename)
					observation := SymbolReference{Symbol: symbol, Kind: kind, File: file, Offset: uint64(position.Offset), Line: uint64(position.Line), Column: uint64(position.Column)}
					err = errors.Join(err, pathErr, contextstate.Validate(ctx), observation.Validate())
					if err != nil {
						yield(SymbolReference{}, err)
						stopped = true
						return false
					}
					stopped = !yield(observation, nil)
					return !stopped
				})
				if stopped {
					return
				}
			}
		}
		if err := contextstate.Validate(ctx); err != nil {
			yield(SymbolReference{}, err)
		}
	}
}

func observedFunctionSymbol(function *types.Func) (capabilities.StandardSymbol, SymbolReferenceKind, error) {
	imported, importErr := gomodule.ParseImportPath(function.Pkg().Path())
	selector, nameErr := capabilities.ParseSymbolName(function.Name())
	symbol := capabilities.StandardSymbol{ImportPath: imported, Selector: selector}
	if err := errors.Join(importErr, nameErr); err != nil {
		return capabilities.StandardSymbol{}, SymbolReferenceUnknown, err
	}
	kind := SymbolReferenceFunction
	if receiver := function.Signature().Recv(); receiver != nil {
		declared := types.Unalias(receiver.Type())
		if pointer, ok := declared.(*types.Pointer); ok {
			declared = types.Unalias(pointer.Elem())
		}
		named, ok := declared.(*types.Named)
		if !ok {
			return symbol, SymbolReferenceAnonymousMethod, symbol.Validate()
		}
		name, err := capabilities.ParseSymbolName(named.Obj().Name())
		if err != nil {
			return capabilities.StandardSymbol{}, SymbolReferenceUnknown, err
		}
		symbol.Receiver = &name
		kind = SymbolReferenceNamedMethod
	}
	return symbol, kind, symbol.Validate()
}

func (SymbolReference) goToolchainProtocolFact() {}

var _ protocolFact = SymbolReference{}
