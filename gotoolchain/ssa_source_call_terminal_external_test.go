package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSASourceCallFallbackPreservesTerminalBoundary(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		panics bool
		want   error
	}{
		{"late cancellation retains native cause", false, io.ErrClosedPipe},
		{"late context panic refuses observation", true, core.ErrContextObservation},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func sink(){};func Entry(){defer sink()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				expression := fn.Syntax().(*ast.FuncDecl).Body.List[0].(*ast.DeferStmt).Call
				request := gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression}
				if got, queryErr := view.CallForExpression(ctx, request); got.Instruction == nil || queryErr != nil {
					t.Fatalf("fallback precondition=(%+v,%v), want native call,nil", got, queryErr)
				}
				parent, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				terminal := &sourceCallTerminalContext{Context: parent, cancel: cancel, panics: row.panics}
				got, queryErr := view.CallForExpression(terminal, request)
				if got != (gotoolchain.SSASourceCallObservation{}) || !errors.Is(queryErr, row.want) || terminal.checks < 3 {
					t.Fatalf("fallback terminal=(%+v,%v), checks=%d, want zero,%v after third context check", got, queryErr, terminal.checks, row.want)
				}
				if !row.panics && !errors.Is(queryErr, context.Canceled) {
					t.Fatalf("fallback cancellation error=%v, want native terminal sentinel", queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

// Entry and block admission remain usable. The third Err observation changes
// the native context at the public return boundary, after instruction selection.
type sourceCallTerminalContext struct {
	context.Context
	cancel context.CancelCauseFunc
	checks int
	panics bool
}

func (c *sourceCallTerminalContext) Err() error {
	c.checks++
	if c.checks == 3 {
		if c.panics {
			panic(io.ErrClosedPipe)
		}
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}
