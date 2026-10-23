package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeBlockDominanceSource = `package sample;func left(){};func right(){};func join(){};func Entry(flag bool){if flag{left()}else{right()};join()};func Other(){join()};func Closure(){_=func(flag bool){if flag{left()}else{right()};join()}}`

func TestSSABlockDominanceUsesCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("dominance owner got=%+v,%v, want compiled capability,nil", match, err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSABlockDominanceRequest]().PkgPath() != path {
		t.Fatalf("dominance contract owner got=%s,%v, want catalog path %s,nil", reflect.TypeFor[gotoolchain.SSABlockDominanceRequest]().PkgPath(), err, path)
	}
}

func TestSSABlockDominanceConservesEveryDiamondRelation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBlockDominanceSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
			t.Fatal("native diamond got missing, want two arms and shared join")
		}
		blocks := [4]*ssa.BasicBlock{entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
		for before, dominator := range blocks {
			for after, block := range blocks {
				got, queryErr := v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: dominator, Block: block})
				// Independent source oracle: entry dominates every diamond
				// node; each other node dominates only itself.
				want := before == 0 || before == after
				if queryErr != nil || got.Proven != want || got.Validate() != nil {
					t.Fatalf("diamond %d->%d got=%+v,%v, want proven=%t,nil", before, after, got, queryErr, want)
				}
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error got=%v, want nil", err)
	}
}

func TestSSABlockDominanceRefusesUnownedLoans(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSABlockDominanceRequest)
		want   error
	}{
		{"absent unit", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSABlockDominanceRequest) {
			*v = gotoolchain.SSAUnitView{}
		}, core.ErrGoToolchainContract},
		{"absent dominator", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Dominator = nil }, core.ErrGoToolchainContract},
		{"absent block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Block = nil }, core.ErrGoToolchainContract},
		{"parentless dominator", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Dominator = &ssa.BasicBlock{}
		}, core.ErrGoToolchainContract},
		{"parentless block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Block = &ssa.BasicBlock{} }, core.ErrGoToolchainContract},
		{"different native function", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Dominator = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainContract},
		{"foreign program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Block.Parent().Prog = nil }, core.ErrGoToolchainContract},
		{"foreign package", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Block.Parent().Pkg = &ssa.Package{Prog: v.Program, Pkg: v.Source.Types}
		}, core.ErrGoToolchainContract},
		{"detached dominator", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			copied := *r.Dominator
			r.Dominator = &copied
		}, core.ErrGoToolchainOutput},
		{"detached block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			copied := *r.Block
			r.Block = &copied
		}, core.ErrGoToolchainOutput},
		{"negative dominator ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Dominator.Index = -1 }, core.ErrGoToolchainOutput},
		{"negative block ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) { r.Block.Index = -1 }, core.ErrGoToolchainOutput},
		{"dominator ordinal at extent", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Dominator.Index = len(r.Dominator.Parent().Blocks)
		}, core.ErrGoToolchainOutput},
		{"block ordinal at extent", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Block.Index = len(r.Block.Parent().Blocks)
		}, core.ErrGoToolchainOutput},
		{"dominator missing from native slots", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Dominator.Parent().Blocks[r.Dominator.Index] = nil
		}, core.ErrGoToolchainOutput},
		{"block missing from native slots", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockDominanceRequest) {
			r.Block.Parent().Blocks[r.Block.Index] = nil
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBlockDominanceSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				r := gotoolchain.SSABlockDominanceRequest{Dominator: entry, Block: entry.Succs[0].Succs[0]}
				row.mutate(&v, &r)
				got, queryErr := v.BlockDominates(ctx, r)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != (row.want == core.ErrGoToolchainOutput) {
					t.Fatalf("native custody got=%+v,%v, want zero,%v", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error got=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockDominanceProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBlockDominanceSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		left, right := entry.Succs[0], entry.Succs[1]
		r := gotoolchain.SSABlockDominanceRequest{Dominator: entry, Block: left}
		got, queryErr := v.BlockDominates(ctx, r)
		if queryErr != nil || !got.Proven {
			t.Fatalf("positive dominance got=%+v,%v, want proven,nil", got, queryErr)
		}
		entry.Index = -1
		got, queryErr = v.BlockDominates(ctx, r)
		if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative custody got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		got, queryErr = v.BlockDominates(ctx, gotoolchain.SSABlockDominanceRequest{Dominator: left, Block: right})
		if queryErr != nil || got != (gotoolchain.SSARelationObservation{}) {
			t.Fatalf("neutral sibling got=%+v,%v, want zero,nil without unrelated entry inspection", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error got=%v, want nil", err)
	}
}

func TestSSABlockDominancePreservesContextAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		mode nativeBlockDominanceContextMode
		want error
	}{
		{"absent", nativeBlockDominanceContextAbsent, core.ErrNilContext},
		{"cancelled", nativeBlockDominanceContextCancelled, io.ErrClosedPipe},
		{"expired deadline", nativeBlockDominanceContextExpired, context.DeadlineExceeded},
		{"final cancellation", nativeBlockDominanceContextFinal, io.ErrClosedPipe},
		{"cancelled invalid request", nativeBlockDominanceContextInvalid, core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBlockDominanceSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				r := gotoolchain.SSABlockDominanceRequest{Dominator: entry, Block: entry}
				queryCtx := ctx
				switch row.mode {
				case nativeBlockDominanceContextAbsent:
					queryCtx = nil
				case nativeBlockDominanceContextCancelled, nativeBlockDominanceContextInvalid:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					queryCtx = cancelled
					if row.mode == nativeBlockDominanceContextInvalid {
						r.Dominator = nil
					}
				case nativeBlockDominanceContextExpired:
					deadline, cancel := context.WithDeadlineCause(ctx, time.Time{}, io.ErrClosedPipe)
					defer cancel()
					queryCtx = deadline
				case nativeBlockDominanceContextFinal:
					parent, cancel := context.WithCancelCause(ctx)
					defer cancel(nil)
					queryCtx = &nativeBlockDominanceFinalContext{Context: parent, cancel: cancel}
				default:
					t.Fatalf("context mode got=%d, want declared native boundary", row.mode)
				}
				got, queryErr := v.BlockDominates(queryCtx, r)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("%s context got=%+v,%v, want zero,%v", row.name, got, queryErr, row.want)
				}
				if row.mode != nativeBlockDominanceContextAbsent && !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("context cause got=%v, want closed pipe", queryErr)
				}
				if row.mode == nativeBlockDominanceContextFinal && !errors.Is(queryErr, context.Canceled) {
					t.Fatalf("final context got=%v, want cancelled", queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error got=%v, want nil", err)
			}
		})
	}
}

type nativeBlockDominanceContextMode uint8

const (
	nativeBlockDominanceContextAbsent nativeBlockDominanceContextMode = iota
	nativeBlockDominanceContextCancelled
	nativeBlockDominanceContextExpired
	nativeBlockDominanceContextFinal
	nativeBlockDominanceContextInvalid
)

type nativeBlockDominanceFinalContext struct {
	context.Context
	cancel context.CancelCauseFunc
	checks int
}

func (c *nativeBlockDominanceFinalContext) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}
