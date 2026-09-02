//go:build windows

package hostfacts

import (
	"context"
	"errors"
	"os"
	"unicode/utf16"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"golang.org/x/sys/windows"
)

type platformRoot struct {
	directory *filestore.HeldDirectory
	file      *os.File
	finalPath string
}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func openRoot(ctx context.Context, path core.AbsolutePath) (*platformRoot, error) {
	directory, err := filestore.OpenDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	file, err := directory.File()
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	finalPath, err := windowsFinalPath(file)
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	return &platformRoot{
		directory: directory, file: file, finalPath: finalPath,
	}, nil
}

func windowsFinalPath(file *os.File) (string, error) {
	var probe uint16
	required, err := windows.GetFinalPathNameByHandle(
		windows.Handle(file.Fd()), &probe, 1, 0,
	)
	if err != nil || required <= 1 {
		return "", errors.Join(core.ErrHostFactsObservation, err)
	}
	storage := make([]uint16, required)
	count, err := windows.GetFinalPathNameByHandle(
		windows.Handle(file.Fd()), &storage[0], uint32(len(storage)), 0,
	)
	if err != nil || count == 0 || count >= uint32(len(storage)) {
		return "", errors.Join(core.ErrHostFactsObservation, err)
	}
	return string(utf16.Decode(storage[:count])), nil
}

func (r *platformRoot) close() error {
	if r == nil || r.directory == nil || r.file == nil {
		return core.ErrHostFactsContract
	}
	directory := r.directory
	r.directory = nil
	r.file = nil
	return directory.Close()
}

func (r *platformRoot) diskCapacity() (DiskCapacity, error) {
	path, err := windows.UTF16PtrFromString(r.finalPath)
	if err != nil {
		return DiskCapacity{}, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, &total, &free); err != nil {
		return DiskCapacity{}, err
	}
	return newDiskCapacity(available, total)
}
