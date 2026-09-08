//go:build darwin || linux

package filestore

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

func openHeldDirectory(path string) (*os.File, FilesystemIdentity, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, FilesystemIdentity{}, err
	}
	information, err := file.Stat()
	if err != nil {
		return nil, FilesystemIdentity{}, errors.Join(err, file.Close())
	}
	status, ok := information.Sys().(*syscall.Stat_t)
	if !ok || !information.IsDir() {
		return nil, FilesystemIdentity{}, errors.Join(fs.ErrInvalid, file.Close())
	}
	// dev_t is an opaque kernel bit pattern, including on hosts that expose
	// it as a signed field. Go owns both the handle and this stat observation.
	return file, newFilesystemIdentity(uint64(status.Dev)), nil // #nosec G115 -- opaque dev_t bit pattern.
}
