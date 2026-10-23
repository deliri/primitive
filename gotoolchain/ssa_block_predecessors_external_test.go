package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativePredecessorSource = `package sample;func left(){};func right(){};func join(){};func Entry(flag bool){if flag{left()}else{right()};join()};func Other(){join()};func Loop(flag bool){for flag{left()};join()};func Constant(){if false{left()};join()};func Closure(flag bool){_=func(){if flag{left()}else{right()};join()}}`

func TestSSABlockPredecessorsUseCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("predecessor owner got=%+v,%v, want compiled capability,nil", match, err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSABlockLinkRequest]().PkgPath() != path {
		t.Fatalf("predecessor request owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSABlockLinkRequest]().PkgPath(), err, path)
	}
}

func TestSSABlockPredecessorsPreserveStructuralOrder(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for _, name := range []string{"Entry", "Closure"} {
			fn := v.Package.Func(name)
			if name == "Closure" {
				fn = fn.AnonFuncs[0]
			}
			entry := fn.Blocks[0]
			if len(entry.Succs) != 2 || len(entry.Succs[0].Succs) != 1 || len(entry.Succs[1].Succs) != 1 || entry.Succs[0].Succs[0] != entry.Succs[1].Succs[0] {
				t.Fatalf("%s native diamond got=%v, want two arms and shared join", name, entry.Succs)
			}
			left, right := entry.Succs[0], entry.Succs[1]
			join := left.Succs[0]
			for _, row := range []struct {
				name  string
				block *ssa.BasicBlock
				want  []*ssa.BasicBlock
			}{
				{"entry", entry, nil}, {"left", left, []*ssa.BasicBlock{entry}}, {"right", right, []*ssa.BasicBlock{entry}}, {"join", join, []*ssa.BasicBlock{left, right}},
			} {
				seen := 0
				for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: row.block, Kind: gotoolchain.SSABlockLinkPredecessors}) {
					if queryErr != nil || seen >= len(row.want) || predecessor != row.want[seen] {
						t.Fatalf("%s/%s predecessor[%d] got=%v,%v, want source ingress %v", name, row.name, seen, predecessor, queryErr, row.want)
					}
					seen++
				}
				if seen != len(row.want) {
					t.Fatalf("%s/%s ingress count got=%d, want %d", name, row.name, seen, len(row.want))
				}
			}
		}
		loop := v.Package.Func("Loop")
		entry := loop.Blocks[0]
		header := entry.Succs[0]
		if len(header.Succs) != 2 {
			t.Fatalf("loop header successors got=%v, want body and exit", header.Succs)
		}
		body := header.Succs[0]
		want := [2]*ssa.BasicBlock{entry, body}
		seen := 0
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: header, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil || seen >= len(want) || predecessor != want[seen] {
				t.Fatalf("loop ingress[%d] got=%v,%v, want %v", seen, predecessor, queryErr, want)
			}
			seen++
		}
		if seen != 2 {
			t.Fatalf("loop ingress count got=%d, want entry and backedge", seen)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABlockPredecessorsRetainUntakenIncomingEdges(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Constant").Blocks[0]
		untaken := entry.Succs[0]
		join := entry.Succs[1]
		// The join has both the constant-false direct edge and the structural
		// edge from the untaken body. Runtime filtering must not shrink Preds.
		want := [2]*ssa.BasicBlock{entry, untaken}
		seen := 0
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: join, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil || seen >= len(want) || predecessor != want[seen] {
				t.Fatalf("constant join predecessor[%d] got=%v,%v, want %v", seen, predecessor, queryErr, want)
			}
			seen++
		}
		if seen != 2 {
			t.Fatalf("constant join ingress got=%d, want two structural edges", seen)
		}
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: untaken, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			t.Fatalf("untaken selected block got=%v,%v, want no observation", predecessor, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABlockPredecessorsRefuseUnownedEdges(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		damage  func(gotoolchain.SSAUnitView, *ssa.BasicBlock)
		wantErr error
	}{
		{"nil edge", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Preds[0] = nil }, core.ErrGoToolchainOutput},
		{"parentless edge", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Preds[0] = &ssa.BasicBlock{} }, core.ErrGoToolchainOutput},
		{"foreign edge", func(v gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Preds[0] = v.Package.Func("Other").Blocks[0] }, core.ErrGoToolchainOutput},
		{"detached edge", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { copy := *b.Preds[0]; b.Preds[0] = &copy }, core.ErrGoToolchainOutput},
		{"negative edge ordinal", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Preds[0].Index = -1 }, core.ErrGoToolchainOutput},
		{"edge ordinal at width", func(_ gotoolchain.SSAUnitView, b *ssa.BasicBlock) { b.Preds[0].Index = len(b.Parent().Blocks) }, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				block := v.Package.Func("Entry").Blocks[0].Succs[0]
				row.damage(v, block)
				refusals := 0
				for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: block, Kind: gotoolchain.SSABlockLinkPredecessors}) {
					if predecessor != nil || !errors.Is(queryErr, row.wantErr) {
						t.Fatalf("malformed ingress got=%v,%v, want nil,%v", predecessor, queryErr, row.wantErr)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("ingress refusals got=%d, want exactly one", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSABlockPredecessorsProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		root := v.Package.Func("Entry").Blocks[0]
		arm := root.Succs[0]
		seen := 0
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: arm, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil || predecessor != root {
				t.Fatalf("positive ingress got=%v,%v, want root=%d,nil", predecessor, queryErr, root.Index)
			}
			seen++
		}
		if seen != 1 {
			t.Fatalf("positive ingress count got=%d, want one", seen)
		}
		copy := *root
		arm.Preds[0] = &copy
		refused := 0
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: arm, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if predecessor != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
				t.Fatalf("negative ingress got=%v,%v, want nil,output refusal", predecessor, queryErr)
			}
			refused++
		}
		if refused != 1 {
			t.Fatalf("negative refusals got=%d, want one", refused)
		}
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: root, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			t.Fatalf("neutral entry got=%v,%v, want no ingress", predecessor, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABlockPredecessorsConserveStopAndCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		join := entry.Succs[0].Succs[0]
		first := join.Preds[0]
		second := join.Preds[1]
		join.Preds[1] = nil
		seen := 0
		v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: join, Kind: gotoolchain.SSABlockLinkPredecessors})(func(predecessor *ssa.BasicBlock, queryErr error) bool {
			seen++
			if queryErr != nil || predecessor != first {
				t.Errorf("stopped ingress got=%v,%v, want first=%d,nil before malformed tail", predecessor, queryErr, first.Index)
			}
			return false
		})
		if seen != 1 {
			t.Fatalf("stopped ingress count got=%d, want one", seen)
		}
		join.Preds[1] = second
		parent, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		seen, refused := 0, 0
		for predecessor, queryErr := range v.BlockLinks(parent, gotoolchain.SSABlockLinkRequest{Block: join, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil {
				if predecessor != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
					t.Fatalf("cancelled ingress got=%v,%v, want nil,cancelled with cause", predecessor, queryErr)
				}
				refused++
				continue
			}
			if predecessor != first {
				t.Fatalf("first ingress got=%v, want first=%d", predecessor, first.Index)
			}
			seen++
			cancel(io.ErrClosedPipe)
		}
		if seen != 1 || refused != 1 {
			t.Fatalf("cancelled stream got values=%d refusals=%d, want one each", seen, refused)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSABlockPredecessorsPreserveRepeatedNativeEdges(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePredecessorSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		entry := v.Package.Func("Entry").Blocks[0]
		arm := entry.Succs[0]
		arm.Preds = append(arm.Preds, entry)
		seen := 0
		for predecessor, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: arm, Kind: gotoolchain.SSABlockLinkPredecessors}) {
			if queryErr != nil || predecessor != entry {
				t.Fatalf("repeated edge[%d] got=%v,%v, want entry=%d,nil", seen, predecessor, queryErr, entry.Index)
			}
			seen++
		}
		if seen != 2 {
			t.Fatalf("native edge multiplicity got=%d, want two loans of the same native pointer", seen)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
