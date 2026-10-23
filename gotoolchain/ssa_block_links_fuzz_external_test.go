package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func FuzzSSABlockLinksConserveNativeAuthority(f *testing.F) {
	for _, seed := range []struct{ mode, selected uint8 }{{0, 0}, {1, 1}, {2, 2}, {4, 3}, {8, 0}, {16, 0}, {32, 1}} {
		f.Add(seed.mode, seed.selected)
	}
	f.Fuzz(func(t *testing.T, mode, selector uint8) {
		condition := "flag"
		if mode%3 == 1 {
			condition = "false"
		} else if mode%3 == 2 {
			condition = "true"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(flag bool){if `+condition+`{invoke()}else{invoke()};invoke()}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			entry := v.Package.Func("Entry").Blocks[0]
			if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
				t.Fatal("fuzz native diamond absent, want two arms and join")
			}
			blocks := []*ssa.BasicBlock{entry, entry.Succs[0], entry.Succs[1], entry.Succs[0].Succs[0]}
			selected := int(selector % 4)
			kind := gotoolchain.SSABlockLinkSuccessors
			var want []*ssa.BasicBlock
			dead := selected == 1 && mode%3 == 1 || selected == 2 && mode%3 == 2
			if mode&4 != 0 {
				kind = gotoolchain.SSABlockLinkDominatorAncestry
				if !dead {
					want = append(want, blocks[selected])
					if selected != 0 {
						want = append(want, entry)
					}
				}
			} else if !dead {
				if selected == 0 {
					if mode%3 != 1 {
						want = append(want, blocks[1])
					}
					if mode%3 != 2 {
						want = append(want, blocks[2])
					}
				} else if selected != 3 {
					want = append(want, blocks[3])
				}
			}
			if mode&16 != 0 && len(want) > 1 {
				want = want[:1]
			}
			r := gotoolchain.SSABlockLinkRequest{Block: blocks[selected], Kind: kind}
			if mode&32 != 0 {
				copied := *r.Block
				r.Block = &copied
			}
			queryCtx, cancel := context.WithCancelCause(ctx)
			defer cancel(nil)
			var wantErr error
			if mode&32 != 0 {
				wantErr = core.ErrGoToolchainOutput
			}
			if mode&8 != 0 {
				cancel(io.ErrClosedPipe)
				wantErr = io.ErrClosedPipe
			}
			if wantErr != nil {
				want = nil
			}
			var got []*ssa.BasicBlock
			refusals := 0
			for block, queryErr := range v.BlockLinks(queryCtx, r) {
				if queryErr != nil {
					if block != nil || wantErr == nil || !errors.Is(queryErr, wantErr) || errors.Is(queryErr, core.ErrGoToolchainOutput) != (wantErr == core.ErrGoToolchainOutput) {
						t.Fatalf("fuzz refusal=%v,%v, want nil,%v", block, queryErr, wantErr)
					}
					refusals++
					continue
				}
				got = append(got, block)
				if mode&16 != 0 {
					break
				}
			}
			wantRefusals := 0
			if wantErr != nil {
				wantRefusals = 1
			}
			if !slices.Equal(got, want) || refusals != wantRefusals {
				t.Fatalf("fuzz mode=%d selector=%d relations/refusals=%v/%d, want %v/%d", mode, selected, got, refusals, want, wantRefusals)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
