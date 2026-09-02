//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris && !windows

package filestore

import (
	"io/fs"
	"os"
)

func openHeldDirectory(string) (*os.File, FilesystemIdentity, error) {
	return nil, FilesystemIdentity{}, fs.ErrInvalid
}
