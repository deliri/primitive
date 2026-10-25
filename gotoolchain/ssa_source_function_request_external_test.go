package gotoolchain_test

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"go/ast"
	"testing"
)

func TestSSASourceFunctionRejectsMissingDeclarationSignature(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		original := v.Source.Syntax[0].Decls[0].(*ast.FuncDecl)
		copy := *original
		copy.Type = nil
		request := gotoolchain.SSASourceFunctionRequest{Kind: gotoolchain.SSASourceFunctionDeclaration, Declaration: &copy}
		if validationErr := request.Validate(); !errors.Is(validationErr, core.ErrGoToolchainContract) {
			t.Fatalf("missing declaration signature validation=%v, want native request contract refusal before position lookup", validationErr)
		}
		got, queryErr := v.SourceFunction(ctx, request)
		if got != (gotoolchain.SSASourceFunctionObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("missing declaration signature loan=%+v,%v, want zero and contract refusal", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
