package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const walkDirectoryBatchEntries = 64

// Walk streams descendants in native directory order with one fixed entry
// batch per open directory. Memory and held handles depend on traversal depth,
// not total entry count. Symbolic links are not descended into. The starting
// directory is not delivered to Visit.
func Walk(ctx context.Context, request WalkRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	return walkDirectory(walkDirectoryInput{
		ctx:           ctx,
		request:       request,
		directoryPath: request.Location.Path,
	})
}

type walkDirectoryInput struct {
	ctx              context.Context
	expectedIdentity fs.FileInfo
	directoryPath    core.RelativePath
	request          WalkRequest
}

func walkDirectory(input walkDirectoryInput) error {
	if err := contextstate.Validate(input.ctx); err != nil {
		return err
	}
	directory, err := openDirectory(input.request.Location.Root, input.directoryPath.String())
	if err != nil {
		return sourceError(err)
	}
	return walkOwnedDirectory(input, directory)
}

// walkOwnedDirectory owns the acquired Go handle for every return or unwind.
func walkOwnedDirectory(input walkDirectoryInput, directory *os.File) (resultErr error) {
	defer func() { resultErr = closeWalkDirectory(directory, resultErr) }()
	info, err := directory.Stat()
	if err != nil {
		return sourceError(err)
	}
	if !info.IsDir() {
		return sourceError(fs.ErrInvalid)
	}
	if input.expectedIdentity != nil && !os.SameFile(input.expectedIdentity, info) {
		return sourceError(fs.ErrInvalid)
	}
	return readDirectoryEntries(readDirectoryInput{
		ctx:           input.ctx,
		request:       input.request,
		directoryPath: input.directoryPath,
		directory:     directory,
	})
}

type readDirectoryInput struct {
	ctx           context.Context
	directory     *os.File
	directoryPath core.RelativePath
	request       WalkRequest
}

func readDirectoryEntries(input readDirectoryInput) error {
	emptyReads := 0
	for {
		if err := contextstate.Validate(input.ctx); err != nil {
			return err
		}
		done, err := readStreamingDirectoryBatch(input, &emptyReads)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func readStreamingDirectoryBatch(input readDirectoryInput, emptyReads *int) (bool, error) {
	entries, err := input.directory.ReadDir(walkDirectoryBatchEntries)
	if len(entries) == 0 && err == nil {
		*emptyReads++
		if *emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
			return false, sourceError(io.ErrNoProgress)
		}
		return false, nil
	}
	*emptyReads = 0
	for _, entry := range entries {
		if err := contextstate.Validate(input.ctx); err != nil {
			return false, err
		}
		if visitErr := visitWalkEntry(visitWalkEntryInput{
			ctx: input.ctx, request: input.request,
			directoryPath: input.directoryPath, entry: entry,
		}); visitErr != nil {
			return false, visitErr
		}
	}
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	if err != nil {
		return false, sourceError(err)
	}
	return false, nil
}

type visitWalkEntryInput struct {
	ctx           context.Context
	entry         fs.DirEntry
	directoryPath core.RelativePath
	request       WalkRequest
}

func visitWalkEntry(input visitWalkEntryInput) error {
	name, err := core.ParsePathComponent(input.entry.Name())
	if err != nil {
		return contractError(err)
	}
	path, err := input.directoryPath.Join(name)
	if err != nil {
		return contractError(err)
	}
	observation := WalkEntry{Path: path, Entry: input.entry}
	if err := observation.Validate(); err != nil {
		return err
	}
	identity, err := walkDirectoryIdentity(input.entry)
	if err != nil {
		return err
	}
	directive, err := input.request.Visit(observation)
	if err != nil {
		return err
	}
	if err := directive.Validate(); err != nil {
		return err
	}
	if identity == nil || directive == WalkSkipDirectory {
		return nil
	}
	return walkDirectory(walkDirectoryInput{
		ctx:              input.ctx,
		request:          input.request,
		directoryPath:    path,
		expectedIdentity: identity,
	})
}

func walkDirectoryIdentity(entry fs.DirEntry) (fs.FileInfo, error) {
	if !entry.IsDir() {
		return nil, nil
	}
	info, err := entry.Info()
	if err != nil {
		return nil, sourceError(err)
	}
	if !info.IsDir() {
		return nil, sourceError(fs.ErrInvalid)
	}
	return info, nil
}

func closeWalkDirectory(directory *os.File, primary error) error {
	closeErr := directory.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return errors.Join(primary, closeErr)
}
