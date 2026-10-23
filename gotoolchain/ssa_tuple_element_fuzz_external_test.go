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

func nativeTupleSemanticBody(mode uint8) string {
	producer := [4]string{"pair(flag)", "m[key]", "value.(int)", "<-ch"}[(mode>>1)&3]
	return strings.Repeat(`consume(0,true);`, int(mode>>5)) + `n,ok:=` + producer + `;consume(n,ok)`
}

// The complete byte domain selects two result ordinals, four native producer
// kinds, four custody/context states and eight prefix extents. Every bit is read.
func TestSSATupleElementFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(`package sample;func pair(flag bool)(int,bool){return 7,flag};func consume(int,bool){};`)
	for mode := 0; mode < 256; mode++ {
		fmt.Fprintf(&source, `func F%d(flag bool,m map[int]int,key int,value interface{},ch <-chan int){%s};`, mode, nativeTupleSemanticBody(uint8(mode)))
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for mode := 0; mode < 256; mode++ {
			nativeTupleSemanticObservation(t, ctx, v, v.Package.Func("F"+strconv.Itoa(mode)), uint8(mode))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func FuzzSSATupleElementConservesProducerAndRefusal(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 23, 24, 31, 32, 64, 128, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func pair(flag bool)(int,bool){return 7,flag};func consume(int,bool){};func Entry(flag bool,m map[int]int,key int,value interface{},ch <-chan int){`+nativeTupleSemanticBody(mode)+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			nativeTupleSemanticObservation(t, ctx, v, v.Package.Func("Entry"), mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit got=%v, want nil", err)
		}
	})
}

func nativeTupleSemanticObservation(t *testing.T, ctx context.Context, v gotoolchain.SSAUnitView, fn *ssa.Function, mode uint8) {
	t.Helper()
	var element *ssa.Extract
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if current, ok := instruction.(*ssa.Extract); ok && current.Index == int(mode&1) {
				element = current
			}
		}
	}
	if element == nil {
		t.Fatalf("mode=%d fixture element got=%v, want native ordinal=%d", mode, element, mode&1)
	}
	original := element.Tuple
	queryCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	damage := (mode >> 3) & 3
	switch damage {
	case 0:
	case 1:
		switch producer := element.Tuple.(type) {
		case *ssa.Call:
			copy := *producer
			element.Tuple = &copy
		case *ssa.Lookup:
			copy := *producer
			element.Tuple = &copy
		case *ssa.TypeAssert:
			copy := *producer
			element.Tuple = &copy
		case *ssa.UnOp:
			copy := *producer
			element.Tuple = &copy
		default:
			t.Fatalf("mode=%d producer got=%T, want declared native tuple kind", mode, producer)
		}
	case 2:
		element.Index = 2
	case 3:
		cancel(io.ErrClosedPipe)
	}
	got, queryErr := v.TupleElement(queryCtx, gotoolchain.SSATupleElementRequest{Element: element})
	switch damage {
	case 0:
		if queryErr != nil || got.Tuple != original || got.Index != int(mode&1) {
			t.Fatalf("mode=%d tuple got=%+v,%v, want original producer,index=%d,nil", mode, got, queryErr, mode&1)
		}
	case 1, 2:
		if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("mode=%d damaged tuple got=%+v,%v, want zero,output refusal", mode, got, queryErr)
		}
	case 3:
		if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("mode=%d cancelled tuple got=%+v,%v, want zero,cancelled with native cause", mode, got, queryErr)
		}
	}
}
