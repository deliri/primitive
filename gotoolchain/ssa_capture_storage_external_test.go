package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
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
		{"nonfirst capture keeps native binding index", `fn:=func(){consume(other);observe(root)};fn()`, true},
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
		{"store after captured read cannot establish prior storage", `var copy *Record;fn:=func(){observe(copy)};fn();copy=root`, false},
		{"conditional store cannot dominate capture", `var copy *Record;if flag{copy=root};fn:=func(){observe(copy)};fn()`, false},
		{"child can escape captured cell address", `fn:=func(){escape(&root);observe(root)};fn()`, false},
		{"captured pointer representation change", `copy:=(*Alias)(root);fn:=func(){observe((*Record)(copy))};fn()`, true},
		{"native nil does not name storage", `root=nil;observe(root)`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{Count int};type Alias Record;func observe(*Record){};func escape(**Record){};func consume(*Record){};func F(root,other *Record,flag bool){`+row.body+`}`)
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

func TestSSAStorageRefusesClonedNativeOperands(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{Count int};var Slot *Record;func observe(*Record){};func observeSlot(*int){};func F(root *Record){_=Slot;fn:=func(){observe(root)};fn();observeSlot(&root.Count)}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		parent := view.Program.FuncValue(unit.Types.Scope().Lookup("F").(*types.Func))
		child := parent.AnonFuncs[0]
		parameter := *parent.Params[0]
		capture := *child.FreeVars[0]
		global := *view.Package.Members["Slot"].(*ssa.Global)
		var load *ssa.UnOp
		for _, block := range child.Blocks {
			for _, instruction := range block.Instrs {
				if value, ok := instruction.(*ssa.UnOp); ok {
					if _, pointer := value.Type().Underlying().(*types.Pointer); pointer {
						clone := *value
						load = &clone
						break
					}
				}
			}
		}
		var address *ssa.FieldAddr
		for _, block := range parent.Blocks {
			for _, instruction := range block.Instrs {
				if value, ok := instruction.(*ssa.FieldAddr); ok {
					clone := *value
					address = &clone
					break
				}
			}
		}
		if load == nil || address == nil {
			t.Fatal("native fixture omitted pointer load or field address")
		}
		for _, row := range []struct {
			name  string
			value ssa.Value
		}{
			{"parameter clone", &parameter}, {"capture clone", &capture}, {"global clone", &global}, {"load clone", load}, {"field address clone", address},
		} {
			t.Run(row.name, func(t *testing.T) {
				got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: row.value, Right: row.value})
				if !errors.Is(err, core.ErrGoToolchainContract) || got != (gotoolchain.SSARelationObservation{}) {
					t.Fatalf("cloned native storage got%+v/%v wantzero/%v", got, err, core.ErrGoToolchainContract)
				}
			})
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
