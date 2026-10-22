package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ConfirmTreeDurable synchronizes an already populated, caller-owned private
// tree before publication. All writers must be closed and namespace changes
// excluded for the duration. It refuses links and special entries, synchronizes
// each regular file and directory, then synchronizes the root and its parent.
// No file inventory is retained; Walk's fixed batches depend only on depth.
// Success acknowledges OS synchronization, not immunity to hardware failure.
func ConfirmTreeDurable(ctx context.Context, request Location) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := confirmTreeEntry(ctx, request); err != nil {
		return err
	}
	err := Walk(ctx, WalkRequest{Location: request, Visit: func(entry WalkEntry) (WalkDirective, error) {
		location := Location{Root: request.Root, Path: entry.Path}
		return WalkContinue, confirmTreeEntry(ctx, location)
	}})
	if err != nil {
		return err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := syncDirectory(request.Root, request.Path); err != nil {
		return activationError(err)
	}
	if err := syncParent(request.Root, request.Path); err != nil {
		return activationError(err)
	}
	return nil
}

func confirmTreeEntry(ctx context.Context, location Location) (resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	info, err := location.Root.Lstat(location.Path.String())
	if err != nil {
		return sourceError(err)
	}
	var file *os.File
	switch {
	case info.IsDir():
		file, err = openDirectory(location.Root, location.Path.String())
	case info.Mode().IsRegular():
		file, err = openCustodyFile(location.Root, location.Path.String())
	default:
		return sourceError(fs.ErrInvalid)
	}
	if err != nil {
		return sourceError(err)
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return sourceError(err)
	}
	if !os.SameFile(info, opened) {
		return sourceError(fs.ErrInvalid)
	}
	if err := syncTreeMember(file); err != nil {
		return activationError(err)
	}
	return nil
}

var _ core.Validatable = Location{}
