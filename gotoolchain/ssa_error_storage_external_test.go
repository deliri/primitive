package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAStoragePreservesStableErrorInterfaceCells(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{name: "direct error parameter", body: `observe(root)`, want: true},
		{name: "read only captured error", body: `copy:=root;read:=func(){observe(copy)};read()`, want: true},
		{name: "captured error read in parent", body: `copy:=root;read:=func(){consume(copy)};read();observe(copy)`, want: true},
		{name: "nested captured error", body: `copy:=root;read:=func(){inner:=func(){observe(copy)};inner()};read()`, want: true},
		{name: "other captured error", body: `copy:=other;read:=func(){observe(copy)};read()`},
		{name: "native global sentinel is not an input identity", body: `copy:=Global;read:=func(){observe(copy)};read()`},
		{name: "parent replacement stays opaque", body: `copy:=root;read:=func(){observe(copy)};copy=other;read()`},
		{name: "child replacement loses original identity", body: `copy:=root;read:=func(){copy=other;observe(copy)};read()`},
		{name: "sibling replacement loses original identity", body: `copy:=root;read:=func(){observe(copy)};write:=func(){copy=other};write();read()`},
		{name: "escaping interface cell stays opaque", body: `copy:=root;escape(&copy);read:=func(){observe(copy)};read()`},
		{name: "conditional store stays opaque", body: `var copy error;if flag{copy=root};read:=func(){observe(copy)};read()`},
		{name: "store after read cannot establish earlier identity", body: `var copy error;read:=func(){observe(copy)};read();copy=root`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			nativeStableErrorStorage(t, t.TempDir(), row.body, row.want)
		})
	}
}

func nativeStableErrorStorage(t *testing.T, directory, body string, want bool) {
	t.Helper()
	analysis := nativeSymbolSourceAnalysis(t, directory, `package sample;var Global error;func observe(error){};func consume(error){};func escape(*error){};func F(root,other error,flag bool){`+body+`}`)
	unit := analysis.Units[0]
	if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		parent := view.Package.Func("F")
		visits := 0
		for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: parent, Extent: gotoolchain.SSAInstructionExtentLexical}) {
			if err != nil {
				return err
			}
			call, ok := instruction.(*ssa.Call)
			if !ok || call.Common().StaticCallee() != view.Package.Func("observe") {
				continue
			}
			got, err := view.SameStorage(ctx, gotoolchain.SSAStorageRequest{Left: parent.Params[0], Right: call.Common().Args[0]})
			if err != nil || got.Validate() != nil || got.Proven != want {
				t.Fatalf("native interface storage = %+v/%v, want %t/nil", got, err, want)
			}
			visits++
		}
		if visits != 1 {
			t.Fatalf("native observed operands = %d, want 1", visits)
		}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
}

func FuzzSSAStableErrorStorageConservesNativeIdentity(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(255))
	f.Add(uint8(16))
	f.Fuzz(func(t *testing.T, mode uint8) {
		first := "root"
		if mode&1 != 0 {
			first = "other"
		}
		if mode&128 != 0 {
			first = "Global"
		}
		body := "copy:=" + first + ";"
		if mode&64 != 0 {
			body = "var copy error=" + first + ";"
		}
		if mode&2 != 0 {
			body += "copy=other;"
		}
		if mode&8 != 0 {
			body += "escape(&copy);"
		}
		observe := "observe(copy)"
		if mode&32 != 0 {
			observe = "consume(copy)"
		}
		if mode&4 != 0 {
			observe = "copy=other;" + observe
		}
		if mode&16 != 0 {
			observe = "inner:=func(){" + observe + "};inner()"
		}
		body += "read:=func(){" + observe + "};read();"
		if mode&32 != 0 {
			body += "observe(copy)"
		}
		nativeStableErrorStorage(t, t.TempDir(), body, mode&(1|2|4|8|128) == 0)
	})
}
