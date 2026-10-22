package gotoolchain_test

import (
	"context"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSAStorageBorrowsStableNativeCaptureCells(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"native parameter copy", `copy:=root;observe(copy)`, true},
		{"captured original pointer", `fn:=func(){observe(root)};fn()`, true},
		{"captured pointer copy", `copy:=root;fn:=func(){observe(copy)};fn()`, true},
		{"nested native capture binding", `outer:=func(){inner:=func(){observe(root)};inner()};outer()`, true},
		{"different captured pointer", `copy:=other;fn:=func(){observe(copy)};fn()`, false},
		{"parent replaces captured pointer", `fn:=func(){observe(root)};root=other;fn()`, false},
		{"child replaces captured pointer", `fn:=func(){root=other;observe(root)};fn()`, false},
		{"child writes after observed load", `fn:=func(){observe(root);root=other};fn()`, false},
		{"sibling can replace shared cell", `read:=func(){observe(root)};write:=func(){root=other};write();read()`, false},
		{"nested closure can replace shared cell", `outer:=func(){write:=func(){root=other};write();observe(root)};outer()`, false},
		{"captured address escapes to unknown callee", `escape(&root);fn:=func(){observe(root)};fn()`, false},
		{"conditional receiver replacement", `if flag{root=other};observe(root)`, false},
		{"native nil does not name storage", `root=nil;observe(root)`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{Count int};func observe(*Record){};func escape(**Record){};func F(root,other *Record,flag bool){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				parent := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
				visits := 0
				var visit func(*ssa.Function) error
				visit = func(frame *ssa.Function) error {
					for _, block := range frame.Blocks {
						for _, instruction := range block.Instrs {
							call, ok := instruction.(*ssa.Call)
							if !ok || call.Common().StaticCallee() == nil || call.Common().StaticCallee().Name() != "observe" {
								continue
							}
							got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: parent.Params[0], Right: call.Common().Args[0]})
							if err != nil || got.Validate() != nil || got.Proven != row.want {
								t.Fatalf("native capture storage got%+v/%v want%t/nil", got, err, row.want)
							}
							visits++
						}
					}
					for _, child := range frame.AnonFuncs {
						if err := visit(child); err != nil {
							return err
						}
					}
					return nil
				}
				if err := visit(parent); err != nil {
					return err
				}
				if visits != 1 {
					t.Fatalf("native observed call operands got%d want1", visits)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
