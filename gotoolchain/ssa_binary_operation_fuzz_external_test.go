package gotoolchain_test

import (
	"context"
	"go/token"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSABinaryOperationConservesNativeSource(f *testing.F) {
	for _, mode := range []uint16{0, 31, 64, 128, 256, 511} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint16) {
		operators := [...]token.Token{token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ, token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT, token.SHL, token.SHR}
		operator := operators[int(mode)%len(operators)]
		constant, dead := mode&32 != 0, mode&64 != 0
		child, parent := mode&128 != 0, mode&256 != 0
		left, right := "x", "y"
		if constant {
			left, right = "1", "2"
		}
		body := "consume(" + left + operator.String() + right + ")"
		if dead {
			body = "if false{" + body + "}"
		}
		if child {
			body = "func(){" + body + "}()"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;func consume[T any](T){};func Entry(x,y int){"+body+"}")
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: mode&16 == 0, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			fn := view.Package.Func("Entry")
			if child && !parent {
				if len(fn.AnonFuncs) != 1 {
					t.Fatalf("native fuzz children=%d, want 1", len(fn.AnonFuncs))
				}
				fn = fn.AnonFuncs[0]
			}
			expression := nativeBinaryExpression(t, fn)
			got, queryErr := view.BinaryOperationForExpression(ctx, gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: expression})
			want := !constant && !dead && (!child || !parent)
			if queryErr != nil || got.Validate() != nil || (got.Operation != nil) != want {
				t.Fatalf("native binary mode=%d observation=%+v,%v, want present=%t,nil", mode, got, queryErr, want)
			}
			if operation := got.Operation; operation != nil {
				if operation.Parent() != fn || operation.Op != operator || operation.Pos() != expression.OpPos || !types.Identical(operation.Type(), view.Source.TypesInfo.TypeOf(expression)) {
					t.Fatalf("native binary mode=%d operation=%v, want exact source opcode, position, type and frame", mode, operation)
				}
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
