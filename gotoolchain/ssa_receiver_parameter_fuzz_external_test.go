package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSAReceiverParameterConservesSelectedAuthority(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 4, 8, 16, 32} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		receiver := "r"
		if mode&1 != 0 {
			receiver = "other"
		}
		declarations, parameters := "", "r *Cell,other *Cell"
		if mode&2 != 0 {
			declarations, parameters = "type Owner struct{*Cell};", "r *Owner,other *Owner"
		}
		if mode&16 != 0 {
			declarations, parameters = "", "r Runner,other Runner"
		}
		prefix := ""
		if mode&8 != 0 {
			prefix = "replace(&r);"
		}
		body := receiver + ".Invoke(1)"
		if mode&4 != 0 {
			body = "bound:=" + receiver + ".Invoke;bound(1)"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)Invoke(int){};type Runner interface{Invoke(int)};func replace[T any](p *T){var zero T;*p=zero};`+declarations+`func Entry(`+parameters+`){`+prefix+body+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			fn := view.Package.Func("Entry")
			request := gotoolchain.SSAReceiverParameterRequest{Call: nativeReceiverParameterLastCall(ctx, t, view, fn), Parameter: fn.Params[0]}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			if mode&32 != 0 {
				cancel(io.ErrClosedPipe)
			}
			got, queryErr := view.ReceiverParameter(queryCtx, request)
			if mode&32 != 0 {
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("cancelled receiver=%+v,%v, want zero,closed pipe", got, queryErr)
				}
				return nil
			}
			want := mode&(1|8) == 0
			if queryErr != nil || got.Proven != want {
				t.Fatalf("receiver mode=%d proof=%+v,%v, want proven=%t,nil", mode, got, queryErr, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
