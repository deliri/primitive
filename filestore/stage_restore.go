package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StageRestoreRequest discards a native stage tail and resumes writing at the
// retained prefix. Prefix cannot extend the observed native file. Zero means
// an intentionally empty prefix, not absent intent.
type StageRestoreRequest struct {
	Destination *StageDestination
	Prefix      core.ByteLength
}

func (r StageRestoreRequest) Validate() error {
	return errors.Join(r.Destination.Validate(), r.Prefix.Validate())
}

// StageRestoreObservation records a completed native truncate-and-seek.
// Its zero value is invalid. It does not prove synchronization or publication.
type StageRestoreObservation struct {
	beforeBytes core.ByteLength
	afterBytes  core.ByteLength
	offset      core.ByteLength
	restored    bool
}

func (o StageRestoreObservation) BeforeBytes() core.ByteLength { return o.beforeBytes }
func (o StageRestoreObservation) AfterBytes() core.ByteLength  { return o.afterBytes }
func (o StageRestoreObservation) Offset() core.ByteLength      { return o.offset }

func (o StageRestoreObservation) Validate() error {
	if !o.restored {
		return contractError(errors.New("filestore stage restoration was not observed"))
	}
	if o.beforeBytes.Uint64() < o.afterBytes.Uint64() || o.afterBytes != o.offset {
		return contractError(errors.New("filestore stage restoration coordinates disagree"))
	}
	return errors.Join(o.beforeBytes.Validate(), o.afterBytes.Validate(), o.offset.Validate())
}

// RestoreStage performs the native mechanical operations without retaining a
// cursor or file model. Failure after mutation is explicitly indeterminate;
// the caller must abandon the owned stage rather than publish uncertain work.
func RestoreStage(ctx context.Context, request StageRestoreRequest) (StageRestoreObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return StageRestoreObservation{}, err
	}
	if err := request.Validate(); err != nil {
		return StageRestoreObservation{}, err
	}
	before, err := observeStageLength(request.Destination.file)
	if err != nil {
		return StageRestoreObservation{}, err
	}
	if request.Prefix.Uint64() > before.Uint64() {
		return StageRestoreObservation{}, sizeError(errors.New("filestore restore prefix exceeds native extent"))
	}
	if err := restoreStageCursor(request); err != nil {
		return StageRestoreObservation{}, err
	}
	after, err := observeStageLength(request.Destination.file)
	if err != nil {
		return StageRestoreObservation{}, indeterminateActivationError(err)
	}
	if after != request.Prefix {
		return StageRestoreObservation{}, indeterminateActivationError(errors.New("filestore restored extent differs"))
	}
	return StageRestoreObservation{beforeBytes: before, afterBytes: after, offset: request.Prefix, restored: true}, nil
}

func restoreStageCursor(request StageRestoreRequest) error {
	prefix, err := request.Prefix.Int64()
	if err != nil {
		return sizeError(err)
	}
	if err := request.Destination.file.Truncate(prefix); err != nil {
		return indeterminateActivationError(err)
	}
	offset, err := request.Destination.file.Seek(prefix, io.SeekStart)
	if err != nil {
		return indeterminateActivationError(err)
	}
	if offset != prefix {
		return indeterminateActivationError(errors.New("filestore restored cursor differs"))
	}
	return nil
}

var (
	_ core.Validatable = StageRestoreRequest{}
	_ core.Validatable = StageRestoreObservation{}
)
