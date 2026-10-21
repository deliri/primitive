package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// GoStringLiteralRequest borrows a Go STRING node. Its native lexeme starts
// with a string delimiter; rune tokens and fabricated rune lexemes cannot
// borrow string identity. Go's decoder owns the remaining literal grammar.
type GoStringLiteralRequest struct{ Literal *ast.BasicLit }

func (r GoStringLiteralRequest) Validate() error {
	if r.Literal == nil || r.Literal.Kind != token.STRING || len(r.Literal.Value) == 0 {
		return core.ErrGoToolchainContract
	}
	switch r.Literal.Value[0] {
	case '"', '`':
		return nil
	default:
		return core.ErrGoToolchainContract
	}
}

func (GoStringLiteralRequest) goToolchainProtocolFact() {}

// GoStringLiteralObservation contains decoded Go string bytes. Empty strings
// and non-UTF-8 bytes are valid data; no text extent or normalization is imposed.
type GoStringLiteralObservation struct{ Value string }

func (GoStringLiteralObservation) goToolchainProtocolFact() {}

var (
	_ protocolFact = GoStringLiteralRequest{}
	_ protocolFact = GoStringLiteralObservation{}
)

// ObserveGoStringLiteral delegates Go escape and raw-string interpretation to
// strconv. Native ErrSyntax and custom cancellation remain discoverable, and
// a refusal cannot lend a provisional decoded value.
func ObserveGoStringLiteral(ctx context.Context, request GoStringLiteralRequest) (GoStringLiteralObservation, error) {
	if err := errors.Join(request.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return GoStringLiteralObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	value, err := strconv.Unquote(request.Literal.Value)
	if err = errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return GoStringLiteralObservation{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	return GoStringLiteralObservation{Value: value}, nil
}
