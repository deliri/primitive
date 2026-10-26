package gotoolchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/testserial"
	"golang.org/x/tools/go/ssa"
)

func wideInvocationCalleeSource(width int) string {
	var source strings.Builder
	source.WriteString("package sample;type Runner struct{};func use(*Runner){};func helper(r *Runner")
	for index := range width {
		fmt.Fprintf(&source, ",x%d int", index)
	}
	source.WriteString(",callback func()){callback()};func Entry(r *Runner){helper(r")
	for range width {
		source.WriteString(",0")
	}
	source.WriteString(",func(){use(r)})}")
	return source.String()
}

func wideInvocationCalleeRequest(ctx context.Context, t testing.TB, view gotoolchain.SSAUnitView) gotoolchain.SSAArgumentInvocationRequest {
	t.Helper()
	entry := view.Package.Func("Entry")
	root, err := view.ArgumentInvocationRoot(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: entry, Argument: entry.Params[0]})
	if err != nil {
		t.Fatalf("wide invocation root error=%v, want nil", err)
	}
	for frame, frameErr := range view.ArgumentInvocationBindings(ctx, gotoolchain.SSAArgumentInvocationRequest{Caller: root, Call: nativeCalleeCall(t, entry).(*ssa.Call)}) {
		if frameErr != nil {
			t.Fatalf("wide invocation helper error=%v, want nil", frameErr)
		}
		return gotoolchain.SSAArgumentInvocationRequest{Caller: frame, Call: nativeCalleeCall(t, frame.Binding().Function).(*ssa.Call)}
	}
	t.Fatal("wide invocation helper absent, want actual selected parameter")
	return gotoolchain.SSAArgumentInvocationRequest{}
}

func TestSSAInvocationCalleeRetainsNoArgumentPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, width := range []int{32, 8193} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), wideInvocationCalleeSource(width))
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				request := wideInvocationCalleeRequest(ctx, t, view)
				actual := request.Caller.Call().Common().Args
				if len(actual) != width+2 {
					t.Fatalf("native helper width=%d, want %d arguments", len(actual), width+2)
				}
				observed, queryErr := view.InvocationCalleeSource(ctx, request)
				want := view.Package.Func("Entry").AnonFuncs[0]
				if queryErr != nil || observed.Function != want || observed.Value != actual[len(actual)-1] {
					t.Fatalf("wide callback baseline=%+v,%v, want exact source and last native argument", observed, queryErr)
				}
				allocations := testing.AllocsPerRun(5, func() { observed, queryErr = view.InvocationCalleeSource(ctx, request) })
				if queryErr != nil || observed.Function != want || observed.Value != actual[len(actual)-1] || allocations != 0 {
					t.Fatalf("width=%d query allocations=%g source=%+v error=%v, want zero allocations and exact source", width, allocations, observed, queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("width=%d native unit error=%v, want nil", width, err)
			}
		})
	}
}

func BenchmarkSSAInvocationCalleeSource(b *testing.B) {
	b.ReportAllocs()
	const width = 257
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), wideInvocationCalleeSource(width))
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		request := wideInvocationCalleeRequest(ctx, b, view)
		actual := request.Caller.Call().Common().Args
		if len(actual) != width+2 {
			b.Fatalf("native helper width=%d, want %d arguments", len(actual), width+2)
		}
		var observed gotoolchain.SSAFunctionValueObservation
		var queryErr error
		for b.Loop() {
			observed, queryErr = view.InvocationCalleeSource(ctx, request)
		}
		if queryErr != nil || observed.Function != view.Package.Func("Entry").AnonFuncs[0] || observed.Value != actual[len(actual)-1] {
			b.Fatalf("native callback benchmark source=%+v error=%v, want exact caller-bound source", observed, queryErr)
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("native callback benchmark unit error=%v, want nil", err)
	}
}
