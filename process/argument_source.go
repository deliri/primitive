package process

import (
	"context"
	"errors"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ArgumentSource lends exact arguments in invocation order. The owner keeps
// its underlying capture alive until collection returns.
type ArgumentSource func(context.Context) iter.Seq2[Argument, error]

// CollectArguments admits one native argv projection. Only Primitive retains
// that projection; callers can lend a source without collecting a second list.
// The same native execution bounds as ParseArguments apply. A refused source
// never returns a partial argv, including cancellation at the end of a source.
func CollectArguments(ctx context.Context, source ArgumentSource) (arguments []Argument, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, temporal.Wait(temporal.WaitRequest{Context: ctx}))
		if resultErr != nil {
			arguments = nil
		}
	}()
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, core.ErrProcessContract
	}
	values := source(ctx)
	if values == nil {
		return nil, core.ErrProcessContract
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return nil, err
	}
	var projectedBytes uint64
	for argument, sourceErr := range values {
		if sourceErr != nil {
			return nil, sourceErr
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			return nil, err
		}
		if err := argument.Validate(); err != nil {
			return nil, err
		}
		if uint64(len(arguments)) == uint64(ArgumentCountMaximum) {
			return nil, contractError(argumentCountMaximumDiagnostic)
		}
		var err error
		projectedBytes, err = extendProjection(projectionExtension{
			current: projectedBytes, valueBytes: uint64(len(argument.value)),
			maximum: ArgumentProjectionMaximumBytes, diagnostic: argumentProjectionMaximumDiagnostic,
		})
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
	}
	return arguments, nil
}
