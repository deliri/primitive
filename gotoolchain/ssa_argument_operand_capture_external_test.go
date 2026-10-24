package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentOperandRefusesDamagedCaptureCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
	}{{"cloned native store", 0}, {"foreign stored value", 1}, {"missing stored value", 2}, {"foreign store address", 3}, {"foreign native referrer", 4}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(r *Runner){_=func(){_=r};sink(r)};func Other(r *Runner){_=func(){_=r};sink(r)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				other := v.Package.Func("Other")
				box := nativeArgumentOperandSink(t, fn).Common().Args[0].(*ssa.MakeInterface)
				read := box.X.(*ssa.UnOp)
				cell := read.X.(*ssa.Alloc)
				refs := cell.Referrers()
				ordinal := -1
				var store *ssa.Store
				for i, ref := range *refs {
					if candidate, ok := ref.(*ssa.Store); ok && candidate.Addr == cell {
						ordinal = i
						store = candidate
					}
				}
				if store == nil {
					t.Fatalf("capture fixture got no initial store, want native parameter initialization")
				}
				request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: box, Argument: fn.Params[0]}
				before, err := v.ArgumentOperand(ctx, request)
				if err != nil || !before.Proven {
					t.Fatalf("capture baseline got=%+v,%v, want proven,nil", before, err)
				}
				switch row.mode {
				case 0:
					copy := *store
					(*refs)[ordinal] = &copy
				case 1:
					store.Val = other.Params[0]
				case 2:
					store.Val = nil
				case 3:
					foreign := nativeArgumentOperandSink(t, other).Common().Args[0].(*ssa.MakeInterface).X.(*ssa.UnOp).X
					store.Addr = foreign
				case 4:
					foreign := nativeArgumentOperandSink(t, other)
					(*refs)[ordinal] = foreign
				}
				got, err := v.ArgumentOperand(ctx, request)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("capture custody got=%+v,%v, want zero,output refusal", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
