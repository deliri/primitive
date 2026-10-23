package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAMethodReceiverPreservesBoundCaptureContract(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(gotoolchain.SSAUnitView, *ssa.Function, *ssa.MakeClosure)
	}{
		{"missing bound target", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) { c.Fn = nil }},
		{"typed nil bound target", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) { c.Fn = (*ssa.Function)(nil) }},
		{"missing binding", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) { c.Bindings = nil }},
		{"nil binding", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) { c.Bindings[0] = nil }},
		{"typed nil binding", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) {
			c.Bindings[0] = (*ssa.Parameter)(nil)
		}},
		{"foreign binding", func(v gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) {
			c.Bindings[0] = v.Package.Func("Other").Params[0]
		}},
		{"wrong native binding type", func(_ gotoolchain.SSAUnitView, fn *ssa.Function, c *ssa.MakeClosure) { c.Bindings[0] = fn.Params[1] }},
		{"missing native capture slot", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) {
			c.Fn.(*ssa.Function).FreeVars = nil
		}},
		{"nil native capture slot", func(_ gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) {
			c.Fn.(*ssa.Function).FreeVars[0] = nil
		}},
		{"unbound target cannot borrow a capture", func(v gotoolchain.SSAUnitView, _ *ssa.Function, c *ssa.MakeClosure) { c.Fn = v.Package.Func("plain") }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample,text string){bound:=r.M;_=bound(r)};func Other(r *sample){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, v, fn)
				closure, ok := call.Common().Value.(*ssa.MakeClosure)
				if !ok || len(closure.Bindings) != 1 {
					t.Fatalf("bound fixture=%v bindings available=%t, want actual one-capture method", call, ok)
				}
				got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if queryErr != nil || got.Receiver != fn.Params[0] || got.Method == nil {
					t.Fatalf("bound precondition=%+v,%v, want actual receiver,nil", got, queryErr)
				}
				row.mutate(v, fn, closure)
				got, queryErr = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("bound refusal=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAMethodReceiverUntakenCallIsNeutral(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){if false{_=r.M(r)}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		selected := 0
		for _, block := range v.Package.Func("Entry").Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok {
					continue
				}
				got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || queryErr != nil {
					t.Fatalf("untaken method=%+v,%v, want no method authority,nil", got, queryErr)
				}
				selected++
			}
		}
		if selected != 1 {
			t.Fatalf("untaken native calls=%d, want 1", selected)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAMethodReceiverRejectsWrongNativeOperandType(t *testing.T) {
	t.Parallel()
	for _, invoke := range []bool{false, true} {
		name, parameters, body := "direct method", `r *sample,text string`, `_=r.M(r)`
		if invoke {
			name, parameters, body = "interface method", `r interface{M(*sample)int},text string,other *sample`, `_=r.M(other)`
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(`+parameters+`){`+body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, v, fn)
				got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if queryErr != nil || got.Method == nil || got.Receiver != fn.Params[0] {
					t.Fatalf("method operand precondition=%+v,%v, want actual receiver,nil", got, queryErr)
				}
				if invoke {
					call.Common().Value = fn.Params[1]
				} else {
					call.Common().Args[0] = fn.Params[1]
				}
				got, queryErr = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("wrong operand type=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
