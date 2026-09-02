//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openHeldDirectory(path string) (*os.File, FilesystemIdentity, error) {
	descriptor, err := unix.Open(
		path,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_DIRECTORY|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, FilesystemIdentity{}, err
	}
	var information unix.Stat_t
	if err := unix.Fstat(descriptor, &information); err != nil {
		return nil, FilesystemIdentity{}, errors.Join(err, unix.Close(descriptor))
	}
	if information.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, FilesystemIdentity{}, errors.Join(errors.New("filestore held path is not a directory"), unix.Close(descriptor))
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		return nil, FilesystemIdentity{}, errors.Join(errors.New("filestore could not own directory descriptor"), unix.Close(descriptor))
	}
	// The device coordinate is an opaque kernel bit pattern. Some supported
	// systems expose dev_t through a signed Go field, so a negative numeric
	// interpretation is not an invalid identity.
	return file, newFilesystemIdentity(uint64(information.Dev)), nil // #nosec G115 -- opaque dev_t bit pattern.
}
