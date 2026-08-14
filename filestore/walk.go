package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const walkDirectoryBatchEntries = 64

// Walk streams descendants in operating-system directory order. It retains
// at most one fixed entry batch per open directory and never follows symbolic
// links. The starting directory itself is not delivered to Visit.
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
	directory, err := input.request.Location.Root.Open(input.directoryPath.String())
	if err != nil {
		return sourceError(err)
	}
	info, err := directory.Stat()
	if err != nil {
		return closeWalkDirectory(directory, sourceError(err))
	}
	if !info.IsDir() {
		return closeWalkDirectory(directory, sourceError(fs.ErrInvalid))
	}
	if input.expectedIdentity != nil && !os.SameFile(input.expectedIdentity, info) {
		return closeWalkDirectory(directory, sourceError(fs.ErrInvalid))
	}
	walkErr := readDirectoryEntries(readDirectoryInput{
		ctx:           input.ctx,
		request:       input.request,
		directoryPath: input.directoryPath,
		directory:     directory,
	})
	return closeWalkDirectory(directory, walkErr)
}

type readDirectoryInput struct {
	ctx           context.Context
	directory     *os.File
	directoryPath core.RelativePath
	request       WalkRequest
}

func readDirectoryEntries(input readDirectoryInput) error {
	if input.request.Order == WalkOrderLexical {
		return readLexicalDirectoryEntries(input)
	}
	for {
		if err := contextstate.Validate(input.ctx); err != nil {
			return err
		}
		entries, err := input.directory.ReadDir(walkDirectoryBatchEntries)
		for _, entry := range entries {
			if visitErr := visitWalkEntry(visitWalkEntryInput{
				ctx:           input.ctx,
				request:       input.request,
				directoryPath: input.directoryPath,
				entry:         entry,
			}); visitErr != nil {
				return visitErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return sourceError(err)
		}
	}
}

func readLexicalDirectoryEntries(input readDirectoryInput) error {
	maximum := int(input.request.DirectoryEntryMaximum.value)
	entries, err := input.directory.ReadDir(maximum + 1)
	if len(entries) > maximum {
		return contractError(errors.New("filestore directory exceeds entry maximum"))
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return sourceError(err)
	}
	slices.SortFunc(entries, func(left, right fs.DirEntry) int {
		return strings.Compare(left.Name(), right.Name())
	})
	for _, entry := range entries {
		if err := contextstate.Validate(input.ctx); err != nil {
			return err
		}
		if err := visitWalkEntry(visitWalkEntryInput{
			ctx:           input.ctx,
			request:       input.request,
			directoryPath: input.directoryPath,
			entry:         entry,
		}); err != nil {
			return err
		}
	}
	return nil
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
