package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
)

// InspectEntry observes one entry through the borrowed root's native handle.
// It does not follow the final symbolic link, open content or close the root.
// Missing and inaccessible entries preserve their native error rather than
// producing an inspection. Renaming the root's pathname cannot redirect it.
func InspectEntry(ctx context.Context, location Location) (observed Inspection, resultErr error) {
	defer func() {
		if resultErr != nil {
			observed = Inspection{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), location.Validate()); err != nil {
		return Inspection{}, err
	}
	info, err := location.Root.Lstat(location.Path.String())
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	observed, err = inspectionForEntry(info)
	return observed, errors.Join(err, contextstate.Validate(ctx))
}
