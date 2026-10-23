package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSABlockLinksKeepUntakenAndLexicalAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body                           string
		wantSuccessors, wantSelectedAncestry int
	}{
		{"constant false hides first arm", `if false{invoke()}else{invoke()};invoke()`, 1, 0},
		{"constant true keeps first arm", `if true{invoke()}else{invoke()};invoke()`, 1, 2},
		{"runtime condition keeps both arms", `if flag{invoke()}else{invoke()};invoke()`, 2, 2},
		{"goto join remains reachable", `if flag{goto body};if false{invoke()}else{invoke()};body:invoke()`, 2, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){`+row.body+`};func Closure(){_=func(){invoke()}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				entry := fn.Blocks[0]
				if len(entry.Succs) != 2 {
					t.Fatalf("native branch edges=%d, want 2", len(entry.Succs))
				}
				successors := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkSuccessors}) {
					if queryErr != nil || block == nil || block.Parent() != fn {
						t.Fatalf("reachable successor=%v,%v, want actual same-frame block,nil", block, queryErr)
					}
					successors++
				}
				ancestry := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry.Succs[0], Kind: gotoolchain.SSABlockLinkDominatorAncestry}) {
					if queryErr != nil || block == nil {
						t.Fatalf("selected ancestry=%v,%v, want native block,nil", block, queryErr)
					}
					ancestry++
				}
				if successors != row.wantSuccessors || ancestry != row.wantSelectedAncestry {
					t.Fatalf("reachable successors/ancestry=%d/%d, want %d/%d", successors, ancestry, row.wantSuccessors, row.wantSelectedAncestry)
				}
				outer := v.Package.Func("Closure")
				if len(outer.AnonFuncs) != 1 {
					t.Fatalf("native children=%d, want 1", len(outer.AnonFuncs))
				}
				child := outer.AnonFuncs[0]
				observed := 0
				for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: child.Blocks[0], Kind: gotoolchain.SSABlockLinkDominatorAncestry}) {
					if queryErr != nil || block != child.Blocks[0] {
						t.Fatalf("child ancestry=%v,%v, want child entry,nil", block, queryErr)
					}
					observed++
				}
				if observed != 1 {
					t.Fatalf("child ancestry=%d, want 1 without outer entry", observed)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockLinksObserveFinalCancellation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name       string
		kind       gotoolchain.SSABlockLinkKind
		selected   int
		wantPrefix int
	}{
		{"empty successors", gotoolchain.SSABlockLinkSuccessors, 3, 0},
		{"untaken selected successors", gotoolchain.SSABlockLinkSuccessors, 1, 0},
		{"untaken selected ancestry", gotoolchain.SSABlockLinkDominatorAncestry, 1, 0},
		{"last successor", gotoolchain.SSABlockLinkSuccessors, 2, 1},
		{"last ancestor", gotoolchain.SSABlockLinkDominatorAncestry, 0, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){if false{invoke()}else{invoke()};invoke()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				entry := v.Package.Func("Entry").Blocks[0]
				if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 {
					t.Fatal("native branch absent, want constant diamond")
				}
				blocks := []*ssa.BasicBlock{entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
				parent, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				queryCtx := context.Context(parent)
				if row.wantPrefix == 0 {
					queryCtx = &blockLinkTerminalContext{Context: parent, cancel: cancel}
				}
				observed, refusals := 0, 0
				for block, queryErr := range v.BlockLinks(queryCtx, gotoolchain.SSABlockLinkRequest{Block: blocks[row.selected], Kind: row.kind}) {
					if queryErr != nil {
						if block != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
							t.Fatalf("final refusal=%v,%v, want nil,cancelled with closed pipe", block, queryErr)
						}
						refusals++
						continue
					}
					observed++
					cancel(io.ErrClosedPipe)
				}
				if observed != row.wantPrefix || refusals != 1 {
					t.Fatalf("final observations/refusals=%d/%d, want %d/1", observed, refusals, row.wantPrefix)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

// The first context observation admits the query. Cancellation on its second
// observation forces even an empty result to own its final context boundary.
type blockLinkTerminalContext struct {
	context.Context
	cancel context.CancelCauseFunc
	checks int
}

func (c *blockLinkTerminalContext) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel(io.ErrClosedPipe)
	}
	return c.Context.Err()
}

func TestSSABlockLinksLendLoopBackedgeOnce(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){for flag{invoke()};invoke()}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		if len(entry.Succs) != 1 || len(entry.Succs[0].Succs) != 2 {
			t.Fatal("native loop absent, want entry, header, body and exit")
		}
		header := entry.Succs[0]
		body := header.Succs[0]
		if len(body.Succs) != 1 || body.Succs[0] != header {
			t.Fatal("native backedge absent, want body to header")
		}
		observed := 0
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: body, Kind: gotoolchain.SSABlockLinkSuccessors}) {
			if queryErr != nil || block != header {
				t.Fatalf("loop successor=%v,%v, want actual header,nil", block, queryErr)
			}
			observed++
		}
		if observed != 1 {
			t.Fatalf("loop successor observations=%d, want 1 without graph traversal", observed)
		}
		want := []*ssa.BasicBlock{body, header, entry}
		observed = 0
		for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: body, Kind: gotoolchain.SSABlockLinkDominatorAncestry}) {
			if queryErr != nil || observed >= len(want) || block != want[observed] {
				t.Fatalf("loop ancestor[%d]=%v,%v, want body/header/entry", observed, block, queryErr)
			}
			observed++
		}
		if observed != len(want) {
			t.Fatalf("loop ancestor observations=%d, want %d", observed, len(want))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
