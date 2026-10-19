package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// SyntaxWalkRequest borrows an existing Go syntax tree. Parsing, compilation
// and policy remain separate operations. Go owns traversal and ancestry.
type SyntaxWalkRequest struct {
	Root ast.Node
}

func (r SyntaxWalkRequest) Validate() error {
	if r.Root == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (SyntaxWalkRequest) goToolchainProtocolFact() {}

// SyntaxNodeObservation lends one native node and its root-to-parent path.
// Ancestors excludes Node and remains valid only until the consumer returns.
// A consumer retaining the path must copy it under its own memory contract.
type SyntaxNodeObservation struct {
	Node      ast.Node
	Ancestors []ast.Node
}

func (o SyntaxNodeObservation) Validate() error {
	if o.Node == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (SyntaxNodeObservation) goToolchainProtocolFact() {}

var (
	_ protocolFact = SyntaxWalkRequest{}
	_ protocolFact = SyntaxNodeObservation{}
)

// WalkSyntax uses Go's preorder walker and its depth-sized traversal stack.
// It retains no node inventory. Consumer refusal and native cancellation stop
// observations before another node is lent. Consumer panics retain Go's normal
// propagation; malformed trees retain the native walker's panic behavior.
func WalkSyntax(ctx context.Context, request SyntaxWalkRequest) iter.Seq2[SyntaxNodeObservation, error] {
	return func(yield func(SyntaxNodeObservation, error) bool) {
		if err := errors.Join(request.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
			yield(SyntaxNodeObservation{}, err)
			return
		}
		stopped := false
		ast.PreorderStack(request.Root, nil, func(node ast.Node, ancestors []ast.Node) bool {
			if stopped {
				return false
			}
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				stopped = true
				yield(SyntaxNodeObservation{}, err)
				return false
			}
			stopped = !yield(SyntaxNodeObservation{Node: node, Ancestors: ancestors}, nil)
			return !stopped
		})
	}
}
