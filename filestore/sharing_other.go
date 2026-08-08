//go:build !windows

package filestore

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// observeSharing refuses: POSIX opens do not contend, so the question has no
// kernel answer here and the supported spelling is composing lsof through
// Process.
func observeSharing(core.AbsolutePath) (Sharing, error) {
	return SharingUnknown, errors.Join(
		core.ErrFilestoreContract,
		errors.New("this host's opens do not contend; sharing has no kernel answer here"),
	)
}
