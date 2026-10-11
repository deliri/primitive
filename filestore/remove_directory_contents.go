package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RemoveDirectoryContents durably removes descendants while preserving the
// directory itself. Every iteration closes its native directory reader before
// removal and reopens at the beginning: no continuation cookie crosses a
// mutation of that directory. It retains one entry, with no cardinality quota.
func RemoveDirectoryContents(ctx context.Context, location Location) error {
	if err := location.Validate(); err != nil {
		return err
	}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		entry, err := firstDirectoryRemovalEntry(location)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return sourceError(err)
		}
		component, err := core.ParsePathComponent(entry.Name())
		if err != nil {
			return sourceError(err)
		}
		child, err := location.Path.Join(component)
		if err != nil {
			return sourceError(err)
		}
		if err := RemoveTree(ctx, TreeRemovalRequest{Location: Location{Root: location.Root, Path: child}}); err != nil {
			return err
		}
	}
}

func firstDirectoryRemovalEntry(location Location) (fs.DirEntry, error) {
	directory, err := openDirectory(location.Root, location.Path.String())
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		if readErr == io.EOF && closeErr == nil && len(entries) == 0 {
			return nil, io.EOF
		}
		return nil, errors.Join(readErr, closeErr)
	}
	if len(entries) != 1 {
		return nil, io.ErrNoProgress
	}
	return entries[0], nil
}
