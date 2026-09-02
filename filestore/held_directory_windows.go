//go:build windows

package filestore

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func openHeldDirectory(path string) (*os.File, FilesystemIdentity, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, FilesystemIdentity{}, err
	}
	if windowsReparsePoint(before) || !before.IsDir() {
		return nil, FilesystemIdentity{}, fs.ErrInvalid
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, FilesystemIdentity{}, err
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, FilesystemIdentity{}, errors.Join(err, root.Close())
	}
	after, statErr := file.Stat()
	information, informationErr := windowsDirectoryInformation(file)
	rootCloseErr := root.Close()
	validationErr := errors.Join(
		validateWindowsHeldDirectory(
			after,
			information,
			errors.Join(statErr, informationErr, rootCloseErr),
		),
		validateWindowsSameDirectory(before, after),
	)
	if validationErr != nil {
		return nil, FilesystemIdentity{}, errors.Join(validationErr, file.Close())
	}
	return file, newFilesystemIdentity(uint64(information.VolumeSerialNumber)), nil
}

func validateWindowsHeldDirectory(
	after fs.FileInfo,
	information windows.ByHandleFileInformation,
	observationErr error,
) error {
	if observationErr != nil {
		return errors.Join(fs.ErrInvalid, observationErr)
	}
	if after == nil || !after.IsDir() {
		return fs.ErrInvalid
	}
	if information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fs.ErrInvalid
	}
	return nil
}

func validateWindowsSameDirectory(before, after fs.FileInfo) error {
	if before == nil || after == nil || !os.SameFile(before, after) {
		return fs.ErrInvalid
	}
	return nil
}

func windowsDirectoryInformation(file *os.File) (windows.ByHandleFileInformation, error) {
	var information windows.ByHandleFileInformation
	err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &information)
	return information, err
}

func windowsReparsePoint(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
