//go:build windows

package hostfacts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/windows"
)

type platformRoot struct {
	root      *os.Root
	file      *os.File
	volumeID  uint32
	finalPath string
}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func treeOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func openRoot(path string) (*platformRoot, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if windowsReparsePoint(before) || !before.IsDir() {
		return nil, core.ErrHostFactsContract
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	information, err := validateWindowsDirectory(file, before, 0)
	if err != nil {
		return nil, errors.Join(err, file.Close(), root.Close())
	}
	finalPath, err := windowsFinalPath(file)
	if err != nil {
		return nil, errors.Join(err, file.Close(), root.Close())
	}
	return &platformRoot{
		root: root, file: file, volumeID: information.VolumeSerialNumber, finalPath: finalPath,
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

func validateWindowsDirectory(
	file *os.File,
	before fs.FileInfo,
	volumeID uint32,
) (windows.ByHandleFileInformation, error) {
	after, err := file.Stat()
	if err != nil || !after.IsDir() || !os.SameFile(before, after) {
		return windows.ByHandleFileInformation{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	information, err := windowsFileInformation(file)
	if err != nil {
		return windows.ByHandleFileInformation{}, err
	}
	if information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		(volumeID != 0 && information.VolumeSerialNumber != volumeID) {
		return windows.ByHandleFileInformation{}, core.ErrHostFactsObservation
	}
	return information, nil
}

func (r *platformRoot) close() error {
	if r == nil || r.root == nil || r.file == nil {
		return core.ErrHostFactsContract
	}
	return errors.Join(r.file.Close(), r.root.Close())
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
	information, err := windowsFileInformation(r.file)
	if err != nil || information.VolumeSerialNumber != r.volumeID {
		return DiskCapacity{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return newDiskCapacity(available, total)
}

func (r *platformRoot) openDirectory(relative string) (*os.File, error) {
	before, err := r.root.Lstat(relative)
	if err != nil {
		return nil, err
	}
	if windowsReparsePoint(before) || !before.IsDir() {
		return nil, core.ErrHostFactsContract
	}
	file, err := r.root.Open(relative)
	if err != nil {
		return nil, err
	}
	if _, err := validateWindowsDirectory(file, before, r.volumeID); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func (r *platformRoot) inspectEntry(_ *os.File, relative, name string) (treeEntry, error) {
	child := filepath.Join(relative, name)
	before, err := r.root.Lstat(child)
	if err != nil {
		return treeEntry{}, err
	}
	if windowsReparsePoint(before) {
		return treeEntry{kind: treeEntryIgnored}, nil
	}
	if before.IsDir() {
		directory, openErr := r.openDirectory(child)
		return treeEntry{directory: directory, kind: treeEntryDirectory}, openErr
	}
	if !before.Mode().IsRegular() {
		return treeEntry{kind: treeEntryIgnored}, nil
	}
	return r.inspectRegular(child, before)
}

func (r *platformRoot) inspectRegular(child string, before fs.FileInfo) (treeEntry, error) {
	file, err := r.root.Open(child)
	if err != nil {
		return treeEntry{}, err
	}
	after, statErr := file.Stat()
	information, informationErr := windowsFileInformation(file)
	closeErr := file.Close()
	if statErr != nil || informationErr != nil || closeErr != nil ||
		!after.Mode().IsRegular() || !os.SameFile(before, after) ||
		information.VolumeSerialNumber != r.volumeID ||
		information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return treeEntry{}, errors.Join(core.ErrHostFactsObservation, statErr, informationErr, closeErr)
	}
	return treeEntry{kind: treeEntryRegular, size: after.Size()}, nil
}

func windowsFileInformation(file *os.File) (windows.ByHandleFileInformation, error) {
	var information windows.ByHandleFileInformation
	err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &information)
	return information, err
}

func windowsReparsePoint(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
