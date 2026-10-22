package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestExpressionNilNativeValueAndFrameMatrix(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, expression string
		want                   bool
	}{
		{"predeclared nil", "", "nil", true},
		{"pointer conversion", "", "(*int)(nil)", true},
		{"interface conversion", "", "any(nil)", true},
		{"slice conversion", "", "[]int(nil)", true},
		{"map conversion", "", "map[int]int(nil)", true},
		{"function conversion", "", "(func())(nil)", true},
		{"channel conversion", "", "(chan int)(nil)", true},
		{"zero interface", "var value any;", "value", true},
		{"zero pointer", "var value *int;", "value", true},
		{"nil copy", "var value any;copy:=value;", "copy", true},
		{"typed pointer inside interface", "", "any((*int)(nil))", false},
		{"allocated pointer", "", "new(int)", false},
		{"integer zero", "", "0", false},
		{"empty slice", "", "[]int{}", false},
		{"opaque parameter", "", "supplied", false},
		{"replacement", "var value any;value=1;", "value", false},
		{"mixed merge", "var value any;if flag {value=1};", "value", false},
		{"shadowed nil", "nil:=1;", "nil", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			body := row.body + "observe(" + row.expression + ")"
			source := "package sample;func observe(any){};var supplied any;var flag bool;func F(){" + body + "};func G(){_=func(){" + body + "}};func init(){" + body + "}"
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
							got, err := view.ExpressionNil(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
							if err != nil || got.Validate() != nil || got.Proven != row.want {
								t.Fatalf("native nil value in frame %d got%+v/%v want%t/nil", count, got, err, row.want)
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
				t.Fatalf("native expression loans got%d want3", count)
			}
		})
	}
}

func TestExpressionNilRefusesForeignAndMissingNativeFacts(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func observe(any){};func F(){var value any;observe(value);observe(nil)}")
	for _, row := range []struct {
		name   string
		debug  bool
		damage uint8
		want   error
	}{
		{"native metadata", true, 0, nil}, {"source values absent", false, 0, core.ErrGoToolchainOutput},
		{"foreign expression", true, 1, core.ErrGoToolchainContract}, {"missing type value", true, 2, core.ErrGoToolchainOutput},
		{"foreign analysis", true, 3, core.ErrGoToolchainContract}, {"incomplete analysis", true, 4, core.ErrGoToolchainOutput},
		{"nil expression", true, 5, core.ErrGoToolchainContract}, {"canceled query", true, 6, context.Canceled},
		{"nil query context", true, 7, core.ErrNilContext},
	} {
		t.Run(row.name, func(t *testing.T) {
			// This subtest temporarily damages its loan; no compiler object is shared concurrently.
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: row.debug, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
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
						request := gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]}
						switch row.damage {
						case 1:
							copy := *call.Args[0].(*ast.Ident)
							request.Expression = &copy
							// Individually valid native nil facts cannot grant a cloned
							// node ownership of this source expression.
							for _, facts := range unit.TypesInfo.Types {
								if facts.IsNil() {
									unit.TypesInfo.Types[&copy] = facts
									break
								}
							}
							defer delete(unit.TypesInfo.Types, &copy)
						case 2:
							facts := unit.TypesInfo.Types[call.Args[0]]
							delete(unit.TypesInfo.Types, call.Args[0])
							defer func() { unit.TypesInfo.Types[call.Args[0]] = facts }()
						case 3:
							request.Analysis.Units = nil
						case 4:
							request.Analysis.Incomplete = true
						case 5:
							request.Expression = nil
						case 6:
							var cancel context.CancelFunc
							ctx, cancel = context.WithCancel(ctx)
							cancel()
						case 7:
							ctx = nil
						}
						got, err := view.ExpressionNil(ctx, request)
						if !errors.Is(err, row.want) || got != (gotoolchain.ExpressionNilObservation{Proven: row.want == nil}) {
							t.Fatalf("native nil refusal got%+v/%v want zero=%t/%v", got, err, row.want != nil, row.want)
						}
						return nil
					}
				}
				t.Fatal("native query not exercised")
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExpressionNilNativeIteratorFrames(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, expression string
		want                   bool
	}{
		{"zero local interface", "var value any;", "value", true},
		{"captured opaque interface", "", "supplied", false},
		{"typed nil inside interface", "value:=any((*int)(nil));", "value", false},
		{"local interface replacement", "var value any;value=1;", "value", false},
		{"local nil copy", "var value any;copy:=value;", "copy", true},
		{"mixed native merge", "var value any;if flag{value=1};", "value", false},
		{"explicit nil", "", "nil", true},
		{"allocated pointer", "", "new(int)", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			body := row.body + "observe(" + row.expression + ")"
			loop := "for range sequence {" + body + "}"
			source := "package sample;func observe(any){};var supplied any;var flag bool;func sequence(yield func(int)bool){yield(1)};func F(){" + loop + "};func G(){for range sequence {" + loop + "}};func H(){for range sequence {_=func(){" + body + "}}};func init(){" + loop + "}"
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
							got, err := view.ExpressionNil(ctx, gotoolchain.ExpressionRequest{Analysis: analysis, Expression: call.Args[0]})
							if err != nil || got.Validate() != nil || got.Proven != row.want {
								t.Fatalf("native nil value in frame %d got%+v/%v want%t/nil", count, got, err, row.want)
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
			if count != 4 {
				t.Fatalf("native expression loans got%d want4", count)
			}
		})
	}
}
