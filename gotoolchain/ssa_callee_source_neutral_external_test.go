package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
	"testing"
)

func TestSSACalleeSourceKeepsNonSourceDispatchNeutral(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		damage       bool
	}{
		{"native builtin", `package sample;func Entry(n int){println(n)}`, false},
		{"multiple builtin arguments", `package sample;func Entry(n int){println(n,n)}`, false},
		{"deferred builtin", `package sample;func Entry(n int){defer println(n)}`, false},
		{"spawned builtin", `package sample;func Entry(n int){go println(n)}`, false},
		{"dynamic interface method", `package sample;type Receiver interface{Method(int)};func Entry(r Receiver){r.Method(1)}`, false},
		{"imported body", `package sample;import "strings";func Entry(){_=strings.Index("sample","a")}`, false},
		{"zero builtin has no native signature", `package sample;func Entry(n int){println(n)}`, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				call := nativeCalleeCall(t, v.Package.Func("Entry"))
				if row.damage {
					call.Common().Value = &ssa.Builtin{}
				}
				got, err := v.CalleeSource(ctx, gotoolchain.SSACalleeSourceRequest{Call: call})
				if got != (gotoolchain.SSAFunctionValueObservation{}) || !row.damage && err != nil || row.damage && !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("non-source callee got=%+v,%v, want zero and native neutrality or typed invalid-signature refusal", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("neutral callee unit got=%v, want nil", err)
			}
		})
	}
}
