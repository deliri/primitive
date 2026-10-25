package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAFunctionAncestorsMalformedLoans(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAFunctionAncestorsRequest)
		want   error
		loans  int
	}{
		{"missing function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) { r.Function = nil }, core.ErrGoToolchainContract, 0},
		{"missing source", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAFunctionAncestorsRequest) { v.Source = nil }, core.ErrGoToolchainContract, 0},
		{"missing program", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAFunctionAncestorsRequest) { v.Program = nil }, core.ErrGoToolchainContract, 0},
		{"missing package", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAFunctionAncestorsRequest) { v.Package = nil }, core.ErrGoToolchainContract, 0},
		{"foreign selected program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) { r.Function.Prog = nil }, core.ErrGoToolchainContract, 0},
		{"foreign selected package", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) { r.Function.Pkg = nil }, core.ErrGoToolchainContract, 0},
		{"missing selected signature", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Signature = nil
		}, core.ErrGoToolchainOutput, 0},
		{"copied selected frame", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			clone := *r.Function
			r.Function = &clone
		}, core.ErrGoToolchainOutput, 0},
		{"nil native entry block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Blocks[0] = nil
		}, core.ErrGoToolchainOutput, 0},
		{"wrong native entry block", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Blocks[0] = v.Package.Func("Instantiate").Blocks[0]
		}, core.ErrGoToolchainOutput, 0},
		{"wrong native entry ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Blocks[0].Index = 1
		}, core.ErrGoToolchainOutput, 0},
		{"absent native child membership", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Parent().AnonFuncs = nil
		}, core.ErrGoToolchainOutput, 0},
		{"broken parent signature", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Parent().Signature = nil
		}, core.ErrGoToolchainOutput, 1},
		{"broken parent program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Parent().Prog = nil
		}, core.ErrGoToolchainOutput, 1},
		{"broken parent package", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			r.Function.Parent().Pkg = nil
		}, core.ErrGoToolchainOutput, 1},
		{"copied intermediate membership", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAFunctionAncestorsRequest) {
			middle := r.Function.Parent()
			clone := *middle
			middle.Parent().AnonFuncs[0] = &clone
		}, core.ErrGoToolchainOutput, 1},
		{"broken root signature", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAFunctionAncestorsRequest) {
			v.Package.Func("Entry").Signature = nil
		}, core.ErrGoToolchainOutput, 2},
		{"broken root block", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAFunctionAncestorsRequest) {
			v.Package.Func("Entry").Blocks[0] = nil
		}, core.ErrGoToolchainOutput, 2},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionAncestorsSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSAFunctionAncestorsRequest{Function: v.Package.Func("Entry").AnonFuncs[0].AnonFuncs[0]}
				intact := 0
				for fn, queryErr := range v.FunctionAncestors(ctx, r) {
					if fn == nil || queryErr != nil {
						t.Fatalf("intact ancestor=%v,%v", fn, queryErr)
					}
					intact++
				}
				if intact != 3 {
					t.Fatalf("intact ancestors=%d, want3", intact)
				}
				row.mutate(&v, &r)
				loans, refusals := 0, 0
				for fn, queryErr := range v.FunctionAncestors(ctx, r) {
					if queryErr != nil {
						if fn != nil || !errors.Is(queryErr, row.want) {
							t.Fatalf("damaged ancestor=%v,%v, want nil,%v", fn, queryErr, row.want)
						}
						refusals++
						continue
					}
					if fn == nil || refusals != 0 {
						t.Fatalf("invalid or post-refusal loan=%v", fn)
					}
					loans++
				}
				if loans != row.loans || refusals != 1 {
					t.Fatalf("damaged ancestor loans/refusals=%d/%d, want%d/1", loans, refusals, row.loans)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSSAFunctionAncestorsNativeGenericInstantiation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionAncestorsSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var instance *ssa.Function
		for _, block := range v.Package.Func("Instantiate").Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					instance = call.Common().StaticCallee()
				}
			}
		}
		if instance == nil || instance.Origin() != v.Package.Func("Generic") || len(instance.TypeArgs()) != 1 {
			t.Fatal("fixture lost its real native generic instantiation")
		}
		loans := 0
		for fn, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: instance}) {
			if queryErr != nil || fn != instance || loans != 0 {
				t.Fatalf("generic ancestor=%v,%v after%d loans", fn, queryErr, loans)
			}
			loans++
		}
		if loans != 1 {
			t.Fatalf("generic loans=%d, want1", loans)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSAFunctionAncestorsBodylessDeclarationKeepsIdentity(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\nimport _ \"unsafe\"\n//go:linkname Entry\nfunc Entry(value int) int\n")
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if fn == nil || len(fn.Blocks) != 0 {
			t.Fatal("fixture lost native bodyless declaration")
		}
		loans := 0
		for selected, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: fn}) {
			if selected != fn || queryErr != nil {
				t.Fatalf("bodyless ancestor=%v,%v, want exact declaration", selected, queryErr)
			}
			loans++
		}
		if loans != 1 {
			t.Fatalf("bodyless loans=%d, want1", loans)
		}
		clone := *fn
		if &clone == fn || clone.Object() != fn.Object() {
			t.Fatal("bodyless copy changed declaration instead of frame identity")
		}
		refusals := 0
		for selected, queryErr := range v.FunctionAncestors(ctx, gotoolchain.SSAFunctionAncestorsRequest{Function: &clone}) {
			if selected != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
				t.Fatalf("copied bodyless ancestor=%v,%v", selected, queryErr)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("bodyless copy refusals=%d, want1", refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
