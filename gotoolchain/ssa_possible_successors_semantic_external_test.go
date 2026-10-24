package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"strings"
	"testing"
)

func nativePossibleSuccessorsBody(mode uint8) string {
	condition := "true"
	switch mode & 3 {
	case 1:
		condition = "false"
	case 2:
		condition = "flag"
	case 3:
		condition = "!flag"
	}
	body := "if " + condition + "{work()}else{work()};work()"
	switch (mode >> 2) & 3 {
	case 1:
		body = "if " + condition + "{work()};work()"
	case 2:
		body = "if " + condition + "{}else{work()};work()"
	case 3:
		body = "if " + condition + "{work();goto done};work();done:work()"
	}
	return strings.Repeat("noise();", int(mode>>6)) + body
}
func nativePossibleSuccessorsSemanticObservation(ctx context.Context, t *testing.T, v gotoolchain.SSAUnitView, fn *ssa.Function, mode uint8) {
	t.Helper()
	entry := fn.Blocks[0]
	if len(entry.Succs) != 2 {
		t.Fatalf("mode=%d native edges got=%d, want two", mode, len(entry.Succs))
	}
	prefix := 0
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == v.Package.Func("noise") {
				prefix++
			}
		}
	}
	if prefix != int(mode>>6) {
		t.Fatalf("mode=%d native prefix got=%d, want %d", mode, prefix, mode>>6)
	}
	branch := entry.Instrs[len(entry.Instrs)-1].(*ssa.If)
	damage := (mode >> 4) & 3
	var wantErr error
	switch damage {
	case 1:
		branch.Cond = nil
		wantErr = core.ErrGoToolchainOutput
	case 2:
		copied := *entry.Succs[0]
		entry.Succs[0] = &copied
		wantErr = core.ErrGoToolchainOutput
	case 3:
		missing := ssa.NewConst(nil, types.Typ[types.Bool])
		missing.Value = nil
		branch.Cond = missing
		wantErr = core.ErrGoToolchainOutput
	}
	seen, refusals := 0, 0
	for block, queryErr := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: entry, Kind: gotoolchain.SSABlockLinkPossibleSuccessors}) {
		if wantErr != nil {
			if block != nil || !errors.Is(queryErr, wantErr) {
				t.Fatalf("mode=%d refusal got=%v,%v, want nil,%v", mode, block, queryErr, wantErr)
			}
			refusals++
			continue
		}
		index := seen
		if mode&3 <= 1 {
			index = int(mode & 3)
		}
		if queryErr != nil || index >= 2 || block != entry.Succs[index] || seen > 0 && mode&3 <= 1 {
			t.Fatalf("mode=%d possible edge got=%v,%v ordinal=%d, want native edge index=%d", mode, block, queryErr, seen, index)
		}
		seen++
	}
	wantCount, wantRefusals := 2, 0
	if mode&3 <= 1 {
		wantCount = 1
	}
	if wantErr != nil {
		wantCount, wantRefusals = 0, 1
	}
	if seen != wantCount || refusals != wantRefusals {
		t.Fatalf("mode=%d result got=%d,%d, want %d,%d", mode, seen, refusals, wantCount, wantRefusals)
	}
}
func TestSSAPossibleSuccessorsFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	var source strings.Builder
	source.WriteString(`package sample;func work(){};func noise(){};`)
	for mode := 0; mode <= 255; mode++ {
		fmt.Fprintf(&source, "func Entry%d(flag bool){%s};", mode, nativePossibleSuccessorsBody(uint8(mode)))
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source.String())
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for mode := 0; mode <= 255; mode++ {
			nativePossibleSuccessorsSemanticObservation(ctx, t, v, v.Package.Func(fmt.Sprintf("Entry%d", mode)), uint8(mode))
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
func FuzzSSAPossibleSuccessorsAuthorityConservation(f *testing.F) {
	for _, mode := range []uint8{0, 1, 2, 7, 16, 32, 48, 255} {
		f.Add(mode)
	}
	f.Fuzz(func(t *testing.T, mode uint8) {
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func work(){};func noise(){};func Entry(flag bool){`+nativePossibleSuccessorsBody(mode)+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			nativePossibleSuccessorsSemanticObservation(ctx, t, v, v.Package.Func("Entry"), mode)
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit got=%v, want nil", err)
		}
	})
}
