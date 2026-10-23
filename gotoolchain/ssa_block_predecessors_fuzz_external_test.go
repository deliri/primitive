package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSABlockPredecessorsFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(`package sample;func work(){};func left(){};func right(){};func join(){};`)
	for mode := 0; mode < 256; mode++ {
		fmt.Fprintf(&source, `func F%d(flag bool){%sif flag{left()}else{right()};join()};`, mode, strings.Repeat(`work();`, mode>>4))
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for mode := 0; mode < 256; mode++ {
			nativePredecessorSemanticObservation(t, ctx, v, v.Package.Func("F"+strconv.Itoa(mode)), uint8(mode))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func FuzzSSABlockPredecessorsConserveNativeIngress(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 4, 7, 8, 11, 12, 15, 16, 32, 64, 128, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func left(){};func right(){};func join(){};func Entry(flag bool){`+strings.Repeat(`work();`, int(mode>>4))+`if flag{left()}else{right()};join()}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			nativePredecessorSemanticObservation(t, ctx, v, v.Package.Func("Entry"), mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit got=%v, want nil", err)
		}
	})
}

func nativePredecessorSemanticObservation(t *testing.T, ctx context.Context, v gotoolchain.SSAUnitView, fn *ssa.Function, mode uint8) {
	t.Helper()
	entry := fn.Blocks[0]
	if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
		t.Fatalf("mode=%d diamond got=%v, want two arms and shared join", mode, entry.Succs)
	}
	left, right := entry.Succs[0], entry.Succs[1]
	join := left.Succs[0]
	blocks := [4]*ssa.BasicBlock{entry, left, right, join}
	selected := int(mode & 3)
	r := gotoolchain.SSABlockLinkRequest{Block: blocks[selected], Kind: gotoolchain.SSABlockLinkPredecessors}
	damage := (mode >> 2) & 3
	queryCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	switch damage {
	case 0:
	case 1:
		copy := *r.Block
		r.Block = &copy
	case 2:
		r.Block.Preds = append([]*ssa.BasicBlock{nil}, r.Block.Preds...)
	case 3:
		cancel(io.ErrClosedPipe)
	}
	seen, refused := 0, 0
	for predecessor, queryErr := range v.BlockLinks(queryCtx, r) {
		if damage != 0 {
			wantErr := error(core.ErrGoToolchainOutput)
			if damage == 3 {
				wantErr = context.Canceled
			}
			if predecessor != nil || !errors.Is(queryErr, wantErr) || damage == 3 && !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("mode=%d refusal got=%v,%v, want nil,%v with native cause", mode, predecessor, queryErr, wantErr)
			}
			refused++
			continue
		}
		var want *ssa.BasicBlock
		switch selected {
		case 0:
		case 1, 2:
			if seen == 0 {
				want = entry
			}
		case 3:
			if seen == 0 {
				want = left
			} else if seen == 1 {
				want = right
			}
		}
		if queryErr != nil || want == nil || predecessor != want {
			t.Fatalf("mode=%d predecessor[%d] got=%v,%v, want %v,nil", mode, seen, predecessor, queryErr, want)
		}
		seen++
	}
	wantCount := [4]int{0, 1, 1, 2}[selected]
	if damage != 0 {
		if seen != 0 || refused != 1 {
			t.Fatalf("mode=%d stream got values=%d refusals=%d, want zero and one", mode, seen, refused)
		}
		return
	}
	if seen != wantCount || refused != 0 {
		t.Fatalf("mode=%d stream got values=%d refusals=%d, want %d and zero", mode, seen, refused, wantCount)
	}
}
