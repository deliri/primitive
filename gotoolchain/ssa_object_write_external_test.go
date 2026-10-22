package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAObjectWrittenUsesActualCompilerObject(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body          string
		root, noDebug, want bool
	}{
		{name: "captured named result is written", body: `defer func(){err=E{}}();return`, want: true},
		{name: "copied result value does not write result", body: `defer func(){copy:=err;copy=E{};_=copy}();return`},
		{name: "same spelled shadow is no result write", body: `defer func(){err:=error(E{});_=err}();return`},
		{name: "read cannot grant a write", body: `defer func(){_=err}();return`},
		{name: "neighboring captured cell cannot grant write", body: `other:=error(nil);defer func(){other=E{};_=other}();return`},
		{name: "constant untaken write grants no fact", body: `defer func(){if false{err=E{}}}();return`},
		{name: "runtime conditional write is possible", body: `defer func(){if flag{err=E{}}}();return`, want: true},
		{name: "nil assignment is still a write", body: `defer func(){err=nil}();return`, want: true},
		{name: "nested body is outside selected function", body: `defer func(){func(){err=E{}}()}();return`},
		{name: "ordinary root result write", body: `err=E{};return`, root: true},
		{name: "absent debug reference grants no object fact", body: `defer func(){err=E{}}();return`, noDebug: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type E struct{};func(E)Error()string{return "error"};func Entry(flag bool)(err error){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: !row.noDebug, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry")
				fn := entry
				if !row.root {
					if len(entry.AnonFuncs) != 1 {
						t.Fatalf("selected closures=%d, want 1", len(entry.AnonFuncs))
					}
					fn = entry.AnonFuncs[0]
				}
				if row.root {
					stores := 0
					for _, block := range fn.Blocks {
						for _, instruction := range block.Instrs {
							if _, stored := instruction.(*ssa.Store); stored {
								stores++
							}
						}
					}
					if stores != 0 {
						t.Fatalf("uncaptured result stores=%d, want 0 native stores", stores)
					}
				}
				observation, err := v.ObjectWritten(ctx, gotoolchain.SSAObjectWriteRequest{Function: fn, Object: entry.Signature.Results().At(0)})
				if err != nil || observation.Proven != row.want || observation.Validate() != nil {
					t.Fatalf("compiler object write=(%+v,%v), want proven=%t and nil", observation, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
