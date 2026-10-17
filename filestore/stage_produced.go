package filestore

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StageProductionRequest lends a writer synchronously. Produce must finish
// before returning and must not close or retain the writer. Destination owns
// the temporary name, mode and optional exact extent; no target is published.
type StageProductionRequest struct {
	Produce     func(context.Context, io.Writer) error
	Destination StageDestinationRequest
}

func (r StageProductionRequest) Validate() error {
	if r.Produce == nil {
		return core.ErrFilestoreContract
	}
	return r.Destination.Validate()
}

// StageProduced owns creation, producer completion, sync and closure. A
// producer refusal, panic or cancellation abandons only the created inode and
// returns no staged capability. Only a completed producer can reach Commit.
func StageProduced(ctx context.Context, request StageProductionRequest) (StagedFile, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return StagedFile{}, err
	}
	destination, err := OpenStageDestination(ctx, request.Destination)
	if err != nil {
		return StagedFile{}, err
	}
	file, err := destination.File()
	if err != nil {
		return StagedFile{}, errors.Join(err, AbandonStageDestination(destination))
	}
	produceErr := useFileScope(ctx, file, func(ctx context.Context, file *os.File) error {
		return request.Produce(ctx, file)
	})
	if err := errors.Join(produceErr, contextstate.Validate(ctx), context.Cause(ctx)); err != nil {
		return StagedFile{}, errors.Join(err, AbandonStageDestination(destination))
	}
	return FinishStageDestination(ctx, destination)
}

var _ core.Validatable = StageProductionRequest{}
