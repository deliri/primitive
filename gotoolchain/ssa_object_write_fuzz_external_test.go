package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzSSAObjectWrittenConservesNativeVariableIdentity(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 4, 8, 16, 32, 63} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		body := "result=nil;"
		want := true
		if mode&1 != 0 {
			body = "_=result;"
			want = false
		}
		if mode&2 != 0 {
			body = "result:=error(nil);_=result;"
			want = false
		}
		if mode&4 != 0 {
			body = "other=nil;_=other;"
			want = false
		}
		if mode&8 != 0 {
			body = "if false{" + body + "};"
			want = false
		}
		if mode&16 != 0 {
			body = "func(){" + body + "}();"
			want = false
		}
		debug := mode&32 == 0
		if !debug {
			want = false
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry()(result error){other:=error(nil);_=other;defer func(){`+body+` }();return}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: debug, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			fn := v.Package.Func("Entry")
			observation, err := v.ObjectWritten(ctx, gotoolchain.SSAObjectWriteRequest{Function: fn.AnonFuncs[0], Object: fn.Signature.Results().At(0)})
			if err != nil || observation.Proven != want || observation.Validate() != nil {
				t.Fatalf("mode %d native write=(%+v,%v), want proven=%t and nil", mode, observation, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
