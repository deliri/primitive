package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAReceiverParameterRejectsMalformedCallFacts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode uint8
		body string
		want error
	}{
		{"detached call pointer", 0, `r.Invoke(1)`, core.ErrGoToolchainContract},
		{"missing call block", 1, `r.Invoke(1)`, core.ErrGoToolchainContract},
		{"missing receiver operand", 2, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"missing call value", 3, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"malformed bound receiver", 4, `f:=r.Invoke;f(1)`, core.ErrGoToolchainOutput},
		{"foreign native program", 5, `r.Invoke(1)`, core.ErrGoToolchainContract},
		{"nil bound receiver", 6, `f:=r.Invoke;f(1)`, core.ErrGoToolchainOutput},
		{"nil direct receiver", 7, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"missing static signature", 8, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"typed nil receiver", 9, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"foreign native receiver", 10, `r.Invoke(1)`, core.ErrGoToolchainOutput},
		{"detached receiver instruction", 11, `unused:=func(){_=r};_=unused;r.Invoke(1)`, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};func Entry(r *Cell){`+row.body+`};func Other(r *Cell){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, view, fn)
				r := gotoolchain.SSAReceiverParameterRequest{Call: call, Parameter: fn.Params[0]}
				switch row.mode {
				case 0:
					copied := *call
					r.Call = &copied
				case 1:
					r.Call = &ssa.Call{}
				case 2:
					call.Common().Args = nil
				case 3:
					call.Common().Value = nil
				case 4:
					closure, ok := call.Common().Value.(*ssa.MakeClosure)
					if !ok || len(closure.Bindings) != 1 {
						t.Fatalf("bound closure=%T, want one native receiver binding", call.Common().Value)
					}
					closure.Bindings = nil
				case 5:
					fn.Prog = nil
				case 6:
					closure := call.Common().Value.(*ssa.MakeClosure)
					closure.Bindings[0] = nil
				case 7:
					call.Common().Args[0] = nil
				case 8:
					call.Common().StaticCallee().Signature = nil
				case 9:
					call.Common().Args[0] = (*ssa.UnOp)(nil)
				case 10:
					call.Common().Args[0] = view.Package.Func("Other").Params[0]
				case 11:
					load, ok := call.Common().Args[0].(*ssa.UnOp)
					if !ok {
						t.Fatalf("receiver load=%T, want native cell load", call.Common().Args[0])
					}
					copied := *load
					call.Common().Args[0] = &copied
				default:
					t.Fatalf("malformed receiver mode=%d, want declared boundary", row.mode)
				}
				got, queryErr := view.ReceiverParameter(ctx, r)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
					t.Fatalf("malformed receiver refusal=%+v,%v, want zero,%v", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAReceiverParameterUntakenCallHasNoAuthority(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};func Entry(r *Cell){if false{r.Invoke(1)}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		calls := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok {
					continue
				}
				got, queryErr := view.ReceiverParameter(ctx, gotoolchain.SSAReceiverParameterRequest{Call: call, Parameter: fn.Params[0]})
				if queryErr != nil || got != (gotoolchain.SSARelationObservation{}) {
					t.Fatalf("untaken receiver=%+v,%v, want zero,nil", got, queryErr)
				}
				calls++
			}
		}
		if calls != 1 {
			t.Fatalf("native untaken calls=%d, want 1", calls)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
