package gotoolchain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

// This finite source partition crosses six different callable producer states,
// one/two helper bindings, native/named function types and conditional calls.
// It is not exhaustion of all possible Go programs or native provider damage.
func invocationCalleeSource(mode uint8) string {
	mode %= 48
	kind := "func()"
	if mode/12%2 != 0 {
		kind = "Callback"
	}
	call := "callback()"
	if mode/24 != 0 {
		call = "if enabled{" + call + "}"
	}
	source := "package sample;type Runner struct{};type Callback func();func use(*Runner){};func named(){};func other(){};"
	source += "func helper(r *Runner,callback " + kind + ",enabled bool){" + call + "};"
	source += "func bridge(r *Runner,callback " + kind + ",enabled bool){helper(r,callback,enabled)};"
	target := "helper"
	if mode/6%2 != 0 {
		target = "bridge"
	}
	prefix, value := "", ""
	switch mode % 6 {
	case 0:
		value = "func(){use(a)}"
	case 1:
		value = "func(){use(b)}"
	case 2:
		value = "named"
	case 3:
		value = "nil"
	case 4:
		value = "unknown"
	case 5:
		prefix = "merged:=named;if enabled{merged=other};"
		value = "merged"
	}
	if kind == "Callback" {
		value = "Callback(" + value + ")"
	}
	return source + "func Entry(a,b *Runner,unknown func(),enabled bool){" + prefix + target + "(a," + value + ",enabled)}"
}

func observeInvocationCalleeMode(ctx context.Context, t testing.TB, view gotoolchain.SSAUnitView, mode uint8) {
	t.Helper()
	mode %= 48
	entry := view.Package.Func("Entry")
	frame, err := view.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: entry, Argument: entry.Params[0]})
	if err != nil {
		t.Fatalf("mode=%d native invocation root error=%v, want nil", mode, err)
	}
	actual := nativeCalleeCall(t, entry).(*ssa.Call).Common().Args[1]
	hops := 1 + int(mode/6%2)
	for range hops {
		call := nativeCalleeCall(t, frame.Binding().Function).(*ssa.Call)
		var next *gotoolchain.SSAArgumentInvocationFrame
		for binding, bindingErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: frame, Call: call}) {
			if bindingErr != nil || next != nil {
				t.Fatalf("mode=%d native helper binding=%+v error=%v, want exactly one native runner parameter", mode, binding, bindingErr)
			}
			next = binding
		}
		if next == nil {
			t.Fatalf("mode=%d helper binding absent, want selected native runner", mode)
		}
		frame = next
	}
	callback := nativeCalleeCall(t, frame.Binding().Function).(*ssa.Call)
	request := gotoolchain.SSAArgumentInvocationRequest{Caller: frame, Call: callback}
	observed, err := view.InvocationCalleeSource(ctx, request)
	var want *ssa.Function
	if mode%6 < 2 {
		want = entry.AnonFuncs[0]
	} else if mode%6 == 2 {
		want = view.Package.Func("named")
	}
	if err != nil || observed.Function != want || want != nil && observed.Value != actual || want == nil && observed != (gotoolchain.SSAFunctionValueObservation{}) {
		t.Fatalf("mode=%d invocation source=%+v,%v, want native function=%v and actual argument=%v when present", mode, observed, err, want, actual)
	}
	captures := 0
	for capture, captureErr := range view.ArgumentInvocationBindings(ctx, request) {
		if captureErr != nil || want == nil || capture.Binding().Function != want || capture.Binding().Local != want.FreeVars[0] || capture.Call() != callback || capture.Caller() != frame {
			t.Fatalf("mode=%d invocation capture=%+v error=%v, want exact selected native capture and ancestry", mode, capture, captureErr)
		}
		captures++
	}
	wantCaptures := 0
	if mode%6 == 0 {
		wantCaptures = 1
	}
	if captures != wantCaptures {
		t.Fatalf("mode=%d native captures=%d, want %d", mode, captures, wantCaptures)
	}
}

func TestSSAInvocationCalleeSourceFinitePartition(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 48; mode++ {
		t.Run(fmt.Sprintf("source_%d", mode), func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), invocationCalleeSource(mode))
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				observeInvocationCalleeMode(ctx, t, view, mode)
				return nil
			}})
			if err != nil {
				t.Fatalf("mode=%d native source partition error=%v, want nil", mode, err)
			}
		})
	}
}

func FuzzSSAInvocationCalleeConservesNativeCallerAndCapture(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 3, 4, 5, 6, 12, 24, 47, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		mode %= 48
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), invocationCalleeSource(mode))
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			observeInvocationCalleeMode(ctx, t, view, mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("mode=%d native invocation fuzz error=%v, want nil", mode, err)
		}
	})
}
