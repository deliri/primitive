package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeFieldSemanticDeclarations = `package sample;type S struct{A int;B int;Flag bool};func work(){};func Other(s *S){};func OtherPointer(p *int){};`

func nativeFieldSemanticFunction(name string, mode uint8) string {
	parameter, expression, result := "s *S", "s.A", "int"
	switch mode & 3 {
	case 1:
		expression = "s.B"
	case 2:
		expression, result = "s.Flag", "bool"
	case 3:
		parameter, expression = "p *int", "*p"
	}
	return "func " + name + "(" + parameter + ")" + result + "{" + strings.Repeat("work();", int(mode>>4)) + "return " + expression + "};"
}

func TestSSAFieldReadFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(nativeFieldSemanticDeclarations)
	for mode := 0; mode <= 255; mode++ {
		source.WriteString(nativeFieldSemanticFunction(fmt.Sprintf("F%d", mode), uint8(mode)))
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		for mode := 0; mode <= 255; mode++ {
			nativeFieldSemanticObservation(ctx, t, view, view.Package.Func(fmt.Sprintf("F%d", mode)), uint8(mode))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func FuzzSSAFieldReadConservesNativeOperandAuthority(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 4, 7, 8, 11, 12, 15, 16, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeFieldSemanticDeclarations+nativeFieldSemanticFunction("Entry", mode))
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			nativeFieldSemanticObservation(ctx, t, view, view.Package.Func("Entry"), mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit got=%v, want nil", err)
		}
	})
}

func nativeFieldSemanticObservation(ctx context.Context, t *testing.T, view gotoolchain.SSAUnitView, fn *ssa.Function, mode uint8) {
	t.Helper()
	field := (mode & 3) != 3
	read := nativeFieldReadInstruction(t, fn, field)
	if read.Op != token.MUL {
		t.Fatalf("mode=%d read opcode got=%v, want dereference", mode, read.Op)
	}
	noise := 0
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "work" {
				noise++
			}
		}
	}
	if noise != int(mode>>4) {
		t.Fatalf("mode=%d native prefix got=%d, want %d", mode, noise, mode>>4)
	}
	var want gotoolchain.SSAFieldReadObservation
	if field {
		record := view.Source.Types.Scope().Lookup("S").Type().Underlying().(*types.Struct)
		want = gotoolchain.SSAFieldReadObservation{Base: fn.Params[0], Field: record.Field(int(mode & 3))}
	}
	before, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
	if queryErr != nil || before != want {
		t.Fatalf("mode=%d field precondition got=%+v,%v, want %+v,nil", mode, before, queryErr, want)
	}
	damage := (mode >> 2) & 3
	var wantErr error
	if damage != 0 {
		switch damage {
		case 1:
			read.X = nil
		case 2:
			if field {
				read.X.(*ssa.FieldAddr).X = view.Package.Func("Other").Params[0]
			} else {
				read.X = view.Package.Func("OtherPointer").Params[0]
			}
		case 3:
			if field {
				copied := *read.X.(*ssa.FieldAddr)
				read.X = &copied
			} else {
				copied := *fn.Params[0]
				read.X = &copied
			}
		}
		want, wantErr = gotoolchain.SSAFieldReadObservation{}, core.ErrGoToolchainOutput
	}
	got, queryErr := view.FieldRead(ctx, gotoolchain.SSAFieldReadRequest{Read: read})
	if got != want || !errors.Is(queryErr, wantErr) || got.Validate() != nil {
		t.Fatalf("mode=%d field loan got=%+v,%v, want %+v,%v", mode, got, queryErr, want, wantErr)
	}
}
