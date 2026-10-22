package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSANilBoxedReturnsPreserveLoanAndConsumerAuthority(t *testing.T) {
	t.Parallel()
	source := `package sample;type E struct{};func(*E)Error()string{return "error"};func Entry(flag bool)error{if flag{return (*E)(nil)};return (*E)(nil)}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	var foreignFrame *ssa.Function
	err := foreign.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: foreign.Units[0], Use: func(_ context.Context, v gotoolchain.SSAUnitView) error {
		foreignFrame = v.Package.Func("Entry")
		return nil
	}})
	if err != nil {
		t.Fatalf("foreign WithSSAUnit error=%v, want nil", err)
	}
	err = a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		request := gotoolchain.SSANilBoxedReturnRequest{Function: v.Package.Func("Entry"), Interface: types.Universe.Lookup("error").Type().Underlying().(*types.Interface)}
		for _, row := range []struct {
			name   string
			change func(*gotoolchain.SSAUnitView, *gotoolchain.SSANilBoxedReturnRequest)
		}{
			{"absent function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSANilBoxedReturnRequest) { r.Function = nil }},
			{"absent interface", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSANilBoxedReturnRequest) { r.Interface = nil }},
			{"foreign native function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSANilBoxedReturnRequest) { r.Function = foreignFrame }},
			{"absent native loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSANilBoxedReturnRequest) {
				*v = gotoolchain.SSAUnitView{}
			}},
		} {
			view, r := v, request
			row.change(&view, &r)
			refusals := 0
			for returned, err := range view.NilBoxedReturns(ctx, r) {
				if returned != nil || !errors.Is(err, core.ErrGoToolchainContract) {
					t.Fatalf("%s return/refusal=(%v,%v), want nil and %v", row.name, returned, err, core.ErrGoToolchainContract)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("%s refusals=%d, want 1", row.name, refusals)
			}
		}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			n := 0
			for returned, err := range v.NilBoxedReturns(current, request) {
				if returned != nil || !errors.Is(err, want) {
					t.Fatalf("cancelled return/refusal=(%v,%v), want nil and %v", returned, err, want)
				}
				n++
			}
			if n != 1 {
				t.Fatalf("cancelled refusals=%d, want 1", n)
			}
		}
		late, cancelLate := context.WithCancelCause(ctx)
		returns, refusals := 0, 0
		for returned, err := range v.NilBoxedReturns(late, request) {
			if err != nil {
				if returned != nil || !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("late return/refusal=(%v,%v), want nil and closed pipe", returned, err)
				}
				refusals++
			} else {
				returns++
				cancelLate(io.ErrClosedPipe)
			}
		}
		cancelLate(nil)
		if returns != 1 || refusals != 1 {
			t.Fatalf("late returns/refusals=%d/%d, want 1/1", returns, refusals)
		}
		// Mutate the later actual return, preserving the first native prefix.
		count := 0
		for _, block := range request.Function.Blocks {
			for _, instruction := range block.Instrs {
				if returned, ok := instruction.(*ssa.Return); ok {
					count++
					if count == 2 {
						returned.Results[0] = nil
					}
				}
			}
		}
		if count != 2 {
			t.Fatalf("native return precondition=%d, want 2", count)
		}
		returns = 0
		for returned, err := range v.NilBoxedReturns(ctx, request) {
			if err != nil || returned == nil {
				t.Fatalf("stopped return/refusal=(%v,%v), want native prefix and nil", returned, err)
			}
			returns++
			break
		}
		if returns != 1 {
			t.Fatalf("stopped returns=%d, want 1", returns)
		}
		returns, refusals = 0, 0
		for returned, err := range v.NilBoxedReturns(ctx, request) {
			if err != nil {
				if returned != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("malformed future=(%v,%v), want nil and output refusal", returned, err)
				}
				refusals++
			} else {
				returns++
			}
		}
		if returns != 1 || refusals != 1 {
			t.Fatalf("native prefix/refusals=%d/%d, want 1/1", returns, refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
