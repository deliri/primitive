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
// each namespace addition. Existing ancestors retain their permissions; the
// final directory receives the requested mode. A native failure may leave the
// exact prefix already created. This operation does not roll that prefix back.
func EnsureDirectory(ctx context.Context, request DirectoryRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	directory, err := openDirectory(request.Location.Root, request.Location.Path.String())
	if err == nil {
		if err := synchronizeOpenedDirectoryMode(directory, request.Location.Path, request.Mode); err != nil {
			return activationError(err)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return activationError(err)
	}
	return ensureDirectoryChain(ctx, request)
}

// Only a missing acquisition enters creation. A later chmod/sync/close error
// must not retry an effect or mistake a partially settled directory for absence.
func ensureDirectoryChain(ctx context.Context, request DirectoryRequest) error {
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
	return synchronizeOpenedDirectoryMode(directory, path, mode)
}

// The same acquired Go handle owns observation, mode, synchronization and close.
func synchronizeOpenedDirectoryMode(directory *os.File, path core.RelativePath, mode fs.FileMode) error {
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
// The owned file is closed on return or Go unwinding. Caller panics propagate;
// a destination effect that panics has no returned byte acknowledgment.
func Read(ctx context.Context, request ReadRequest) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	if err := request.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	file, info, err := openRegularReadFile(
		request.Location.Root,
		request.Location.Path.String(),
	)
	if err != nil {
		return core.ByteLength{}, err
	}
	return readOwnedRegularFile(ctx, request, file, info)
}

func readOwnedRegularFile(ctx context.Context, request ReadRequest, file *os.File, info fs.FileInfo) (count core.ByteLength, err error) {
	defer func() { err = closeReadFile(file, err) }()
	extent, err := core.CheckedUint64FromInt64(info.Size())
	if err != nil {
		return core.ByteLength{}, sourceError(err)
	}
	return copyStream(streamCopyRequest{
		ctx: ctx, destination: request.Destination, source: file, buffer: request.Buffer,
		kind:        streamDestinationCaller,
		knownExtent: extent, extentKnown: true,
	})
}

func openRegularReadFile(root *os.Root, path string) (*os.File, fs.FileInfo, error) {
	file, err := openReadFile(root, path)
	if err != nil {
		return nil, nil, sourceError(err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, closeReadFile(file, sourceError(err))
	}
	if !info.Mode().IsRegular() {
		return nil, nil, closeReadFile(file, sourceError(fs.ErrInvalid))
	}
	if err := prepareRegularReadFile(file); err != nil {
		return nil, nil, closeReadFile(file, sourceError(err))
	}
	return file, info, nil
}

func closeReadFile(file *os.File, primary error) error {
	closeErr := file.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return errors.Join(primary, closeErr)
}
