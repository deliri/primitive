package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAObjectWrittenPreservesNativeLoanAndRefusal(t *testing.T) {
	t.Parallel()
	source := `package sample;func Entry()(result error){defer func(){result=nil}();return}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	var foreignFrame *ssa.Function
	var foreignObject *types.Var
	err := foreign.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: foreign.Units[0], SourceValues: true, Use: func(_ context.Context, v gotoolchain.SSAUnitView) error {
		foreignFrame = v.Package.Func("Entry").AnonFuncs[0]
		foreignObject = v.Package.Func("Entry").Signature.Results().At(0)
		return nil
	}})
	if err != nil {
		t.Fatalf("foreign WithSSAUnit error=%v, want nil", err)
	}
	err = a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry")
		r := gotoolchain.SSAObjectWriteRequest{Function: entry.AnonFuncs[0], Object: entry.Signature.Results().At(0)}
		for _, row := range []struct {
			name   string
			change func(*gotoolchain.SSAUnitView, *gotoolchain.SSAObjectWriteRequest)
		}{
			{"absent frame", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) { r.Function = nil }},
			{"absent variable", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) { r.Object = nil }},
			{"foreign frame", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) { r.Function = foreignFrame }},
			{"foreign variable", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) { r.Object = foreignObject }},
			{"absent source package", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) {
				r.Object = types.NewVar(token.NoPos, nil, "result", types.Typ[types.Int])
			}},
			{"absent variable type", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) {
				r.Object = types.NewVar(token.NoPos, v.Source.Types, "result", nil)
			}},
			{"field is no local variable", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAObjectWriteRequest) {
				r.Object = types.NewField(token.NoPos, v.Source.Types, "result", types.Typ[types.Int], false)
			}},
			{"absent native loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAObjectWriteRequest) { *v = gotoolchain.SSAUnitView{} }},
		} {
			view, request := v, r
			row.change(&view, &request)
			observation, err := view.ObjectWritten(ctx, request)
			if observation != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("%s observation/refusal=(%+v,%v), want zero and %v", row.name, observation, err, core.ErrGoToolchainContract)
			}
		}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			observation, err := v.ObjectWritten(current, r)
			if observation != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, want) {
				t.Fatalf("cancelled observation/refusal=(%+v,%v), want zero and %v", observation, err, want)
			}
		}
		r.Function.Blocks[0].Index = 99
		observation, err := v.ObjectWritten(ctx, r)
		if observation != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("malformed root=(%+v,%v), want zero and %v", observation, err, core.ErrGoToolchainOutput)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
