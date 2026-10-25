package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSASourceFunctionRefusesUnownedInputAndBrokenNativeOutput(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		damage      uint8
		want        error
		nativePanic bool
	}{
		{"unknown selection", 0, core.ErrGoToolchainContract, false},
		{"invalid selection", 1, core.ErrGoToolchainContract, false},
		{"missing declaration", 2, core.ErrGoToolchainContract, false},
		{"initializer with declaration", 3, core.ErrGoToolchainContract, false},
		{"missing declaration name", 4, core.ErrGoToolchainContract, false},
		{"copied declaration", 5, core.ErrGoToolchainContract, false},
		{"copied declaration and identifier", 6, core.ErrGoToolchainContract, false},
		{"foreign source declaration", 7, core.ErrGoToolchainContract, false},
		{"missing view", 8, core.ErrGoToolchainContract, false},
		{"missing native binding", 9, core.ErrGoToolchainOutput, false},
		{"typed nil native function", 10, core.ErrGoToolchainOutput, false},
		{"wrong native object kind", 11, core.ErrGoToolchainOutput, false},
		{"counterfeit function identity", 12, core.ErrGoToolchainOutput, false},
		{"foreign native object", 13, core.ErrGoToolchainOutput, false},
		{"different actual declaration object", 14, core.ErrGoToolchainOutput, false},
		{"missing source file", 15, core.ErrGoToolchainOutput, false},
		{"missing declaration population", 16, core.ErrGoToolchainOutput, false},
		{"native declaration search panic", 17, core.ErrGoToolchainOutput, true},
		{"copied synthetic initializer", 18, core.ErrGoToolchainOutput, false},
		{"named function substituted for initializer", 19, core.ErrGoToolchainOutput, false},
		{"initializer block ordinal contradiction", 20, core.ErrGoToolchainOutput, false},
		{"nil initializer block", 21, core.ErrGoToolchainOutput, false},
		{"nil context", 22, core.ErrNilContext, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;var State int;func Entry(){};func Other(){}`)
			foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				decl := v.Source.Syntax[0].Decls[1].(*ast.FuncDecl)
				original := v.Source.TypesInfo.Defs[decl.Name].(*types.Func)
				request := gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: decl}
				view := v
				switch row.damage {
				case 0:
					request.Kind = gotoolchain.SSASourceFunctionUnknown
				case 1:
					request.Kind = gotoolchain.SSASourceFunctionKind(255)
				case 2:
					request.Declaration = nil
				case 3:
					request.Kind = gotoolchain.SSASourceFunctionInitializer
				case 4:
					copy := *decl
					copy.Name = nil
					request.Declaration = &copy
				case 5:
					copy := *decl
					request.Declaration = &copy
				case 6:
					copy := *decl
					name := *decl.Name
					copy.Name = &name
					request.Declaration = &copy
				case 7:
					request.Declaration = foreign.Units[0].Syntax[0].Decls[0].(*ast.FuncDecl)
				case 8:
					view = gotoolchain.SSAUnitView{}
				case 9:
					delete(v.Source.TypesInfo.Defs, decl.Name)
				case 10:
					v.Source.TypesInfo.Defs[decl.Name] = (*types.Func)(nil)
				case 11:
					v.Source.TypesInfo.Defs[decl.Name] = v.Source.Types.Scope().Lookup("State")
				case 12:
					v.Source.TypesInfo.Defs[decl.Name] = types.NewFunc(original.Pos(), original.Pkg(), original.Name(), original.Signature())
				case 13:
					v.Source.TypesInfo.Defs[decl.Name] = foreign.Units[0].Types.Scope().Lookup("Entry")
				case 14:
					v.Source.TypesInfo.Defs[decl.Name] = v.Source.Types.Scope().Lookup("Other")
				case 15:
					v.Source.Syntax[0] = nil
				case 16:
					v.Source.Syntax[0].Decls = nil
				case 17:
					v.Source.Syntax[0].Decls[1] = nil
				case 18:
					request = gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer}
					copy := *v.Package.Func("init")
					v.Package.Members["init"] = &copy
				case 19:
					request = gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer}
					v.Package.Members["init"] = v.Package.Func("Other")
				case 20:
					request = gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer}
					v.Package.Func("init").Blocks[0].Index += 100
				case 21:
					request = gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionInitializer}
					v.Package.Func("init").Blocks[0] = nil
				case 22:
					ctx = nil
				}
				got, queryErr := view.SourceFunction(ctx, request)
				if got != (gotoolchain.SSASourceFunctionObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("source custody %s loan=%+v,%v, want zero and %v", row.name, got, queryErr, row.want)
				}
				if row.nativePanic {
					var cause runtime.Error
					if !errors.As(queryErr, &cause) {
						t.Fatalf("native declaration panic lost its runtime cause: %v", queryErr)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
