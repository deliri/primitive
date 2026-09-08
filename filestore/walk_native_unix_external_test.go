//go:build darwin || linux

package filestore_test

import (
	"os"
	"syscall"
)

// Independent Go acquisition supplies the platform-owned errno for a replaced
// regular entry. No Primitive classifier participates in the expected cause.
func nativeWalkDirectoryReopen(root *os.Root, path string) error {
	file, err := root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	return file.Close()
}
