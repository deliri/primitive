package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAMethodReceiverRefusesImpossibleNativeCallLayouts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		damage     uint8
	}{
		{"direct method baseline", `r.Method()`, 0},
		{"bound method baseline", `method:=r.Method;method()`, 0},
		{"extra direct method argument", `r.Method()`, 1},
		{"extra bound method argument", `method:=r.Method;method()`, 1},
		{"cloned source method callable", `r.Method()`, 2},
		{"foreign source method program", `r.Method()`, 3},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Receiver struct{};func(*Receiver)Method(){};func Entry(r *Receiver){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeCalleeCall(t, fn)
				got, err := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if err != nil || got.Method == nil || got.Method.Name() != "Method" || got.Receiver != fn.Params[0] || got.Validate() != nil {
					t.Fatalf("layout baseline got=%+v,%v, want actual method/receiver,nil", got, err)
				}
				if row.damage == 0 {
					return nil
				}
				switch row.damage {
				case 1:
					call.Common().Args = append(call.Common().Args, fn.Params[0])
				case 2:
					original := call.Common().Value.(*ssa.Function)
					copy := *original
					call.Common().Value = &copy
				case 3:
					call.Common().StaticCallee().Prog = ssa.NewProgram(v.Program.Fset, 0)
				}
				got, err = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("impossible method layout got=%+v,%v, want zero,output", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("method layout unit got=%v, want nil", err)
			}
		})
	}
}
