package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSABlockPredecessorsObserveFinalCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		arm := entry.Succs[0]
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		seen, refused := 0, 0
		for predecessor, queryErr := range v.BlockLinks(parent, gotoolchain.SSABlockLinkRequest{Block: arm, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil {
				if predecessor != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("final predecessor receipt got=%v,%v, want nil,cancelled with cause", predecessor, queryErr)
				}
				refused++
				continue
			}
			if predecessor != entry {
				t.Fatalf("single ingress got=%v, want entry=%d", predecessor, entry.Index)
			}
			seen++
			cancel(io.ErrClosedPipe)
		}
		if seen != 1 || refused != 1 {
			t.Fatalf("final stream got values=%d refusals=%d, want one value and one final refusal", seen, refused)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABlockPredecessorsRefuseAbsentAndExpiredContexts(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		deadline, stop := context.WithDeadlineCause(ctx, time.Time{}, io.ErrClosedPipe)
		defer stop()
		for _, row := range []struct {
			name               string
			ctx                context.Context
			wantErr, wantCause error
		}{
			{"absent", nil, core.ErrNilContext, nil}, {"expired", deadline, context.DeadlineExceeded, io.ErrClosedPipe},
		} {
			refused := 0
			for predecessor, queryErr := range v.BlockLinks(row.ctx, gotoolchain.SSABlockLinkRequest{Block: entry.Succs[0], Kind: gotoolchain.SSABlockLinkPredecessors}) {
				if predecessor != nil || !errors.Is(queryErr, row.wantErr) || row.wantCause != nil && !errors.Is(queryErr, row.wantCause) {
					t.Fatalf("%s context got=%v,%v, want nil,%v with cause %v", row.name, predecessor, queryErr, row.wantErr, row.wantCause)
				}
				refused++
			}
			if refused != 1 {
				t.Fatalf("%s refusals got=%d, want one", row.name, refused)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
