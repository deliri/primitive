//go:build windows

package filestore_test

import (
	"errors"
	"io/fs"
	"os"
)

func nativeWalkDirectoryReopen(root *os.Root, path string) error {
	file, err := root.Open(path)
	if err != nil {
		return err
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil {
		return errors.Join(statErr, closeErr)
	}
	if !info.IsDir() {
		return fs.ErrInvalid
	}
	return nil
}
