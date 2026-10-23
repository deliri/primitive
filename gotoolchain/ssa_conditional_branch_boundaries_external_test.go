package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAConditionalBranchPreservesOpaqueNativeConditions(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Flag bool;func work(){};func consume(bool){};func Named(flag Flag){if flag{work()}};func Equality(a,b bool){if a==b{work()}};func Error(err error){if err==nil{work()}};func Load(flag *bool){if *flag{work()}};func Closure(flag bool){_=func(){if flag{work()}}};func ConstantTrue(){if true{work()}};func ConstantFalse(){if false{work()}};func Hoisted(flag,other bool){a:=!flag;consume(a);if other{if a{work()}}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for _, name := range []string{"Named", "Equality", "Error", "Load", "Closure", "ConstantTrue", "ConstantFalse"} {
			fn := v.Package.Func(name)
			if name == "Closure" {
				fn = fn.AnonFuncs[0]
			}
			block := fn.Blocks[0]
			branch := block.Instrs[len(block.Instrs)-1].(*ssa.If)
			got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
			if queryErr != nil || got.Condition != branch.Cond || got.WhenTrue != block.Succs[0] || got.WhenFalse != block.Succs[1] || got.Validate() != nil {
				t.Fatalf("opaque %s got=%+v,%v, want actual condition and original edges", name, got, queryErr)
			}
			if name == "ConstantTrue" || name == "ConstantFalse" {
				untaken := block.Succs[1]
				if name == "ConstantFalse" {
					untaken = block.Succs[0]
				}
				absent, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: untaken})
				if absent != (gotoolchain.SSAConditionalBranchObservation{}) || queryErr != nil {
					t.Fatalf("untaken %s got=%+v,%v, want zero,nil", name, absent, queryErr)
				}
			}
		}
		fn := v.Package.Func("Hoisted")
		block := fn.Blocks[0].Succs[0]
		got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
		if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != block.Succs[1] || got.WhenFalse != block.Succs[0] {
			t.Fatalf("hoisted got=%+v,%v, want dominating flag and reversed edges", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAConditionalBranchRefusesForwardAndSiblingDefinitions(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		sibling bool
	}{{"forward", false}, {"sibling", true}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource+`;func flag()bool{return true};func Sibling(other bool){if other{_=flag()}else{if flag(){yes()}}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				block := fn.Blocks[0]
				if row.sibling {
					fn = v.Package.Func("Sibling")
					root := fn.Blocks[0]
					block = root.Succs[1]
					branch := block.Instrs[len(block.Instrs)-1].(*ssa.If)
					for _, instruction := range root.Succs[0].Instrs {
						if call, ok := instruction.(*ssa.Call); ok {
							branch.Cond = call
							break
						}
					}
				} else {
					branch := block.Instrs[len(block.Instrs)-1].(*ssa.If)
					outer := branch.Cond.(*ssa.BinOp)
					inner := outer.X.(*ssa.UnOp)
					before, after := -1, -1
					for index, instruction := range block.Instrs {
						if instruction == inner {
							before = index
						}
						if instruction == outer {
							after = index
						}
					}
					if before < 0 || after <= before {
						t.Fatalf("native definition order got=%d/%d, want inner before outer", before, after)
					}
					block.Instrs[before], block.Instrs[after] = block.Instrs[after], block.Instrs[before]
				}
				got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
				if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("definition custody got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAConditionalBranchSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		request gotoolchain.SSAConditionalBranchRequest
		wantErr error
	}{
		{"missing block", gotoolchain.SSAConditionalBranchRequest{}, core.ErrGoToolchainContract},
		{"parentless block", gotoolchain.SSAConditionalBranchRequest{Block: &ssa.BasicBlock{}}, core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			if err := row.request.Validate(); !errors.Is(err, row.wantErr) {
				t.Fatalf("request got=%v, want %v", err, row.wantErr)
			}
		})
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		if err := (gotoolchain.SSAConditionalBranchRequest{Block: block}).Validate(); err != nil {
			t.Fatalf("native request got=%v, want nil", err)
		}
		// Exhaust the complete three-field presence domain: zero is neutral,
		// all present is positive, every partial projection is a typed refusal.
		for fields := 0; fields < 8; fields++ {
			var observation gotoolchain.SSAConditionalBranchObservation
			if fields&1 != 0 {
				observation.Condition = fn.Params[0]
			}
			if fields&2 != 0 {
				observation.WhenTrue = block.Succs[0]
			}
			if fields&4 != 0 {
				observation.WhenFalse = block.Succs[1]
			}
			var wantErr error
			if fields != 0 && fields != 7 {
				wantErr = core.ErrGoToolchainOutput
			}
			gotErr := observation.Validate()
			if (wantErr == nil && gotErr != nil) || (wantErr != nil && !errors.Is(gotErr, wantErr)) {
				t.Fatalf("field presence=%03b got=%+v,%v, want %v", fields, observation, gotErr, wantErr)
			}
		}
		for _, row := range []struct {
			name    string
			in      gotoolchain.SSAConditionalBranchObservation
			wantErr error
		}{
			{"typed nil condition", gotoolchain.SSAConditionalBranchObservation{Condition: (*ssa.Call)(nil), WhenTrue: block.Succs[0], WhenFalse: block.Succs[1]}, core.ErrGoToolchainOutput},
			{"nonboolean condition", gotoolchain.SSAConditionalBranchObservation{Condition: ssa.NewConst(nil, types.Typ[types.Int]), WhenTrue: block.Succs[0], WhenFalse: block.Succs[1]}, core.ErrGoToolchainOutput},
			{"parentless true edge", gotoolchain.SSAConditionalBranchObservation{Condition: fn.Params[0], WhenTrue: &ssa.BasicBlock{}, WhenFalse: block.Succs[1]}, core.ErrGoToolchainOutput},
			{"foreign false edge", gotoolchain.SSAConditionalBranchObservation{Condition: fn.Params[0], WhenTrue: block.Succs[0], WhenFalse: v.Package.Func("Other").Blocks[0]}, core.ErrGoToolchainOutput},
		} {
			if err := row.in.Validate(); !errors.Is(err, row.wantErr) {
				t.Fatalf("%s observation got=%+v,%v, want %v", row.name, row.in, err, row.wantErr)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
