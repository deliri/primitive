package objectstore

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// exactDownloadWriter protects the destination's agreed object extent.
// This is identity/integrity validation, not a configurable transfer quota.
type exactDownloadWriter struct {
	destination io.Writer
	remaining   uint64
}

func (w *exactDownloadWriter) Write(payload []byte) (int, error) {
	excess := uint64(len(payload)) > w.remaining
	if excess {
		payload = payload[:w.remaining]
	}
	if excess && len(payload) == 0 {
		return 0, core.ErrObjectStoreIntegrity
	}
	n, err := w.destination.Write(payload)
	if n < 0 || n > len(payload) {
		return 0, errors.Join(core.ErrObjectStoreDestination, err)
	}
	w.remaining -= uint64(n)
	if excess {
		err = errors.Join(err, core.ErrObjectStoreIntegrity)
	}
	return n, err
}
