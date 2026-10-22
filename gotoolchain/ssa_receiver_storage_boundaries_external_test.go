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

func TestSSAReceiverStorageRefusesBrokenBindings(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		change func(*ssa.Call, *ssa.MakeClosure)
	}{
		{"absent callee", func(c *ssa.Call, _ *ssa.MakeClosure) { c.Call.Value = nil }},
		{"typed nil callee", func(c *ssa.Call, _ *ssa.MakeClosure) { c.Call.Value = (*ssa.Function)(nil) }},
		{"typed nil receiver", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Bindings[0] = (*ssa.Parameter)(nil) }},
		{"absent receiver", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Bindings[0] = nil }},
		{"absent closure function", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Fn = nil }},
		{"wrong closure function kind", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Fn = b.Bindings[0] }},
		{"absent binding", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Bindings = nil }},
		{"wrong binding type", func(c *ssa.Call, b *ssa.MakeClosure) { b.Bindings[0] = c.Parent().Params[1] }},
		{"missing target signature", func(_ *ssa.Call, b *ssa.MakeClosure) { b.Fn.(*ssa.Function).Signature = nil }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};func Entry(a *Cell,n int){first:=a.First;first();a.Second()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var calls [2]*ssa.Call
				count := 0
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if c, ok := instruction.(*ssa.Call); ok {
							if count < len(calls) {
								calls[count] = c
							}
							count++
						}
					}
				}
				if count != 2 {
					t.Fatalf("fixture calls=%d, want 2", count)
				}
				binding, ok := calls[0].Call.Value.(*ssa.MakeClosure)
				if !ok || len(binding.Bindings) != 1 {
					t.Fatalf("fixture binding=%T, want one native capture", calls[0].Call.Value)
				}
				row.change(calls[0], binding)
				defer func() {
					if failure := recover(); failure != nil {
						t.Errorf("receiver refusal panicked=%v, want zero proof and output error", failure)
					}
				}()
				got, queryErr := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: calls[0], Right: calls[1]})
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("broken binding=(%+v,%v), want zero proof and %v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAReceiverStorageRequiresLoanAndContext(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};func Entry(a *Cell){a.First();a.Second()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var calls [2]*ssa.Call
		count := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if c, ok := instruction.(*ssa.Call); ok {
					if count < len(calls) {
						calls[count] = c
					}
					count++
				}
			}
		}
		if count != 2 {
			t.Fatalf("fixture calls=%d, want 2", count)
		}
		clone := *calls[0]
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			name        string
			view        gotoolchain.SSAUnitView
			ctx         context.Context
			left, right *ssa.Call
			want        error
		}{
			{"absent loan", gotoolchain.SSAUnitView{}, ctx, calls[0], calls[1], core.ErrGoToolchainContract},
			{"absent left call", v, ctx, nil, calls[1], core.ErrGoToolchainContract},
			{"absent right call", v, ctx, calls[0], nil, core.ErrGoToolchainContract},
			{"cloned call", v, ctx, &clone, calls[1], core.ErrGoToolchainContract},
			{"nil context", v, nil, calls[0], calls[1], core.ErrNilContext},
			{"custom cancellation cause", v, cancelled, calls[0], calls[1], io.ErrClosedPipe},
		} {
			t.Run(row.name, func(t *testing.T) {
				got, queryErr := row.view.SameReceiverStorage(row.ctx, gotoolchain.SSACallReceiverStorageRequest{Left: row.left, Right: row.right})
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("receiver authority=(%+v,%v), want zero proof and %v", got, queryErr, row.want)
				}
			})
		}
		fn.Blocks[0].Index = 99
		got, queryErr := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: calls[0], Right: calls[1]})
		if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("malformed block=(%+v,%v), want zero proof and output refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
