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
		if err := ensureDirectoryEntry(directoryEntryEnsure{
			root: request.Location.Root, path: path, mode: request.Mode, position: position,
		}); err != nil {
			return err
		}
	}
	return nil
}

type directoryEntryEnsure struct {
	root     *os.Root
	path     core.RelativePath
	mode     fs.FileMode
	position directoryPosition
}

func ensureDirectoryEntry(request directoryEntryEnsure) error {
	if err := request.position.Validate(); err != nil {
		return err
	}
	err := request.root.Mkdir(request.path.String(), request.mode)
	if err == nil {
		if err := synchronizeDirectoryMode(request.root, request.path, request.mode); err != nil {
			return activationError(err)
		}
		if err := syncParent(request.root, request.path); err != nil {
			return activationError(err)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return activationError(err)
	}
	if request.position == directoryFinal {
		if err := synchronizeDirectoryMode(request.root, request.path, request.mode); err != nil {
			return activationError(err)
		}
		return nil
	}
	return validateExistingDirectory(request.root, request.path)
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
	directory, err := openDirectory(root, path.String())
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
	entry, err := openDirectory(root, path.String())
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
	file, err := openRegularReadFile(
		request.Location.Root,
		request.Location.Path.String(),
	)
	if err != nil {
		return core.ByteLength{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return core.ByteLength{}, closeReadFile(file, sourceError(err))
	}
	extent, err := core.CheckedUint64FromInt64(info.Size())
	if err != nil {
		return core.ByteLength{}, closeReadFile(file, sourceError(err))
	}
	count, copyErr := copyBounded(boundedCopyRequest{
		ctx: ctx, destination: request.Destination, source: file,
		maximum: request.MaximumBytes, kind: streamDestinationCaller,
		knownExtent: extent, extentKnown: true,
	})
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return count, errors.Join(copyErr, closeErr)
}

func openRegularReadFile(root *os.Root, path string) (*os.File, error) {
	file, err := openReadFile(root, path)
	if err != nil {
		return nil, sourceError(err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, closeReadFile(file, sourceError(err))
	}
	if !info.Mode().IsRegular() {
		return nil, closeReadFile(file, sourceError(fs.ErrInvalid))
	}
	if err := prepareRegularReadFile(file); err != nil {
		return nil, closeReadFile(file, sourceError(err))
	}
	return file, nil
}

func closeReadFile(file *os.File, primary error) error {
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return errors.Join(primary, closeErr)
}
