package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAObjectWrittenRequiresExactNativeStoreReference(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		change func(*ssa.DebugRef)
		want   error
	}{
		{"different native value", func(r *ssa.DebugRef) { r.X = ssa.NewConst(nil, r.X.Type()) }, nil},
		{"address reference is no assignment", func(r *ssa.DebugRef) { r.IsAddr = true }, nil},
		{"different source position", func(r *ssa.DebugRef) { r.Expr = ast.NewIdent("result") }, nil},
		{"indirect source is no variable assignment", func(r *ssa.DebugRef) { r.Expr = &ast.StarExpr{Star: r.Expr.Pos(), X: ast.NewIdent("result")} }, nil},
		{"missing native expression", func(r *ssa.DebugRef) { r.Expr = nil }, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry()(result error){defer func(){result=nil}();return}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry")
				r := gotoolchain.SSAObjectWriteRequest{Function: entry.AnonFuncs[0], Object: entry.Signature.Results().At(0)}
				before, err := v.ObjectWritten(ctx, r)
				if err != nil || !before.Proven {
					t.Fatalf("store precondition=(%+v,%v), want proven and nil", before, err)
				}
				changed := 0
				for _, block := range r.Function.Blocks {
					for _, instruction := range block.Instrs {
						if reference, ok := instruction.(*ssa.DebugRef); ok && reference.Object() == r.Object && !reference.IsAddr {
							row.change(reference)
							changed++
						}
					}
				}
				if changed != 1 {
					t.Fatalf("native reference mutations=%d, want 1", changed)
				}
				observation, err := v.ObjectWritten(ctx, r)
				if observation != (gotoolchain.SSARelationObservation{}) || (row.want == nil && err != nil) || (row.want != nil && !errors.Is(err, row.want)) {
					t.Fatalf("changed pair=(%+v,%v), want zero and %v", observation, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
