package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSAInstructionsConserveLexicalSource(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 4, 8, 16, 25, 31} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		body := "go invoke();"
		want := 1
		if mode&1 != 0 {
			body = "if false{" + body + "};"
			want = 0
		}
		extent := gotoolchain.SSAInstructionExtentFunction
		if mode&2 != 0 {
			extent = gotoolchain.SSAInstructionExtentLexical
		}
		if mode&4 != 0 {
			body = "unused:=func(){" + body + "};_=unused;"
			if mode&2 == 0 {
				want = 0
			}
		}
		if mode&8 != 0 {
			body += "defer invoke();"
		}
		if mode&16 != 0 {
			body += "helper();"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func helper(){go invoke()};func Entry(){`+body+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			got := 0
			for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: extent}) {
				if err != nil || instruction == nil {
					t.Fatalf("source observation=(%v,%v), want native instruction and nil", instruction, err)
				}
				if _, ok := instruction.(*ssa.Go); ok {
					got++
				}
			}
			if got != want {
				t.Fatalf("mode %d native spawn conservation=%d, want %d", mode, got, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
