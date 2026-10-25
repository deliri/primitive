package gotoolchain

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"sort"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

type SSASourceFunctionKind uint8

const (
	SSASourceFunctionUnknown SSASourceFunctionKind = iota
	SSASourceFunctionDeclaration
	SSASourceFunctionInitializer
)

func (k SSASourceFunctionKind) Validate() error {
	if k != SSASourceFunctionDeclaration && k != SSASourceFunctionInitializer {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (k SSASourceFunctionKind) IsValid() bool { return k.Validate() == nil }
func (SSASourceFunctionKind) OffWireEnum()    {}
func (k SSASourceFunctionKind) String() string {
	switch k {
	case SSASourceFunctionDeclaration:
		return "declaration"
	case SSASourceFunctionInitializer:
		return "initializer"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// SSASourceFunctionRequest selects an exact native declaration or the selected
// unit's synthetic initializer. A declaration copy or matching spelling cannot
// replace source identity. Initializer selection carries no declaration.
type SSASourceFunctionRequest struct {
	Kind        SSASourceFunctionKind
	Declaration *ast.FuncDecl
}

func (r SSASourceFunctionRequest) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if r.Kind == SSASourceFunctionInitializer {
		if r.Declaration != nil {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	if r.Declaration == nil || r.Declaration.Name == nil || r.Declaration.Type == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSASourceFunctionRequest) goToolchainInternalFlow() {}

type SSASourceFunctionObservation struct{ Function *ssa.Function }

func (o SSASourceFunctionObservation) Validate() error {
	if o.Function == nil {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (SSASourceFunctionObservation) goToolchainInternalFlow() {}

// SourceFunction lends Go's actual SSA binding. File identity and native
// source-ordered declaration search authenticate the AST without a second
// function catalog. Go's private object map owns declaration-to-SSA lookup.
// Bodyless source declarations remain valid loans with no instructions.
func (v SSAUnitView) SourceFunction(ctx context.Context, r SSASourceFunctionRequest) (out SSASourceFunctionObservation, resultErr error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return out, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			out = SSASourceFunctionObservation{}
		}
	}()
	if r.Kind == SSASourceFunctionInitializer {
		fn := v.Package.Func("init")
		if fn == nil || fn.Prog != v.Program || fn.Pkg != v.Package || fn.Parent() != nil || fn.Syntax() != nil || fn.Object() != nil || len(fn.Blocks) == 0 || fn.Blocks[0] == nil || fn.Blocks[0].Parent() != fn || fn.Blocks[0].Index != 0 {
			return out, core.ErrGoToolchainOutput
		}
		return SSASourceFunctionObservation{Function: fn}, nil
	}
	file := v.Source.Fset.File(r.Declaration.Pos())
	if file == nil {
		return out, core.ErrGoToolchainContract
	}
	owned := false
	for _, syntax := range v.Source.Syntax {
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return out, err
		}
		if syntax == nil {
			return out, core.ErrGoToolchainOutput
		}
		if v.Source.Fset.File(syntax.Pos()) != file {
			continue
		}
		var err error
		owned, err = sourceFunctionDeclarationOwned(syntax, r.Declaration)
		if err != nil {
			return out, err
		}
		break
	}
	if !owned {
		// A private native binding can still prove the exact requested node.
		// Losing its source list is missing compiler output, not a caller clone.
		if object, ok := v.Source.TypesInfo.Defs[r.Declaration.Name].(*types.Func); ok && object != nil {
			if fn := v.Program.FuncValue(object); fn != nil && fn.Syntax() == r.Declaration {
				return out, core.ErrGoToolchainOutput
			}
		}
		return out, core.ErrGoToolchainContract
	}
	object, ok := v.Source.TypesInfo.Defs[r.Declaration.Name].(*types.Func)
	if !ok || object == nil || object.Pkg() != v.Source.Types || object.Signature() == nil || object.Pos() != r.Declaration.Name.Pos() || object.Name() != r.Declaration.Name.Name {
		return out, core.ErrGoToolchainOutput
	}
	fn := v.Program.FuncValue(object)
	if fn == nil || fn.Prog != v.Program || fn.Pkg != v.Package || fn.Syntax() != r.Declaration || fn.Object() != object {
		return out, core.ErrGoToolchainOutput
	}
	return SSASourceFunctionObservation{Function: fn}, nil
}

// Keep recovery at the native AST/search boundary; caller code never runs here.
func sourceFunctionDeclarationOwned(file *ast.File, declaration *ast.FuncDecl) (owned bool, resultErr error) {
	defer func() {
		if failure := recover(); failure != nil {
			owned = false
			if cause, ok := failure.(error); ok {
				resultErr = errors.Join(core.ErrGoToolchainOutput, cause)
			} else {
				resultErr = fmt.Errorf("%w: native source declaration: %v", core.ErrGoToolchainOutput, failure)
			}
		}
	}()
	position := declaration.Pos()
	index := sort.Search(len(file.Decls), func(index int) bool { return file.Decls[index].End() > position })
	return index < len(file.Decls) && file.Decls[index] == declaration, nil
}

var (
	_ internalFlow     = SSASourceFunctionRequest{}
	_ internalFlow     = SSASourceFunctionObservation{}
	_ core.OffWireEnum = SSASourceFunctionUnknown
)
