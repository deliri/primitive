//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// openRootDirectory acquires a directory descriptor with O_DIRECTORY and
// O_NONBLOCK before adapting that exact descriptor into the standard-library
// rooted capability. A FIFO can therefore never park the acquisition, and a
// path replacement after the first open cannot change the object OpenRoot
// receives through the descriptor filesystem.
func openRootDirectory(path string) (*os.Root, error) {
	descriptor, err := unix.Open(
		path,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_DIRECTORY,
		0,
	)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(openRootDescriptorPath(descriptor))
	if err != nil || closeRootDescriptorAfterOpen() {
		_ = unix.Close(descriptor)
	}
	return root, err
}
