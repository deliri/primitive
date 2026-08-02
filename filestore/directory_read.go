package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type directoryPosition uint8

const (
	directoryPositionUnknown directoryPosition = iota
	directoryIntermediate
	directoryFinal
	directoryPositionLimit
)

const mkdirOperation = "mkdir"

func directoryPositionDiagnostics() [directoryPositionLimit]string {
	return [directoryPositionLimit]string{
		directoryIntermediate: "intermediate",
		directoryFinal:        "final",
	}
}

func (p directoryPosition) Validate() error {
	if !p.IsValid() {
		return contractError(errors.New("filestore directory position is invalid"))
	}
	return nil
}

func (p directoryPosition) IsValid() bool {
	return p > directoryPositionUnknown && p < directoryPositionLimit &&
		directoryPositionDiagnostics()[p] != ""
}

func (p directoryPosition) String() string {
	if !p.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return directoryPositionDiagnostics()[p]
}

func (directoryPosition) OffWireEnum() {}

// EnsureDirectory creates one real directory chain and durably synchronizes
// each namespace addition.
func EnsureDirectory(ctx context.Context, request DirectoryRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	components := strings.Split(
		request.Location.Path.String(),
		string(filepath.Separator),
	)
	current := ""
	for index, component := range components {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		current = filepath.Join(current, component)
		path, err := core.ParseRelativePath(current)
		if err != nil {
			return contractError(err)
		}
		position := directoryIntermediate
		if index == len(components)-1 {
			position = directoryFinal
		}
		if err := ensureDirectoryEntry(
			request.Location.Root,
			path,
			request.Mode,
			position,
		); err != nil {
			return err
		}
	}
	return nil
}

func ensureDirectoryEntry(
	root *os.Root,
	path core.RelativePath,
	mode fs.FileMode,
	position directoryPosition,
) error {
	if err := position.Validate(); err != nil {
		return err
	}
	err := root.Mkdir(path.String(), mode)
	if err == nil {
		if err := synchronizeDirectoryMode(root, path, mode); err != nil {
			return activationError(err)
		}
		if err := syncParent(root, path); err != nil {
			return activationError(err)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return activationError(err)
	}
	if position == directoryFinal {
		if err := synchronizeDirectoryMode(root, path, mode); err != nil {
			return activationError(err)
		}
		return nil
	}
	return validateExistingDirectory(root, path)
}

var (
	_ core.Validatable = directoryPositionUnknown
	_ core.OffWireEnum = directoryPositionUnknown
)

func synchronizeDirectoryMode(
	root *os.Root,
	path core.RelativePath,
	mode fs.FileMode,
) error {
	directory, err := root.Open(path.String())
	if err != nil {
		return err
	}
	info, statErr := directory.Stat()
	if statErr == nil && !info.IsDir() {
		statErr = &os.PathError{
			Op:   mkdirOperation,
			Path: path.String(),
			Err:  fs.ErrExist,
		}
	}
	chmodErr := error(nil)
	syncErr := error(nil)
	if statErr == nil {
		chmodErr = directory.Chmod(mode)
	}
	if statErr == nil && chmodErr == nil {
		syncErr = directory.Sync()
	}
	closeErr := directory.Close()
	return errors.Join(statErr, chmodErr, syncErr, closeErr)
}

func validateExistingDirectory(root *os.Root, path core.RelativePath) error {
	entry, err := root.Open(path.String())
	if err != nil {
		return activationError(err)
	}
	info, statErr := entry.Stat()
	closeErr := entry.Close()
	if statErr != nil || closeErr != nil {
		return activationError(errors.Join(statErr, closeErr))
	}
	if !info.IsDir() {
		return activationError(&os.PathError{
			Op:   mkdirOperation,
			Path: path.String(),
			Err:  fs.ErrExist,
		})
	}
	return nil
}

// Read streams one regular file through the caller's standard io.Writer.
func Read(ctx context.Context, request ReadRequest) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	if err := request.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	file, err := request.Location.Root.Open(request.Location.Path.String())
	if err != nil {
		return core.ByteLength{}, sourceError(err)
	}
	info, err := file.Stat()
	if err != nil {
		return core.ByteLength{}, closeReadFile(file, sourceError(err))
	}
	if !info.Mode().IsRegular() {
		return core.ByteLength{}, closeReadFile(file, sourceError(fs.ErrInvalid))
	}
	count, copyErr := copyBounded(
		ctx,
		request.Destination,
		file,
		request.MaximumBytes,
		streamDestinationCaller,
	)
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return count, errors.Join(copyErr, closeErr)
}

func closeReadFile(file *os.File, primary error) error {
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return errors.Join(primary, closeErr)
}
