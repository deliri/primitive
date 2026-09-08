//go:build darwin || linux

package filestore

import "github.com/deliri/primitive/v2026/core"

// observeSharing refuses unsupported share-mode observations without I/O.
func observeSharing(core.AbsolutePath) (Sharing, error) {
	return SharingUnknown, core.ErrFilestoreContract
}
