package gotoolchain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func BenchmarkSSAArgumentInvocationsSelectedFrames(b *testing.B) {
	b.ReportAllocs()
	const prefix = 257
	for _, row := range []struct {
		name, body string
		loans      int
	}{
		{"two actual calls", `f:=func(){use(a)};f();f()`, 3},
		{"uncalled literal", `f:=func(){use(a)};_=f`, 1},
	} {
		b.Run(row.name, func(b *testing.B) {
			b.ReportAllocs()
			a := nativeSymbolSourceAnalysis(b, b.TempDir(), argumentInvocationDeclarations+`func Entry(a,b *Runner){`+strings.Repeat(`use(b);`, prefix)+row.body+`}`)
			err := a.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.AnonFuncs) != 1 {
					b.Fatalf("native literals got%d want1", len(fn.AnonFuncs))
				}
				request := gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]}
				for b.Loop() {
					loans := 0
					for frame, queryErr := range v.ArgumentInvocations(ctx, request) {
						if queryErr != nil {
							return queryErr
						}
						if frame == nil || frame.Binding().Validate() != nil {
							b.Fatalf("native loan got=%+v want non-nil validated binding", frame)
						}
						if loans == 0 {
							if frame.Kind() != gotoolchain.SSAInvocationKindRoot || frame.Binding().Function != fn {
								b.Fatalf("root got=%+v want kind=root function=%v", frame, fn)
							}
						} else if frame.Kind() != gotoolchain.SSAInvocationKindArgument || frame.Binding().Function != fn.AnonFuncs[0] || frame.Caller() == nil || frame.Caller().Binding().Function != fn || frame.Call() == nil {
							b.Fatalf("child got=%+v want kind=argument function=%v caller=%v and non-nil call", frame, fn.AnonFuncs[0], fn)
						}
						loans++
					}
					if loans != row.loans {
						b.Fatalf("native loans%d want%d", loans, row.loans)
					}
				}
				b.ReportMetric(prefix, "prefix_calls/op")
				b.ReportMetric(float64(row.loans), "native_loans/op")
				return nil
			}})
			if err != nil {
				b.Fatal(err)
			}
		})
	}
}
