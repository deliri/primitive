package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSABlockLinksLendNativeRelations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name     string
		kind     gotoolchain.SSABlockLinkKind
		selected int
		want     []int
	}{
		{"branch successors preserve Go edge order", gotoolchain.SSABlockLinkSuccessors, 0, []int{1, 2}},
		{"left edge reaches shared join", gotoolchain.SSABlockLinkSuccessors, 1, []int{3}},
		{"right edge reaches shared join", gotoolchain.SSABlockLinkSuccessors, 2, []int{3}},
		{"return has no successors", gotoolchain.SSABlockLinkSuccessors, 3, nil},
		{"entry ancestry includes self", gotoolchain.SSABlockLinkDominatorAncestry, 0, []int{0}},
		{"left ancestry reaches entry", gotoolchain.SSABlockLinkDominatorAncestry, 1, []int{1, 0}},
		{"right ancestry reaches entry", gotoolchain.SSABlockLinkDominatorAncestry, 2, []int{2, 0}},
		{"join ancestry excludes conditional arms", gotoolchain.SSABlockLinkDominatorAncestry, 3, []int{3, 0}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func left(){};func right(){};func join(){};func Entry(flag bool){if flag{left()}else{right()};join()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				entry := fn.Blocks[0]
				if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
					t.Fatal("native diamond shape absent, want two arms and shared join")
				}
				blocks := []*ssa.BasicBlock{entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
				var got []*ssa.BasicBlock
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: blocks[row.selected], Kind: row.kind}) {
					if queryErr != nil {
						t.Fatalf("native relation error=%v, want nil", queryErr)
					}
					got = append(got, block)
				}
				var want []*ssa.BasicBlock
				for _, index := range row.want {
					want = append(want, blocks[index])
				}
				if !slices.Equal(got, want) {
					t.Fatalf("native relations=%v, want %v", got, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockLinksRefuseDamagedCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSABlockLinkRequest)
		want   error
		prefix int
	}{
		{"absent loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSABlockLinkRequest) { *v = gotoolchain.SSAUnitView{} }, core.ErrGoToolchainContract, 0},
		{"absent selected block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block = nil }, core.ErrGoToolchainContract, 0},
		{"parentless selected block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block = &ssa.BasicBlock{} }, core.ErrGoToolchainContract, 0},
		{"unknown relation", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Kind = gotoolchain.SSABlockLinkUnknown
		}, core.ErrGoToolchainContract, 0},
		{"foreign program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block.Parent().Prog = nil }, core.ErrGoToolchainContract, 0},
		{"foreign package custody", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Parent().Pkg = &ssa.Package{Prog: v.Program, Pkg: v.Source.Types}
		}, core.ErrGoToolchainContract, 0},
		{"detached selector", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			copied := *r.Block
			r.Block = &copied
		}, core.ErrGoToolchainOutput, 0},
		{"negative selected ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block.Index = -1 }, core.ErrGoToolchainOutput, 0},
		{"selected ordinal at extent", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Index = len(r.Block.Parent().Blocks)
		}, core.ErrGoToolchainOutput, 0},
		{"selected ordinal names sibling", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Index = r.Block.Succs[0].Index
		}, core.ErrGoToolchainOutput, 0},
		{"nil successor", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block.Succs[0] = nil }, core.ErrGoToolchainOutput, 0},
		{"foreign successor", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Succs[0] = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainOutput, 0},
		{"detached successor", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			copied := *r.Block.Succs[0]
			r.Block.Succs[0] = &copied
		}, core.ErrGoToolchainOutput, 0},
		{"negative successor ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block.Succs[0].Index = -1 }, core.ErrGoToolchainOutput, 0},
		{"successor ordinal at extent", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Succs[0].Index = len(r.Block.Parent().Blocks)
		}, core.ErrGoToolchainOutput, 0},
		{"successor missing from owning slots", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			r.Block.Parent().Blocks[r.Block.Succs[0].Index] = nil
		}, core.ErrGoToolchainOutput, 0},
		{"damaged later successor preserves prefix only", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) { r.Block.Succs[1] = nil }, core.ErrGoToolchainOutput, 1},
		{"damaged ancestor refuses after selected block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSABlockLinkRequest) {
			entry := r.Block
			r.Block = entry.Succs[0]
			r.Kind = gotoolchain.SSABlockLinkDominatorAncestry
			entry.Index = -1
		}, core.ErrGoToolchainOutput, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){if flag{invoke()}else{invoke()};invoke()};func Other(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSABlockLinkRequest{Block: v.Package.Func("Entry").Blocks[0], Kind: gotoolchain.SSABlockLinkSuccessors}
				row.mutate(&v, &r)
				observed, refusals := 0, 0
				for block, queryErr := range v.BlockLinks(ctx, r) {
					if queryErr == nil {
						if block == nil || refusals != 0 {
							t.Fatalf("prefix block=%v after %d refusals, want actual prefix", block, refusals)
						}
						observed++
						continue
					}
					if block != nil || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
						t.Fatalf("relation refusal=%v,%v, want nil,%v", block, queryErr, row.want)
					}
					refusals++
				}
				if observed != row.prefix || refusals != 1 {
					t.Fatalf("relations/refusals=%d/%d, want %d/1", observed, refusals, row.prefix)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockLinkKindIsClosed(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		kind := gotoolchain.SSABlockLinkKind(value)
		want := kind == gotoolchain.SSABlockLinkSuccessors || kind == gotoolchain.SSABlockLinkDominatorAncestry
		if kind.IsValid() != want || (kind.Validate() == nil) != want || !want && (!errors.Is(kind.Validate(), core.ErrGoToolchainContract) || kind.String() != core.UnknownEnumDiagnostic) {
			t.Fatalf("relation kind=%d valid=%t error=%v string=%q, want closed domain", value, kind.IsValid(), kind.Validate(), kind.String())
		}
	}
	if gotoolchain.SSABlockLinkSuccessors.String() != "successors" || gotoolchain.SSABlockLinkDominatorAncestry.String() != "dominator_ancestry" {
		t.Fatal("relation diagnostics differ, want successors and dominator_ancestry")
	}
}

func TestSSABlockLinksPreserveCancellationAndStop(t *testing.T) {
	t.Parallel()
	for _, kind := range []gotoolchain.SSABlockLinkKind{gotoolchain.SSABlockLinkSuccessors, gotoolchain.SSABlockLinkDominatorAncestry} {
		t.Run(kind.String(), func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){if flag{invoke()}else{invoke()};invoke()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				r := gotoolchain.SSABlockLinkRequest{Block: entry, Kind: kind}
				if kind == gotoolchain.SSABlockLinkDominatorAncestry {
					r.Block = entry.Succs[0]
				}
				cancelled, cancel := context.WithCancelCause(ctx)
				cancel(io.ErrClosedPipe)
				deadline, stopDeadline := context.WithDeadlineCause(ctx, time.Time{}, io.ErrUnexpectedEOF)
				defer stopDeadline()
				for _, row := range []struct {
					ctx         context.Context
					want, cause error
				}{{nil, core.ErrNilContext, nil}, {cancelled, context.Canceled, io.ErrClosedPipe}, {deadline, context.DeadlineExceeded, io.ErrUnexpectedEOF}} {
					refusals := 0
					for block, queryErr := range v.BlockLinks(row.ctx, r) {
						if block != nil || !errors.Is(queryErr, row.want) || row.cause != nil && !errors.Is(queryErr, row.cause) {
							t.Fatalf("initial refusal=%v,%v, want nil,%v with %v", block, queryErr, row.want, row.cause)
						}
						refusals++
					}
					if refusals != 1 {
						t.Fatalf("initial refusals=%d, want 1", refusals)
					}
				}
				late, stop := context.WithCancelCause(ctx)
				defer stop(nil)
				observed, refusals := 0, 0
				for block, queryErr := range v.BlockLinks(late, r) {
					if queryErr != nil {
						if block != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
							t.Fatalf("late refusal=%v,%v, want nil,cancelled with native cause", block, queryErr)
						}
						refusals++
						continue
					}
					observed++
					stop(io.ErrClosedPipe)
				}
				if observed != 1 || refusals != 1 {
					t.Fatalf("late observations/refusals=%d/%d, want 1/1", observed, refusals)
				}
				stopped, stopConsumer := context.WithCancelCause(ctx)
				defer stopConsumer(nil)
				observed = 0
				v.BlockLinks(stopped, r)(func(block *ssa.BasicBlock, queryErr error) bool {
					if block == nil || queryErr != nil {
						t.Fatalf("consumer prefix=%v,%v, want native block,nil", block, queryErr)
					}
					observed++
					stopConsumer(io.ErrClosedPipe)
					if kind == gotoolchain.SSABlockLinkSuccessors {
						entry.Succs[1] = nil
					} else {
						entry.Index = -1
					}
					return false
				})
				if observed != 1 {
					t.Fatalf("consumer prefix=%d, want 1", observed)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
