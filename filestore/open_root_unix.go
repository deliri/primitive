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
	// #nosec G304 -- This is the caller-authorized root acquisition boundary; child operations use the resulting os.Root.
	// rootFromHeldDirectory closes this handle explicitly, joining its failure and closing a derived Root before returning; deferring Close would lose that ownership decision.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	return rootFromHeldDirectory(file)
}

// openChildRootDirectory acquires a directory beneath parent through the same
// confined, directory-only, nonblocking open as every rooted directory read,
// then duplicates that held descriptor into os.Root.
func openChildRootDirectory(parent *os.Root, path string) (*os.Root, error) {
	file, err := openDirectory(parent, path)
	if err != nil {
		return nil, err
	}
	return rootFromHeldDirectory(file)
}

// rootFromHeldDirectory converts an acquired directory handle and closes it,
// joining every failure and closing a derived root before returning one.
func rootFromHeldDirectory(file *os.File) (*os.Root, error) {
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
