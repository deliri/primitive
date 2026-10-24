//go:build darwin || linux

package filestore

import (
	"errors"
	"os"
	"syscall"
)

func duplicateBlockingFile(file *os.File) (*os.File, error) {
	_ = file.Fd()
	connection, err := file.SyscallConn()
	if err != nil {
		return nil, sourceError(err)
	}
	duplicate := -1
	var duplicateErr error
	controlErr := connection.Control(func(descriptor uintptr) {
		duplicate, duplicateErr = syscall.Dup(int(descriptor))
	})
	if err := errors.Join(controlErr, os.NewSyscallError("dup", duplicateErr)); err != nil {
		return nil, sourceError(err)
	}
	return os.NewFile(uintptr(duplicate), file.Name()), nil
}
