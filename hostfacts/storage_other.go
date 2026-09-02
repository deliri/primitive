//go:build !linux

package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// observeDiskRotation validates the held directory exactly as the Linux
// probe does, then reports that this operating system exposes no portable
// rotation interface. The refusal contract stays uniform across platforms;
// only the answer differs. A platform whose root capability cannot open at
// all has nothing to validate with, and answers unsupported directly.
func observeDiskRotation(ctx context.Context, directory core.AbsolutePath) (DiskRotation, error) {
	root, err := openRoot(ctx, directory)
	if errors.Is(err, core.ErrHostFactsUnsupported) {
		return DiskRotationUnsupported, nil
	}
	if err != nil {
		return DiskRotationUnknown, failRootOpen(OperationOpenRoot, core.ErrHostFactsObservation, err)
	}
	if err := root.close(); err != nil {
		return DiskRotationUnknown, fail(OperationDiskRotation, core.ErrHostFactsObservation, err)
	}
	return DiskRotationUnsupported, nil
}
