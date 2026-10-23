package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

// Value is an open compiler interface. A foreign implementation is not a
// native loan, even if it reports a boolean type or copies a frame identity.
type foreignConditionalScalar struct{ frame *ssa.Function }

func (foreignConditionalScalar) Name() string                  { return "foreign" }
func (foreignConditionalScalar) String() string                { return "foreign" }
func (foreignConditionalScalar) Type() types.Type              { return types.Typ[types.Bool] }
func (v foreignConditionalScalar) Parent() *ssa.Function       { return v.frame }
func (foreignConditionalScalar) Referrers() *[]ssa.Instruction { return nil }
func (foreignConditionalScalar) Pos() token.Pos                { return token.NoPos }

func TestSSAConditionalBranchRefusesForeignValueRepresentations(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		pointer bool
		wantErr error
	}{
		{"scalar implementation", false, core.ErrGoToolchainOutput},
		{"pointer implementation", true, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				block := fn.Blocks[0]
				branch := block.Instrs[len(block.Instrs)-1].(*ssa.If)
				foreign := foreignConditionalScalar{frame: fn}
				branch.Cond = foreign
				if row.pointer {
					branch.Cond = &foreign
				}
				got, queryErr := v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: block})
				if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, row.wantErr) {
					t.Fatalf("foreign value got=%+v,%v, want zero,%v", got, queryErr, row.wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAConditionalBranchForeignRepresentationLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeConditionalSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		request := gotoolchain.SSAConditionalBranchRequest{Block: block}
		got, queryErr := v.ConditionalBranch(ctx, request)
		if queryErr != nil || got.Condition != fn.Params[0] || got.WhenTrue != block.Succs[0] || got.WhenFalse != block.Succs[1] || got.Validate() != nil {
			t.Fatalf("positive native projection got=%+v,%v, want actual flag and original edges,nil", got, queryErr)
		}
		block.Instrs[len(block.Instrs)-1].(*ssa.If).Cond = foreignConditionalScalar{frame: fn}
		got, queryErr = v.ConditionalBranch(ctx, request)
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative foreign projection got=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		got, queryErr = v.ConditionalBranch(ctx, gotoolchain.SSAConditionalBranchRequest{Block: v.Package.Func("Plain").Blocks[0]})
		if got != (gotoolchain.SSAConditionalBranchObservation{}) || queryErr != nil {
			t.Fatalf("neutral nonbranch got=%+v,%v, want zero,nil", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
