//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris

package filestore

import "os"

// openReadFile keeps the platform's native open where POSIX nonblocking
// descriptor acquisition is unavailable. The rooted open is the whole
// platform contract and the regular-file proof still runs against the acquired
// handle.
func openReadFile(root *os.Root, path string) (*os.File, error) {
	return root.Open(path)
}

// prepareRegularReadFile has nothing to restore where the open was never made
// nonblocking.
func prepareRegularReadFile(_ *os.File) error {
	return nil
}
