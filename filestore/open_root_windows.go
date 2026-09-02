//go:build windows

package filestore

import (
	"errors"
	"io/fs"
	"os"
)

// openRootDirectory refuses a blocking or redirected leaf before acquisition,
// then proves the rooted capability still names the inspected directory.
func openRootDirectory(path string) (*os.Root, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if windowsReparsePoint(before) || !before.IsDir() {
		return nil, fs.ErrInvalid
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	after, statErr := root.Stat(".")
	validationErr := errors.Join(statErr, validateWindowsSameDirectory(before, after))
	if validationErr != nil {
		return nil, errors.Join(validationErr, root.Close())
	}
	return root, nil
}
