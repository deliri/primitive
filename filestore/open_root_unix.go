//go:build darwin || linux

package filestore

import (
	"errors"
	"os"
	"syscall"
)

// Acquire through Go with directory-only, nonblocking flags, then duplicate
// that exact held descriptor into os.Root while SyscallConn owns its lifetime.
func openRootDirectory(path string) (*os.Root, error) {
	// witness:waiver doctrine/code_form/defer_after_acquire -- The explicit Close below joins its failure and closes the derived Root before returning; deferring Close would lose that ownership decision.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	root, openErr := rootFromDirectoryFile(file)
	closeErr := file.Close()
	if err := errors.Join(openErr, closeErr); err != nil {
		if root != nil {
			return nil, errors.Join(err, root.Close())
		}
		return nil, err
	}
	return root, nil
}

func rootFromDirectoryFile(file *os.File) (*os.Root, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	var root *os.Root
	var openErr error
	controlErr := connection.Control(func(descriptor uintptr) {
		root, openErr = os.OpenRoot(openRootDescriptorPath(int(descriptor)))
	})
	return root, errors.Join(controlErr, openErr)
}
