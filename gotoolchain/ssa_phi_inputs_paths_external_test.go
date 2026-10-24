package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAPhiInputsNativeAncestryAndLoopBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		limit        uint8
		want         []int
	}{
		{"diamond source order", `func Entry(flag bool,a,b int)int{value:=a;if flag{value=b};return value}`, 0, []int{1, 2}},
		{"strict ancestor of later block", `func Entry(flag,again bool,a,b int)int{value:=a;if flag{value=b};if again{_=touch(value)};return value}`, 1, []int{2, 3}},
		{"equal block stays neutral", `func Entry(flag bool,a,b int)int{value:=a;if flag{value=b};return value}`, 2, nil},
		{"nondominated limit stays neutral", `func Entry(flag bool,a,b int)int{value:=a;if flag{value=b};return value}`, 3, nil},
		{"loop backedge is not source input", `func Entry(a,n int)int{value:=a;for n>0{value++;n--};return value}`, 0, []int{0}},
		{"nested loop backedge stays excluded", `func Entry(a,n int)int{value:=a;for n>0{for j:=0;j<n;j++{value++};n--};return value}`, 0, []int{0}},
		{"untaken merge stays neutral", `func Entry(flag bool,a,b int)int{if false{value:=a;if flag{value=b};return value};return a}`, 0, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func touch(int)int{return 0};`+row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				merge := nativeReturnedPhi(t, fn)
				request := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
				switch row.limit {
				case 0:
				case 1:
					for _, block := range fn.Blocks {
						for _, instruction := range block.Instrs {
							if _, ok := instruction.(*ssa.Return); ok {
								request.AncestorOf = block
							}
						}
					}
					if request.AncestorOf == nil || request.AncestorOf == merge.Block() {
						t.Fatalf("strict ancestor fixture got=%v, want actual later block", request.AncestorOf)
					}
				case 2:
					request.AncestorOf = merge.Block()
				case 3:
					request.AncestorOf = fn.Blocks[0]
				default:
					t.Fatalf("Phi limit got=%d, want declared case", row.limit)
				}
				seen := 0
				for got, err := range v.PhiInputs(ctx, request) {
					if err != nil || got.Validate() != nil || seen >= len(row.want) || got.Value != fn.Params[row.want[seen]] || got.Block != merge.Block() {
						t.Fatalf("Phi path index=%d got=%+v,%v, want source parameter indices=%v", seen, got, err, row.want)
					}
					seen++
				}
				if seen != len(row.want) {
					t.Fatalf("Phi path source count got=%d, want %d", seen, len(row.want))
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAPhiInputsPreservesConsumerStopAndCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePhiInputsSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		merge := nativeReturnedPhi(t, fn)
		r := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
		query, cancel := context.WithCancelCause(ctx)
		stopped := 0
		v.PhiInputs(query, r)(func(got gotoolchain.SSAPhiInputObservation, err error) bool {
			if stopped != 0 || err != nil || got.Value != fn.Params[1] || got.Block != merge.Block() {
				t.Fatalf("Phi stop got=%+v,%v count=%d, want first source,nil", got, err, stopped)
			}
			stopped++
			cancel(io.ErrClosedPipe)
			return false
		})
		if stopped != 1 {
			t.Fatalf("Phi stop count got=%d, want1", stopped)
		}
		query, cancel = context.WithCancelCause(ctx)
		defer cancel(nil)
		seen, refused := 0, 0
		for got, err := range v.PhiInputs(query, r) {
			if seen == 0 {
				if err != nil || got.Value != fn.Params[1] {
					t.Fatalf("Phi cancel first got=%+v,%v, want first source,nil", got, err)
				}
				seen++
				cancel(io.ErrClosedPipe)
				continue
			}
			if got != (gotoolchain.SSAPhiInputObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("Phi cancelled got=%+v,%v, want zero and both native causes", got, err)
			}
			refused++
		}
		if seen != 1 || refused != 1 {
			t.Fatalf("Phi cancellation got=%d,%d, want1,1", seen, refused)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAPhiInputObservationRequiresCompleteNativeShape(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePhiInputsSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := nativeReturnedPhi(t, fn).Block()
		for _, row := range []struct {
			name  string
			value ssa.Value
			block *ssa.BasicBlock
			want  error
		}{
			{"native pair", fn.Params[1], block, nil}, {"zero is not an input fact", nil, nil, core.ErrGoToolchainOutput},
			{"missing input", nil, block, core.ErrGoToolchainOutput}, {"missing block", fn.Params[1], nil, core.ErrGoToolchainOutput},
			{"typed nil input", (*ssa.Parameter)(nil), block, core.ErrGoToolchainOutput}, {"foreign input parent", v.Package.Func("Other").Params[1], block, core.ErrGoToolchainOutput},
		} {
			got := (gotoolchain.SSAPhiInputObservation{Value: row.value, Block: row.block}).Validate()
			if !errors.Is(got, row.want) {
				t.Fatalf("%s Phi observation got=%v, want %v", row.name, got, row.want)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
