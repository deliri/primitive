package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// HandleInspectionRequest borrows one already-open native file. The caller
// owns its lifetime and coordinates concurrent changes to the held object.
type HandleInspectionRequest struct {
	File *os.File
}

// Validate refuses an absent handle. A closed handle remains a native source
// failure observed during execution, rather than an invented lifecycle state.
func (r HandleInspectionRequest) Validate() error {
	if r.File == nil {
		return contractError(errors.New("file inspection names no open handle"))
	}
	return nil
}

// InspectOpenFile observes the held descriptor, even if its former name has
// been replaced or removed. It reads no content, changes no offset, and closes
// nothing. Native failures preserve their identity; a path is never reopened
// as a substitute for the caller's exact object.
func InspectOpenFile(ctx context.Context, request HandleInspectionRequest) (Inspection, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Inspection{}, err
	}
	if err := request.Validate(); err != nil {
		return Inspection{}, err
	}
	info, err := request.File.Stat()
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	return inspectionForEntry(info)
}
