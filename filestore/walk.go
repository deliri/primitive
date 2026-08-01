package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	return walkDirectory(ctx, request, request.Location.Path)
}

func walkDirectory(
	ctx context.Context,
	request WalkRequest,
	directoryPath core.RelativePath,
) error {
	directory, err := request.Location.Root.Open(directoryPath.String())
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
	walkErr := readDirectoryEntries(ctx, request, directoryPath, directory)
	return closeWalkDirectory(directory, walkErr)
}

func readDirectoryEntries(
	ctx context.Context,
	request WalkRequest,
	directoryPath core.RelativePath,
	directory *os.File,
) error {
	if request.Order == WalkOrderLexical {
		return readLexicalDirectoryEntries(ctx, request, directoryPath, directory)
	}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		entries, err := directory.ReadDir(walkDirectoryBatchEntries)
		for _, entry := range entries {
			if visitErr := visitWalkEntry(ctx, request, directoryPath, entry); visitErr != nil {
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

func readLexicalDirectoryEntries(
	ctx context.Context,
	request WalkRequest,
	directoryPath core.RelativePath,
	directory *os.File,
) error {
	maximum := int(request.DirectoryEntryMaximum.value)
	entries, err := directory.ReadDir(maximum + 1)
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
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if err := visitWalkEntry(ctx, request, directoryPath, entry); err != nil {
			return err
		}
	}
	return nil
}

func visitWalkEntry(
	ctx context.Context,
	request WalkRequest,
	directoryPath core.RelativePath,
	entry fs.DirEntry,
) error {
	path, err := core.ParseRelativePath(filepath.Join(directoryPath.String(), entry.Name()))
	if err != nil {
		return contractError(err)
	}
	observation := WalkEntry{Path: path, Entry: entry}
	if err := observation.Validate(); err != nil {
		return err
	}
	directive, err := request.Visit(observation)
	if err != nil {
		return err
	}
	if err := directive.Validate(); err != nil {
		return err
	}
	if !entry.IsDir() || directive == WalkSkipDirectory {
		return nil
	}
	return walkDirectory(ctx, request, path)
}

func closeWalkDirectory(directory *os.File, primary error) error {
	closeErr := directory.Close()
	if closeErr != nil {
		closeErr = sourceError(closeErr)
	}
	return errors.Join(primary, closeErr)
}
