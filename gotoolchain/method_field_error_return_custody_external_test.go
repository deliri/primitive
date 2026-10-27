package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/ast"
	"go/types"
	"testing"
)

func TestReturnedRecordFieldProofRefusesDamagedCompilerBindings(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*ast.CompositeLit, *types.Info, *types.Struct)
	}{
		{name: "missing selected key", mutate: func(literal *ast.CompositeLit, facts *types.Info, _ *types.Struct) {
			delete(facts.Uses, literal.Elts[0].(*ast.KeyValueExpr).Key.(*ast.Ident))
		}},
		{name: "missing later key cannot hide behind earlier success", mutate: func(literal *ast.CompositeLit, facts *types.Info, _ *types.Struct) {
			delete(facts.Uses, literal.Elts[1].(*ast.KeyValueExpr).Key.(*ast.Ident))
		}},
		{name: "copied field object cannot own slot", mutate: func(literal *ast.CompositeLit, facts *types.Info, _ *types.Struct) {
			key := literal.Elts[0].(*ast.KeyValueExpr).Key.(*ast.Ident)
			field := facts.Uses[key].(*types.Var)
			facts.Uses[key] = types.NewField(field.Pos(), field.Pkg(), field.Name(), field.Type(), field.Embedded())
		}},
		{name: "neighboring field cannot own differently named key", mutate: func(literal *ast.CompositeLit, facts *types.Info, record *types.Struct) {
			facts.Uses[literal.Elts[0].(*ast.KeyValueExpr).Key.(*ast.Ident)] = record.Field(0)
		}},
		{name: "missing constructed record type", mutate: func(literal *ast.CompositeLit, facts *types.Info, _ *types.Struct) {
			delete(facts.Types, literal)
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type refusal uint8;func(refusal)Error()string{return "refusal"};type Child struct{Count int};func(v Child)Inspect()error{if v.Count<0{return refusal(1)};return nil};type Parent struct{A,B Child};type Bridge struct{Other,Selected Child};func(v Bridge)Check()error{return v.Selected.Inspect()};func(v Parent)project()Bridge{return Bridge{Selected:v.A,Other:v.B}};func(v Parent)Check()error{return v.project().Check()}`)
			unit := analysis.Units[0]
			method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
			target := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
			record := unit.Types.Scope().Lookup("Bridge").Type().Underlying().(*types.Struct)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				request := gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Target: target}
				before, queryErr := view.MethodFieldErrorPropagation(ctx, request)
				if queryErr != nil || !before.Propagated || before.Source != gotoolchain.FunctionSourcePresent {
					t.Fatalf("intact constructed field=%+v error=%v, want real native child propagation before damage", before, queryErr)
				}
				changed := 0
				for _, file := range unit.Syntax {
					for node := range ast.Preorder(file) {
						literal, ok := node.(*ast.CompositeLit)
						if ok && unit.TypesInfo.TypeOf(literal) == unit.Types.Scope().Lookup("Bridge").Type() {
							row.mutate(literal, unit.TypesInfo, record)
							changed++
						}
					}
				}
				if changed != 1 {
					t.Fatalf("changed constructed records=%d, want exactly one native literal", changed)
				}
				got, queryErr := view.MethodFieldErrorPropagation(ctx, request)
				if got != (gotoolchain.MethodFieldErrorPropagationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("damaged constructed field=%+v error=%v, want zero observation and %v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("damaged constructed-field native loan error=%v, want nil", err)
			}
		})
	}
}
