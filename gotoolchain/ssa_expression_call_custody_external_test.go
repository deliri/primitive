package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/constant"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAExpressionCallRefusesBrokenNativeCustody(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"absent view", "nil function", "cloned function", "wrong native frame", "nil expression", "typed nil expression", "cloned expression", "canceled native cause", "missing operand", "surplus operand", "foreign operand", "wrong operand type", "malformed block ordinal", "removed call instruction", "cloned call instruction", "typed nil source value", "address metadata", "missing debug value", "missing type ownership", "missing type fact", "foreign expression type", "foreign function program"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func produce(int)<-chan struct{}{return nil};func observe(<-chan struct{}){};func Entry(){observe(produce(1))};func Other(input int){_=input}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var expression ast.Expr
				for node := range ast.Preorder(fn.Syntax()) {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == "observe" {
						expression = call.Args[0]
						break
					}
				}
				if expression == nil {
					t.Fatal("fixture has no native observed expression")
				}
				request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
				baseline, queryErr := v.ExpressionCall(ctx, request)
				if queryErr != nil || baseline.Call == nil || baseline.Call.Parent() != fn || baseline.ResultIndex != 0 {
					t.Fatalf("source loan precondition=%+v,%v", baseline, queryErr)
				}
				var reference *ssa.DebugRef
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if ref, ok := instruction.(*ssa.DebugRef); ok && ref.Expr == expression {
							reference = ref
						}
					}
				}
				if reference == nil {
					t.Fatal("fixture has no actual Go source reference")
				}
				want := error(core.ErrGoToolchainContract)
				switch boundary {
				case "absent view":
					v = gotoolchain.SSAUnitView{}
				case "nil function":
					request.Function = nil
				case "cloned function":
					clone := *fn
					request.Function = &clone
				case "wrong native frame":
					request.Function = v.Package.Func("Other")
				case "nil expression":
					request.Expression = nil
				case "typed nil expression":
					var absent *ast.CallExpr
					request.Expression = absent
				case "cloned expression":
					clone := *expression.(*ast.CallExpr)
					request.Expression = &clone
				case "canceled native cause":
					terminal, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = terminal
					want = io.ErrClosedPipe
				case "missing operand":
					baseline.Call.Common().Args[0] = nil
					want = core.ErrGoToolchainOutput
				case "surplus operand":
					baseline.Call.Common().Args = append(baseline.Call.Common().Args, baseline.Call.Common().Args[0])
					want = core.ErrGoToolchainOutput
				case "foreign operand":
					baseline.Call.Common().Args[0] = v.Package.Func("Other").Params[0]
					want = core.ErrGoToolchainOutput
				case "wrong operand type":
					baseline.Call.Common().Args[0] = ssa.NewConst(constant.MakeString("wrong"), types.Typ[types.String])
					want = core.ErrGoToolchainOutput
				case "malformed block ordinal":
					baseline.Call.Block().Index = 99
					want = core.ErrGoToolchainOutput
				case "removed call instruction":
					block := baseline.Call.Block()
					for i, instruction := range block.Instrs {
						if instruction == baseline.Call {
							block.Instrs = append(block.Instrs[:i], block.Instrs[i+1:]...)
							break
						}
					}
				case "cloned call instruction":
					clone := *baseline.Call
					reference.X = &clone
				case "typed nil source value":
					var absent *ssa.Call
					reference.X = absent
					want = core.ErrGoToolchainOutput
				case "address metadata":
					reference.IsAddr = true
					want = core.ErrGoToolchainOutput
				case "missing debug value":
					reference.Expr = nil
					want = core.ErrGoToolchainOutput
				case "missing type ownership":
					delete(v.Source.TypesInfo.Types, expression)
				case "missing type fact":
					fact := v.Source.TypesInfo.Types[expression]
					fact.Type = nil
					v.Source.TypesInfo.Types[expression] = fact
					want = core.ErrGoToolchainOutput
				case "foreign expression type":
					fact := v.Source.TypesInfo.Types[expression]
					fact.Type = types.Typ[types.Int]
					v.Source.TypesInfo.Types[expression] = fact
					want = core.ErrGoToolchainOutput
				case "foreign function program":
					fn.Prog = nil
				default:
					t.Fatal("fixture has no selected custody mutation")
				}
				got, queryErr := v.ExpressionCall(ctx, request)
				if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, want) {
					t.Fatalf("source custody %s got=%+v,%v, want zero and %v", boundary, got, queryErr, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
