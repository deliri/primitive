//go:build windows

package filestore_test

import (
	"errors"
	"io/fs"
)

// syscallMkfifo reports that this host cannot create a named pipe, so the one
// table case that needs a real FIFO skips with the substrate's own answer
// instead of failing the build on hosts whose syscall package has no Mkfifo.
func syscallMkfifo(string) error {
	return errors.ErrUnsupported
}

func nativeInspectionAbsence(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func nativeInspectionStorage(fs.FileInfo) (inspectionNativeStorage, error) {
	return inspectionNativeStorage{}, nil
}
