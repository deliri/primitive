package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAReferrersPreserveContextAndEmptyUseList(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeReferrerSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		value, queryErr := nativeReferrerValue(ctx, view)
		if queryErr != nil || value == nil {
			t.Fatalf("native closure=%v,%v, want nonnil,nil", value, queryErr)
		}
		canceled, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			ctx  context.Context
			want error
		}{{nil, core.ErrNilContext}, {canceled, io.ErrClosedPipe}} {
			refusals := 0
			for instruction, queryErr := range view.Referrers(row.ctx, gotoolchain.SSAReferrerRequest{Instruction: value}) {
				if instruction != nil || !errors.Is(queryErr, row.want) {
					t.Fatalf("context reference refusal=%v,%v, want nil,%v", instruction, queryErr, row.want)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("context reference refusals=%d, want 1", refusals)
			}
		}
		for instruction, queryErr := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: view.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentLexical}) {
			if queryErr != nil {
				return queryErr
			}
			call, ok := instruction.(*ssa.Call)
			if !ok || call.Common().StaticCallee() != view.Package.Func("sink") {
				continue
			}
			if uses := call.Referrers(); uses != nil && len(*uses) != 0 {
				t.Fatalf("unused native call uses=%v, want absent or empty", uses)
			}
			for use, queryErr := range view.Referrers(ctx, gotoolchain.SSAReferrerRequest{Instruction: call}) {
				t.Fatalf("unused native call yielded=%v,%v, want no observations", use, queryErr)
			}
			return nil
		}
		t.Fatal("native sink instruction=nil, want actual unused call result")
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func FuzzSSAReferrersConserveNativeUses(f *testing.F) {
	f.Add(uint8(0), false, false)
	f.Add(uint8(15), true, false)
	f.Add(uint8(7), true, true)
	f.Fuzz(func(t *testing.T, extent uint8, untaken, canceled bool) {
		population := 1 + int(extent%16)
		dead := ""
		if untaken {
			dead = "if false{f()};"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(int){};func Entry(x int){f:=func(){sink(x)};`+dead+strings.Repeat("f();", population)+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			value, queryErr := nativeReferrerValue(ctx, view)
			if queryErr != nil || value == nil {
				t.Fatalf("native closure=%v,%v, want nonnil,nil", value, queryErr)
			}
			queryContext, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if canceled {
				cancel(io.ErrClosedPipe)
			}
			uses, refusals := 0, 0
			for instruction, queryErr := range view.Referrers(queryContext, gotoolchain.SSAReferrerRequest{Instruction: value}) {
				if queryErr != nil {
					if !canceled || instruction != nil || !errors.Is(queryErr, io.ErrClosedPipe) {
						t.Fatalf("fuzz reference refusal=%v,%v, canceled=%t", instruction, queryErr, canceled)
					}
					refusals++
					continue
				}
				owned := false
				for _, native := range *value.(ssa.Value).Referrers() {
					owned = owned || native == instruction
				}
				if !owned || instruction.Parent() != value.Parent() {
					t.Fatalf("fuzz native use=%v, want exact Go-owned use in selected function", instruction)
				}
				uses++
			}
			wantUses, wantRefusals := population, 0
			if canceled {
				wantUses, wantRefusals = 0, 1
			}
			if uses != wantUses || refusals != wantRefusals {
				t.Fatalf("fuzz native uses/refusals=%d/%d, want %d/%d", uses, refusals, wantUses, wantRefusals)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
