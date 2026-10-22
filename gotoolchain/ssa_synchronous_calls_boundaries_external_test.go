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

func TestSSASynchronousCallsConserveNativeLoanAndConsumerAuthority(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;import "strings";func invoke(){};func Entry(){invoke();invoke();_=strings.TrimSpace("x")}`)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	unit := analysis.Units[0]
	var foreignFrame *ssa.Function
	foreignErr := foreign.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: foreign.Units[0], Use: func(_ context.Context, v gotoolchain.SSAUnitView) error {
		foreignFrame = v.Package.Func("Entry")
		return nil
	}})
	if foreignErr != nil {
		t.Fatalf("WithSSAUnit(foreign) error = %v, want nil", foreignErr)
	}
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := gotoolchain.SSASynchronousCallRequest{Function: view.Package.Func("Entry"), Timing: gotoolchain.SSACallTimingSynchronous}
		for _, row := range []struct {
			name   string
			change func(*gotoolchain.SSASynchronousCallRequest, *gotoolchain.SSAUnitView)
		}{
			{"absent frame", func(r *gotoolchain.SSASynchronousCallRequest, _ *gotoolchain.SSAUnitView) { r.Function = nil }},
			{"unknown timing", func(r *gotoolchain.SSASynchronousCallRequest, _ *gotoolchain.SSAUnitView) {
				r.Timing = gotoolchain.SSACallTimingUnknown
			}},
			{"invalid timing", func(r *gotoolchain.SSASynchronousCallRequest, _ *gotoolchain.SSAUnitView) {
				r.Timing = gotoolchain.SSACallTiming(255)
			}},
			{"foreign analysis frame", func(r *gotoolchain.SSASynchronousCallRequest, _ *gotoolchain.SSAUnitView) { r.Function = foreignFrame }},
			{"absent native view", func(_ *gotoolchain.SSASynchronousCallRequest, v *gotoolchain.SSAUnitView) {
				*v = gotoolchain.SSAUnitView{}
			}},
		} {
			copyRequest, copyView := request, view
			row.change(&copyRequest, &copyView)
			refusals := 0
			for call, err := range copyView.SynchronousCalls(ctx, copyRequest) {
				if call != nil || !errors.Is(err, core.ErrGoToolchainContract) {
					t.Fatalf("%s call/refusal=(%v, %v), want nil and %v", row.name, call, err, core.ErrGoToolchainContract)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("%s refusal count=%d, want 1", row.name, refusals)
			}
		}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			refusals := 0
			for call, err := range view.SynchronousCalls(current, request) {
				if call != nil || !errors.Is(err, want) {
					t.Fatalf("cancelled call/refusal=(%v, %v), want nil and %v", call, err, want)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("cancelled refusal count=%d, want 1", refusals)
			}
		}
		afterFirst, cancelFirst := context.WithCancelCause(ctx)
		calls, refusals := 0, 0
		for call, err := range view.SynchronousCalls(afterFirst, request) {
			if err != nil {
				if call != nil || !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("late call/refusal=(%v, %v), want nil and closed pipe", call, err)
				}
				refusals++
				continue
			}
			calls++
			cancelFirst(io.ErrClosedPipe)
		}
		cancelFirst(nil)
		if calls != 1 || refusals != 1 {
			t.Fatalf("late cancellation calls/refusals=%d/%d, want 1/1", calls, refusals)
		}
		stopped, cancelStop := context.WithCancelCause(ctx)
		calls = 0
		for call, err := range view.SynchronousCalls(stopped, request) {
			if err != nil || call == nil {
				t.Fatalf("stopped call/refusal=(%v, %v), want first call and nil", call, err)
			}
			calls++
			cancelStop(io.ErrClosedPipe)
			break
		}
		cancelStop(nil)
		if calls != 1 {
			t.Fatalf("refused consumer calls=%d, want 1", calls)
		}
		// Imported declarations are real members of this native program with no
		// admitted source body. An empty stream grants no observed call authority.
		var target *types.Func
		for _, imported := range unit.Types.Imports() {
			if imported.Path() == "strings" {
				target = imported.Scope().Lookup("TrimSpace").(*types.Func)
			}
		}
		imported := view.Program.FuncValue(target)
		observed := 0
		for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: imported, Timing: gotoolchain.SSACallTimingSynchronous}) {
			t.Fatalf("imported source absence=(%v, %v), want no calls", call, err)
			observed++
		}
		if observed != 0 {
			t.Fatalf("imported source observations=%d, want 0", observed)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit(authority) error = %v, want nil", err)
	}
}
func TestSSASynchronousCallsObserveBoundNativeTarget(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){call:=r.Invoke;call()}`)
	unit := analysis.Units[0]
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		root := view.Package.Func("Entry")
		target := types.NewMethodSet(root.Params[0].Type()).Lookup(nil, "Invoke").Obj()
		actual := view.Program.FuncValue(target.(*types.Func))
		calls, adapters := 0, 0
		for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: root, Timing: gotoolchain.SSACallTimingImmediate}) {
			if err != nil {
				t.Fatalf("bound native call error = %v, want nil", err)
			}
			if callee := call.Common().StaticCallee(); callee != nil && callee.Object() == target {
				if callee == actual {
					calls++
				} else {
					if callee.Syntax() != nil || len(callee.FreeVars) != 1 {
						t.Fatalf("native bound adapter=%v, want one borrowed receiver capture", callee)
					}
					adapters++
				}
			}
		}
		if calls != 1 || adapters != 1 {
			t.Fatalf("bound native method/adapter calls=%d/%d, want 1/1", calls, adapters)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit(bound method) error = %v, want nil", err)
	}
}

func TestSSASynchronousCallsRefuseMalformedNativeInstructions(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*ssa.Function)
	}{
		{"nil block", func(fn *ssa.Function) { fn.Blocks[0] = nil }},
		{"foreign block ordinal", func(fn *ssa.Function) { fn.Blocks[0].Index = 99 }},
		{"nil instruction", func(fn *ssa.Function) { fn.Blocks[0].Instrs[0] = nil }},
		{"typed nil instruction", func(fn *ssa.Function) { fn.Blocks[0].Instrs[0] = (*ssa.Call)(nil) }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){invoke()}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				frame := view.Package.Func("Entry")
				row.mutate(frame)
				refusals := 0
				for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: frame, Timing: gotoolchain.SSACallTimingSynchronous}) {
					if call != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("malformed native call/refusal=(%v, %v), want nil and %v", call, err, core.ErrGoToolchainOutput)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("malformed native refusal count=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(malformed instructions) error = %v, want nil", err)
			}
		})
	}
}

func TestSSASynchronousCallsBorrowActualSpawnTargetFrames(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       int
	}{
		{"literal target", `go func(){r.Invoke()}()`, 1},
		{"bound wrapper target", `call:=r.Invoke;go call()`, 1},
		{"direct method body has no nested calls", `go r.Invoke()`, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Runner struct{};func(*Runner)Invoke(){};func Entry(r *Runner){`+row.body+`}`)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				entry := view.Package.Func("Entry")
				target := types.NewMethodSet(entry.Params[0].Type()).Lookup(nil, "Invoke").Obj()
				roots, calls := 0, 0
				for _, block := range entry.Blocks {
					for _, instruction := range block.Instrs {
						spawn, ok := instruction.(*ssa.Go)
						if !ok {
							continue
						}
						roots++
						for call, err := range view.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: spawn.Common().StaticCallee(), Timing: gotoolchain.SSACallTimingSynchronous}) {
							if err != nil {
								t.Fatalf("spawn target call stream error = %v, want nil", err)
							}
							if callee := call.Common().StaticCallee(); callee != nil && callee.Object() == target {
								calls++
							}
						}
					}
				}
				if roots != 1 || calls != row.want {
					t.Fatalf("native spawn roots/target calls=%d/%d, want 1/%d", roots, calls, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit(spawn target) error = %v, want nil", err)
			}
		})
	}
}
