package gotoolchain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSSAArgumentInvocationsFirstLoanRetainsNoPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, row := range []struct {
		name         string
		width, depth int
	}{
		{"width32", 32, 1}, {"width8193", 8193, 1}, {"depth8", 1, 8}, {"depth257", 1, 257},
	} {
		t.Run(row.name, func(t *testing.T) {
			body := strings.Repeat(`_=func(){use(a)};`, row.width-1) + strings.Repeat(`_=func(){`, row.depth) + `use(a)` + strings.Repeat(`};`, row.depth)
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func Entry(a *Runner){`+body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				if len(fn.AnonFuncs) != row.width {
					t.Fatalf("native width%d want%d", len(fn.AnonFuncs), row.width)
				}
				leaf := fn.AnonFuncs[row.width-1]
				for level := 1; level < row.depth; level++ {
					if len(leaf.AnonFuncs) != 1 {
						t.Fatalf("native depth%d child count got%d want1", level, len(leaf.AnonFuncs))
					}
					leaf = leaf.AnonFuncs[0]
				}
				start, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				loans := 0
				v.ArgumentInvocations(ctx, gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]})(func(frame *gotoolchain.SSAArgumentInvocationFrame, queryErr error) bool {
					if queryErr != nil || frame == nil || frame.Kind() != gotoolchain.SSAInvocationKindRoot || frame.Binding().Function != fn || frame.Binding().Local != fn.Params[0] || frame.Caller() != nil || loans != 0 {
						t.Fatalf("first native root=%v,%v loans%d", frame, queryErr, loans)
					}
					loans++
					return false
				})
				end, err := hostfacts.ObserveGoAllocationTotal(ctx)
				if err != nil {
					return err
				}
				allocated := end.Uint64() - start.Uint64()
				t.Logf("native first loan width%d depth%d bytes%d; compiler graph and active native traversal depth excluded", row.width, row.depth, allocated)
				if loans != 1 || allocated > 512 {
					t.Fatalf("first loan count%d bytes%d want1 and <=512 fixed scratch", loans, allocated)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
