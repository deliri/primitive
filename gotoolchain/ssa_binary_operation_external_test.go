package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeBinarySource = `package sample;func consumeBool(bool){};func consumeInt(int){};func Entry(x,y int,flag bool){consumeBool(x==y)};func Other(x,y int){consumeBool(x!=y)}`

func nativeBinaryExpression(t testing.TB, fn *ssa.Function) *ast.BinaryExpr {
	t.Helper()
	var expression *ast.BinaryExpr
	count := 0
	ast.Inspect(fn.Syntax(), func(node ast.Node) bool {
		if binary, ok := node.(*ast.BinaryExpr); ok {
			expression = binary
			count++
		}
		return true
	})
	if count != 1 {
		t.Fatalf("native source binary expressions=%d, want 1", count)
	}
	return expression
}

func TestSSABinaryOperationPreservesNativeExpressionIdentity(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body     string
		debug, present bool
		child          bool
	}{
		{"native equality", `consumeBool(x==y)`, true, true, false},
		{"native inequality", `consumeBool(x!=y)`, true, true, false},
		{"native less", `consumeBool(x<y)`, true, true, false},
		{"native less or equal", `consumeBool(x<=y)`, true, true, false},
		{"native greater", `consumeBool(x>y)`, true, true, false},
		{"native greater or equal", `consumeBool(x>=y)`, true, true, false},
		{"native addition", `consumeInt(x+y)`, true, true, false},
		{"native bitwise", `consumeInt(x&y)`, true, true, false},
		{"native shift", `consumeInt(x<<y)`, true, true, false},
		{"native subtraction", `consumeInt(x-y)`, true, true, false},
		{"native multiplication", `consumeInt(x*y)`, true, true, false},
		{"native division", `consumeInt(x/y)`, true, true, false},
		{"native remainder", `consumeInt(x%y)`, true, true, false},
		{"native bitwise or", `consumeInt(x|y)`, true, true, false},
		{"native bitwise xor", `consumeInt(x^y)`, true, true, false},
		{"native bitwise and not", `consumeInt(x&^y)`, true, true, false},
		{"native right shift", `consumeInt(x>>y)`, true, true, false},
		{"debug-free equality", `consumeBool(x==y)`, false, true, false},
		{"debug-free arithmetic", `consumeInt(x+y)`, false, true, false},
		{"debug-free inequality", `consumeBool(x!=y)`, false, true, false},
		{"debug-free less", `consumeBool(x<y)`, false, true, false},
		{"debug-free less or equal", `consumeBool(x<=y)`, false, true, false},
		{"debug-free greater", `consumeBool(x>y)`, false, true, false},
		{"debug-free greater or equal", `consumeBool(x>=y)`, false, true, false},
		{"debug-free subtraction", `consumeInt(x-y)`, false, true, false},
		{"debug-free multiplication", `consumeInt(x*y)`, false, true, false},
		{"debug-free division", `consumeInt(x/y)`, false, true, false},
		{"debug-free remainder", `consumeInt(x%y)`, false, true, false},
		{"debug-free bitwise and", `consumeInt(x&y)`, false, true, false},
		{"debug-free bitwise or", `consumeInt(x|y)`, false, true, false},
		{"debug-free bitwise xor", `consumeInt(x^y)`, false, true, false},
		{"debug-free bitwise and not", `consumeInt(x&^y)`, false, true, false},
		{"debug-free left shift", `consumeInt(x<<y)`, false, true, false},
		{"debug-free right shift", `consumeInt(x>>y)`, false, true, false},
		{"folded constant is absent", `consumeBool(1==2)`, true, false, false},
		{"short circuit is not a binary instruction", `consumeBool(flag&&flag)`, true, false, false},
		{"untaken expression is absent", `if false{consumeBool(x==y)}`, true, false, false},
		{"debug-free untaken expression is absent", `if false{consumeBool(x==y)}`, false, false, false},
		{"runtime branch retains operation", `if flag{consumeBool(x==y)}`, true, true, false},
		{"child expression grants no parent operation", `_=func(){consumeBool(x==y)}`, true, false, false},
		{"actual child owns its operation", `func(){consumeBool(x==y)}()`, true, true, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func consumeBool(bool){};func consumeInt(int){};func Entry(x,y int,flag bool){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: row.debug, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				if row.child {
					if len(fn.AnonFuncs) != 1 {
						t.Fatalf("native lexical children=%d, want 1", len(fn.AnonFuncs))
					}
					fn = fn.AnonFuncs[0]
				}
				expression := nativeBinaryExpression(t, fn)
				got, queryErr := view.BinaryOperationForExpression(ctx, gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: expression})
				if queryErr != nil || got.Validate() != nil || (got.Operation != nil) != row.present {
					t.Fatalf("native binary=(%+v,%v), want present=%t,nil", got, queryErr, row.present)
				}
				if operation := got.Operation; operation != nil {
					if operation.Parent() != fn || operation.Op != expression.Op || operation.Pos() != expression.OpPos {
						t.Fatalf("native operation parent/op/position=%v/%v/%v, want source frame/%v/%v", operation.Parent(), operation.Op, operation.Pos(), expression.Op, expression.OpPos)
					}
					members := 0
					for _, instruction := range operation.Block().Instrs {
						if instruction == operation {
							members++
						}
					}
					if members != 1 {
						t.Fatalf("native operation membership=%d, want 1 actual instruction", members)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABinaryOperationPreservesCustodyRefusals(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"absent loan", "nil function", "nil expression", "cloned expression", "other function", "foreign function", "bad ordinal", "detached native value", "nil native block", "changed source opcode", "changed source position"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBinarySource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				request := gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: nativeBinaryExpression(t, fn)}
				baseline, queryErr := view.BinaryOperationForExpression(ctx, request)
				if queryErr != nil || baseline.Operation == nil {
					t.Fatalf("native binary precondition=%+v,%v, want operation,nil", baseline, queryErr)
				}
				loan, want := view, core.ErrGoToolchainContract
				switch name {
				case "absent loan":
					loan = gotoolchain.SSAUnitView{}
				case "nil function":
					request.Function = nil
				case "nil expression":
					request.Expression = nil
				case "cloned expression":
					clone := *request.Expression
					request.Expression = &clone
				case "changed source opcode":
					request.Expression.Op = token.NEQ
				case "changed source position":
					request.Expression.OpPos++
				case "other function":
					request.Function = view.Package.Func("Other")
				case "foreign function":
					request.Function = &ssa.Function{}
				case "bad ordinal":
					baseline.Operation.Block().Index += 100
					want = core.ErrGoToolchainOutput
				case "detached native value":
					block := baseline.Operation.Block()
					clone := *baseline.Operation
					for index, instruction := range block.Instrs {
						if instruction == baseline.Operation {
							block.Instrs[index] = &clone
						}
					}
				case "nil native block":
					fn.Blocks[0] = nil
					want = core.ErrGoToolchainOutput
				default:
					t.Fatalf("binary custody case=%q, want declared boundary", name)
				}
				defer func() {
					if failure := recover(); failure != nil {
						t.Fatalf("native binary panic=%v, want zero observation and %v", failure, want)
					}
				}()
				got, queryErr := loan.BinaryOperationForExpression(ctx, request)
				if got != (gotoolchain.SSABinaryOperationObservation{}) || !errors.Is(queryErr, want) {
					t.Fatalf("binary refusal=%+v,%v, want zero,%v", got, queryErr, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABinaryOperationPreservesCauseAndEmptyObservation(t *testing.T) {
	t.Parallel()
	if err := (gotoolchain.SSABinaryOperationObservation{}).Validate(); err != nil {
		t.Fatalf("empty binary observation error=%v, want nil", err)
	}
	if err := (gotoolchain.SSABinaryOperationObservation{Operation: &ssa.BinOp{}}).Validate(); !errors.Is(err, core.ErrGoToolchainOutput) {
		t.Fatalf("detached binary observation error=%v, want %v", err, core.ErrGoToolchainOutput)
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBinarySource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		ctx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		cancel(io.ErrClosedPipe)
		got, queryErr := view.BinaryOperationForExpression(ctx, gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: nativeBinaryExpression(t, fn)})
		if got != (gotoolchain.SSABinaryOperationObservation{}) || !errors.Is(queryErr, io.ErrClosedPipe) || !errors.Is(queryErr, context.Canceled) {
			t.Fatalf("binary cancellation=%+v,%v, want zero and native cancellation/cause", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
