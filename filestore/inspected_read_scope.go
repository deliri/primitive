package filestore

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// InspectedReadSource exposes the native file's read capabilities without
// lending write, truncate or close authority.
type InspectedReadSource interface {
	io.ReadSeeker
	io.ReaderAt
}

// InspectedRead lends bytes and metadata from the same held native object.
// The reader is borrowed synchronously; it must not be retained or closed.
// Replacing the source pathname cannot replace this descriptor's observation.
type InspectedRead struct {
	Source   InspectedReadSource
	Metadata Inspection
}

func (r InspectedRead) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrFilestoreContract
	}
	return r.Metadata.Validate()
}

type InspectedReadScopeRequest struct {
	Location Location
	Use      func(context.Context, InspectedRead) error
}

func (r InspectedReadScopeRequest) Validate() error {
	if r.Use == nil {
		return errors.Join(core.ErrFilestoreContract, r.Location.Validate())
	}
	return r.Location.Validate()
}

// WithInspectedReadScope acquires, observes, lends and closes one regular file.
// Acquisition refusal returns no completed scope. Metadata or consumer failure,
// cancellation and panic retain the native close owner and return no invented
// lifecycle authority. Native causes remain inspectable in the scope result.
func WithInspectedReadScope(ctx context.Context, request InspectedReadScopeRequest) (result FileScopeResult, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, resultErr)
		result.operationError = scopeCancellationError(ctx, result.operationError)
	}()

	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return FileScopeResult{}, err
	}
	file, err := OpenRead(ctx, ReadHandleRequest{Location: request.Location})
	if err != nil {
		return FileScopeResult{}, err
	}
	result = finishFileScope(ctx, file, func(ctx context.Context, file *os.File) error {
		metadata, err := InspectOpenFile(ctx, HandleInspectionRequest{File: file})
		if err != nil {
			return err
		}
		err = request.Use(ctx, InspectedRead{Source: file, Metadata: metadata})
		return errors.Join(err, contextstate.Validate(ctx), context.Cause(ctx))
	})
	return result, result.Validate()
}

var _ core.Validatable = InspectedRead{}
var _ core.Validatable = InspectedReadScopeRequest{}
