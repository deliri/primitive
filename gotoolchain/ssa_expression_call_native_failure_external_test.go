package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

type expressionCallTypeAssertion struct{ ssa.Value }

func (*expressionCallTypeAssertion) Type() types.Type { panic(io.ErrClosedPipe) }

func TestSSAExpressionCallPreservesNativeAccessorCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func produce()int{return 1};func observe(int){};func Entry(){observe(produce())}`)
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
			t.Fatal("fixture has no observed expression")
		}
		request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
		baseline, queryErr := v.ExpressionCall(ctx, request)
		if queryErr != nil || baseline.Call == nil {
			t.Fatalf("native source precondition=%+v,%v", baseline, queryErr)
		}
		for _, ordinal := range []uint64{1, ^uint64(0)} {
			invalid := gotoolchain.SSAExpressionCallObservation{Call: baseline.Call, ResultIndex: ordinal}
			if queryErr := invalid.Validate(); !errors.Is(queryErr, core.ErrGoToolchainOutput) {
				t.Fatalf("native result ordinal %d got=%v, want output refusal", ordinal, queryErr)
			}
		}
		baseline.Call.Common().Value = &expressionCallTypeAssertion{}
		if queryErr := baseline.Validate(); !errors.Is(queryErr, core.ErrGoToolchainOutput) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("native observation cause=%v, want output refusal and native closed pipe", queryErr)
		}
		got, queryErr := v.ExpressionCall(ctx, request)
		if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("native accessor query=%+v,%v, want zero and both error identities", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAExpressionCallRefusesCyclicAndInvalidResultEdges(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"self conversion", "negative tuple slot", "past tuple slot", "cloned tuple extraction"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			body := `channel,_:=pair();observe(channel)`
			if boundary == "self conversion" {
				body = `channel:=Signal(produce());observe(channel)`
			}
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Signal <-chan struct{};func produce()<-chan struct{}{return nil};func pair()(<-chan struct{},int){return nil,0};func observe(<-chan struct{}){};func Entry(){`+body+`}`)
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
					t.Fatal("fixture has no native observed edge")
				}
				request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
				baseline, queryErr := v.ExpressionCall(ctx, request)
				if queryErr != nil || baseline.Call == nil {
					t.Fatalf("native edge precondition=%+v,%v", baseline, queryErr)
				}
				value, _ := fn.ValueForExpr(expression)
				want := error(core.ErrGoToolchainOutput)
				if boundary == "self conversion" {
					converted, ok := value.(*ssa.ChangeType)
					if !ok {
						t.Fatalf("native conversion edge=%T", value)
					}
					converted.X = converted
				} else {
					extracted, ok := value.(*ssa.Extract)
					if !ok {
						t.Fatalf("native tuple edge=%T", value)
					}
					switch boundary {
					case "negative tuple slot":
						extracted.Index = -1
					case "past tuple slot":
						extracted.Index = 2
					case "cloned tuple extraction":
						clone := *extracted
						for _, block := range fn.Blocks {
							for _, instruction := range block.Instrs {
								if ref, ok := instruction.(*ssa.DebugRef); ok && ref.Expr == expression {
									ref.X = &clone
								}
							}
						}
						want = core.ErrGoToolchainContract
					}
				}
				got, queryErr := v.ExpressionCall(ctx, request)
				if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, want) {
					t.Fatalf("native edge %s got=%+v,%v, want zero and %v", boundary, got, queryErr, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAExpressionCallPreservesSourceValueAccessorCause(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func produce()int{return 1};func observe(int){};func Entry(){observe(produce())}`)
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
			t.Fatal("fixture has no observed native source value")
		}
		request := gotoolchain.SSAExpressionCallRequest{Function: fn, Expression: expression}
		before, queryErr := v.ExpressionCall(ctx, request)
		if queryErr != nil || before.Call == nil {
			t.Fatalf("native source-value precondition=%+v,%v", before, queryErr)
		}
		replaced := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if ref, ok := instruction.(*ssa.DebugRef); ok && ref.Expr == expression {
					ref.X = &expressionCallTypeAssertion{}
					replaced++
				}
			}
		}
		if replaced != 1 {
			t.Fatalf("native source value replacements=%d, want 1", replaced)
		}
		got, queryErr := v.ExpressionCall(ctx, request)
		if got != (gotoolchain.SSAExpressionCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("source value accessor=%+v,%v, want zero and output/native cause identities", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
