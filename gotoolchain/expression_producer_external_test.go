package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestExpressionProducerNativeIdentityAndFrameMatrix(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, expression, producer string
		slot                             uint64
	}{
		{"direct scalar", "", "produce()", "produce", 0},
		{"copied scalar", "value:=produce();", "value", "produce", 0},
		{"copied function", "call:=produce;", "call()", "produce", 0},
		{"first tuple slot", "value,_:=pair();", "value", "pair", 0},
		{"second tuple slot", "_,value:=pair();", "value", "pair", 1},
		{"copied tuple value", "value,_:=pair();copy:=value;", "copy", "pair", 0},
		{"original replaced after copy", "value,_:=pair();copy:=value;value=0;_=value;", "copy", "pair", 0},
		{"copy replaced", "value,_:=pair();copy:=value;copy=0;", "copy", "", 0},
		{"scalar replaced", "value:=produce();value=0;", "value", "", 0},
		{"native type conversion", "", "int(Number(produce()))", "produce", 0},
		{"outer helper owns result", "", "carry(produce())", "carry", 0},
		{"named method producer", "", "(Record{}).produce()", "produce", 0},
		{"dynamic callee stays opaque", "", "dynamic()", "", 0},
		{"anonymous callee stays opaque", "", "(func()int{return 0})()", "", 0},
		{"mixed merge stays opaque", "value:=produce();if flag{value=0};", "value", "", 0},
		{"different calls merged stay opaque", "value:=produce();if flag{value=produce()};", "value", "", 0},
		{"native constant has no producer", "", "7", "", 0},
		{"runtime input has no producer", "", "supplied", "", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			body := row.body + "observe(" + row.expression + ")"
			source := "package sample;type Number int;type Record struct{};func(Record)produce()int{return 0};func produce()int{return 1};func pair()(int,int){return 1,2};func carry(value int)int{return value};func observe(int){};var flag bool;var supplied int;var dynamic func()int;func F(){" + body + "};func G(){_=func(){" + body + "}};func init(){" + body + "}"
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			count := 0
			for _, unit := range analysis.Units {
				err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					for _, file := range unit.Syntax {
						for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: file}) {
							if err != nil {
								return err
							}
							call, ok := syntax.Node.(*ast.CallExpr)
							if !ok {
								continue
							}
							name, ok := call.Fun.(*ast.Ident)
							if !ok || name.Name != "observe" {
								continue
							}
							got, err := view.ExpressionProducer(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
							if err != nil || got.Validate() != nil {
								t.Fatalf("native producer got%+v/%v", got, err)
							}
							if row.producer == "" {
								if got != (gotoolchain.ExpressionProducerObservation{}) {
									t.Fatalf("opaque value manufactured producer%+v", got)
								}
							} else if got.Function == nil || got.Function.Name() != row.producer || got.ResultIndex != row.slot || got.Function.Pkg() != unit.Types {
								t.Fatalf("native producer got%+v want%s slot%d owned by actual package", got, row.producer, row.slot)
							}
							count++
						}
					}
					return nil
				}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if count != 3 {
				t.Fatalf("native producer frame loans got%d want3", count)
			}
		})
	}
}

func TestExpressionRequestAndProducerObservationBoundaries(t *testing.T) {
	t.Parallel()
	var typedNil *ast.Ident
	if err := (gotoolchain.ExpressionRequest{Expression: typedNil}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("typed nil expression got%v want native contract refusal", err)
	}
	pkg := types.NewPackage("example.com/native", "native")
	signature := types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewVar(0, pkg, "", types.Typ[types.Int])), false)
	function := types.NewFunc(0, pkg, "produce", signature)
	for _, row := range []struct {
		name        string
		observation gotoolchain.ExpressionProducerObservation
		want        error
	}{
		{name: "opaque observation"},
		{name: "native scalar result", observation: gotoolchain.ExpressionProducerObservation{Function: function}},
		{name: "slot without producer", observation: gotoolchain.ExpressionProducerObservation{ResultIndex: 1}, want: core.ErrGoToolchainContract},
		{name: "slot past native result", observation: gotoolchain.ExpressionProducerObservation{Function: function, ResultIndex: 1}, want: core.ErrGoToolchainContract},
		{name: "slot overflow", observation: gotoolchain.ExpressionProducerObservation{Function: function, ResultIndex: ^uint64(0)}, want: core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := row.observation.Validate(); !errors.Is(err, row.want) {
				t.Fatalf("producer observation got%v want%v", err, row.want)
			}
		})
	}
}

func TestExpressionProducerMissingSourceValuesRefuses(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func produce()int{return 0};func observe(int){};func F(){observe(produce())}")
	for _, unit := range analysis.Units {
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			calls := 0
			for _, file := range unit.Syntax {
				for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: file}) {
					if err != nil {
						return err
					}
					call, ok := syntax.Node.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if !ok || name.Name != "observe" {
						continue
					}
					got, err := view.ExpressionProducer(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
					if !errors.Is(err, core.ErrGoToolchainOutput) || got != (gotoolchain.ExpressionProducerObservation{}) {
						t.Fatalf("missing native source values got%+v/%v wantzero/refusal", got, err)
					}
					calls++
				}
			}
			if calls != 1 {
				t.Fatalf("native missing-source loans got%d want1", calls)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
}
