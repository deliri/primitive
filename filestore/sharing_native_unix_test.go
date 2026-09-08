//go:build darwin || linux

package filestore_test

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// This platform has no share-mode API. No substitute lock model is an oracle.
func nativeSharingProbe(string) (filestore.Sharing, error) {
	return filestore.SharingUnknown, core.ErrFilestoreContract
}
