//go:build darwin || linux

package filestore

import (
	"io/fs"
	"os"
	"syscall"
)

func createNamedPipe(request NamedPipeRequest) error {
	if err := syscall.Mkfifo(request.Path.String(), uint32(request.Mode)); err != nil {
		return classifyCreateError(os.NewSyscallError("mkfifo", err))
	}
	return nil
}

func openNamedPipeReadWrite(location Location) (*os.File, error) {
	file, err := location.Root.OpenFile(location.Path.String(), os.O_RDWR, 0)
	if err != nil {
		return nil, sourceError(err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, closeReadFile(file, sourceError(err))
	}
	if info.Mode()&fs.ModeNamedPipe == 0 {
		return nil, closeReadFile(file, sourceError(fs.ErrInvalid))
	}
	return file, nil
}
