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

// Exhaust the structural query domain: four possible blocks at each endpoint
// and four custody/context conditions. These are independent native functions,
// so a damaged loan cannot leak into the next case.
func TestSSABlockDominanceFiniteLoanDomain(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(`package sample;func left(){};func right(){};func join(){};`)
	for mode := 0; mode < 64; mode++ {
		fmt.Fprintf(&source, `func Entry%d(flag bool){if flag{left()}else{right()};join()};`, mode)
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for mode := 0; mode < 64; mode++ {
			nativeBlockDominanceSemanticObservation(t, ctx, v, v.Package.Func("Entry"+strconv.Itoa(mode)), uint8(mode))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error got=%v, want nil", err)
	}
}

func FuzzSSABlockDominanceConservesNativeRelationAndRefusal(f *testing.F) {
	for _, mode := range []uint8{0, 3, 5, 6, 15, 16, 32, 48, 64, 128, 192, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		condition := [4]string{"flag", "true", "false", "!flag"}[mode>>6]
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func left(){};func right(){};func join(){};func Entry(flag bool){if `+condition+`{left()}else{right()};join()}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			nativeBlockDominanceSemanticObservation(t, ctx, v, v.Package.Func("Entry"), mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error got=%v, want nil", err)
		}
	})
}

func nativeBlockDominanceSemanticObservation(t *testing.T, ctx context.Context, v gotoolchain.SSAUnitView, fn *ssa.Function, mode uint8) {
	t.Helper()
	entry := fn.Blocks[0]
	if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
		t.Fatalf("native mode=%d diamond got absent, want two arms and shared join", mode)
	}
	blocks := [4]*ssa.BasicBlock{entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
	before, after := int(mode&3), int((mode>>2)&3)
	r := gotoolchain.SSABlockDominanceRequest{Dominator: blocks[before], Block: blocks[after]}
	if err := r.Validate(); err != nil {
		t.Fatalf("native mode=%d request got=%v, want admitted same-function shape", mode, err)
	}
	damage := (mode >> 4) & 3
	queryCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	switch damage {
	case 0:
	case 1:
		copied := *r.Dominator
		r.Dominator = &copied
	case 2:
		copied := *r.Block
		r.Block = &copied
	case 3:
		cancel(io.ErrClosedPipe)
	}
	got, queryErr := v.BlockDominates(queryCtx, r)
	if damage == 1 || damage == 2 {
		if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("mode=%d detached relation got=%+v,%v, want zero,%v", mode, got, queryErr, core.ErrGoToolchainOutput)
		}
		return
	}
	if damage == 3 {
		if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("mode=%d cancelled relation got=%+v,%v, want zero,cancelled with native cause", mode, got, queryErr)
		}
		return
	}
	// This source-level diamond oracle does not call the queried native method.
	want := before == 0 || before == after
	if queryErr != nil || got.Proven != want {
		t.Fatalf("mode=%d relation got=%+v,%v, want proven=%t,nil", mode, got, queryErr, want)
	}
}
