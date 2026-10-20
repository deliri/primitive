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
		for occurrence, err := range a.SymbolOccurrences(ctx) {
			if !yield(occurrence.Reference, err) || err != nil {
				return
			}
		}
	}
}

// SymbolOccurrence lends the compiler fact and its exact native syntax path
// together. Syntax.Ancestors is borrowed only until the next observation.
// Consumers need no coordinate lookup, symbol index or second syntax traversal.
type SymbolOccurrence struct {
	Reference SymbolReference
	Syntax    SyntaxNodeObservation
}

func (SymbolOccurrence) goToolchainProtocolFact() {}
func (o SymbolOccurrence) Validate() error {
	if err := errors.Join(o.Reference.Validate(), o.Syntax.Validate()); err != nil {
		return err
	}
	if _, ok := o.Syntax.Node.(*ast.Ident); !ok {
		return core.ErrGoToolchainContract
	}
	return nil
}

// SymbolOccurrences projects native declarations during one Go syntax walk.
// It retains only Go's depth-sized traversal path, with no occurrence population.
func (a PackageAnalysis) SymbolOccurrences(ctx context.Context) iter.Seq2[SymbolOccurrence, error] {
	return func(yield func(SymbolOccurrence, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), a.Validate()); err != nil {
			yield(SymbolOccurrence{}, err)
			return
		}
		if a.Incomplete {
			yield(SymbolOccurrence{}, core.ErrGoToolchainOutput)
			return
		}
		for _, unit := range a.Units {
			for _, source := range unit.Syntax {
				for syntax, err := range WalkSyntax(ctx, SyntaxWalkRequest{Root: source}) {
					if err != nil {
						yield(SymbolOccurrence{}, err)
						return
					}
					identifier, ok := syntax.Node.(*ast.Ident)
					if !ok {
						continue
					}
					function, ok := unit.TypesInfo.Uses[identifier].(*types.Func)
					if !ok || function.Pkg() == nil {
						continue
					}
					symbol, kind, err := observedFunctionSymbol(function)
					position := unit.Fset.PositionFor(identifier.Pos(), false)
					file, pathErr := core.ParseAbsolutePath(position.Filename)
					observation := SymbolOccurrence{Reference: SymbolReference{Symbol: symbol, Kind: kind, File: file, Offset: uint64(position.Offset), Line: uint64(position.Line), Column: uint64(position.Column)}, Syntax: syntax}
					if err := errors.Join(err, pathErr, contextstate.Validate(ctx), observation.Validate()); err != nil {
						yield(SymbolOccurrence{}, err)
						return
					}
					if !yield(observation, nil) {
						return
					}
				}
			}
		}
		if err := contextstate.Validate(ctx); err != nil {
			yield(SymbolOccurrence{}, err)
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
