package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSASourceCallFallbackRefusesMalformedNativeInstructions(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		mutate       func(*ssa.Function, *ssa.Function)
		runtimeCause bool
	}{
		{"nil block", func(fn, _ *ssa.Function) { fn.Blocks[0] = nil }, true},
		{"malformed block ordinal", func(fn, _ *ssa.Function) { fn.Blocks[0].Index += 100 }, false},
		{"nil instruction", func(fn, _ *ssa.Function) { fn.Blocks[0].Instrs[0] = nil }, false},
		{"typed nil call instruction", func(fn, _ *ssa.Function) { fn.Blocks[0].Instrs[0] = (*ssa.Call)(nil) }, false},
		{"typed nil debug reference", func(fn, _ *ssa.Function) { fn.Blocks[0].Instrs[0] = (*ssa.DebugRef)(nil) }, true},
		{"foreign instruction", func(fn, other *ssa.Function) { fn.Blocks[0].Instrs[0] = other.Blocks[0].Instrs[0] }, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if failure := recover(); failure != nil {
					t.Fatalf("source lookup panic=%v, want zero observation and %v", failure, core.ErrGoToolchainOutput)
				}
			}()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(){};func Entry(){defer sink()};func Other(){sink()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				expression := fn.Syntax().(*ast.FuncDecl).Body.List[0].(*ast.DeferStmt).Call
				r := gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression}
				before, queryErr := v.CallForExpression(ctx, r)
				if _, deferred := before.Instruction.(*ssa.Defer); !deferred || queryErr != nil {
					t.Fatalf("deferred precondition=(%+v,%v), want deferred call and nil", before, queryErr)
				}
				row.mutate(fn, v.Package.Func("Other"))
				got, queryErr := v.CallForExpression(ctx, r)
				if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("malformed native source=(%+v,%v), want zero observation and %v", got, queryErr, core.ErrGoToolchainOutput)
				}
				if row.runtimeCause {
					var cause runtime.Error
					if !errors.As(queryErr, &cause) {
						t.Fatalf("native runtime cause=%v, want preserved runtime.Error", queryErr)
					}
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
