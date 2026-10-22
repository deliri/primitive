package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAInstructionsRefuseMalformedNativeLoan(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAInstructionRequest)
		want   error
	}{
		{"absent native view", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAInstructionRequest) { *v = gotoolchain.SSAUnitView{} }, core.ErrGoToolchainContract},
		{"nil native block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Function.Blocks[0] = nil }, core.ErrGoToolchainOutput},
		{"foreign block ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Function.Blocks[0].Index = 99
		}, core.ErrGoToolchainOutput},
		{"nil native instruction", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Function.Blocks[0].Instrs[0] = nil
		}, core.ErrGoToolchainOutput},
		{"typed nil native instruction", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Function.Blocks[0].Instrs[0] = (*ssa.Call)(nil)
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){invoke()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentFunction}
				row.mutate(&v, &r)
				refusals := 0
				for instruction, err := range v.Instructions(ctx, r) {
					if instruction != nil || !errors.Is(err, row.want) {
						t.Fatalf("malformed=(%v,%v), want nil and %v", instruction, err, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("malformed refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAInstructionsBorrowActualAnonymousRootAndImportedAbsence(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "strings";func invoke(){};func Entry(){_=strings.TrimSpace("x");_=func(){go invoke()}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if len(fn.AnonFuncs) != 1 {
			t.Fatalf("anonymous roots=%d, want 1", len(fn.AnonFuncs))
		}
		spawns := 0
		for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn.AnonFuncs[0], Extent: gotoolchain.SSAInstructionExtentFunction}) {
			if err != nil {
				t.Fatalf("anonymous root refusal=%v, want nil", err)
			}
			if _, ok := instruction.(*ssa.Go); ok {
				spawns++
			}
		}
		if spawns != 1 {
			t.Fatalf("anonymous root spawns=%d, want 1", spawns)
		}
		imported := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					callee := call.Common().StaticCallee()
					if callee != nil && callee.Pkg != v.Package {
						if len(callee.Blocks) != 0 {
							t.Fatalf("imported body blocks=%d, want 0", len(callee.Blocks))
						}
						imported++
						for borrowed, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: callee, Extent: gotoolchain.SSAInstructionExtentLexical}) {
							t.Fatalf("imported absence=(%v,%v), want no observations", borrowed, err)
						}
					}
				}
			}
		}
		if imported != 1 {
			t.Fatalf("imported declarations=%d, want 1", imported)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
