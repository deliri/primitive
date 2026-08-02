package process

import (
	"io"
	"math"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

// TruncatingWriter forwards the first Limit bytes to one caller-owned writer
// and counts every later byte without retaining it. It is safe for concurrent
// calls and its memory use is independent of source extent.
type TruncatingWriter struct {
	destination io.Writer
	limit       core.ByteCount
	retained    uint64
	dropped     uint64
	mu          sync.Mutex
}

// NewTruncatingWriter constructs one bounded streaming projection.
func NewTruncatingWriter(destination io.Writer, limit core.ByteCount) (*TruncatingWriter, error) {
	if destination == nil {
		return nil, contractError("truncating writer destination is nil")
	}
	if err := validateOutputLimit(limit); err != nil {
		return nil, err
	}
	return &TruncatingWriter{destination: destination, limit: limit}, nil
}

// Validate rejects an unset destination or limit.
func (w *TruncatingWriter) Validate() error {
	if w == nil || w.destination == nil {
		return contractError("truncating writer is unset")
	}
	if err := validateOutputLimit(w.limit); err != nil {
		return err
	}
	return nil
}

// Write implements io.Writer. Once the retained limit is full, later bytes
// are successfully consumed and counted as dropped so the producer can keep
// running without unbounded output custody.
func (w *TruncatingWriter) Write(buffer []byte) (int, error) {
	if err := w.Validate(); err != nil {
		return 0, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	maximum, err := w.limit.Uint64()
	if err != nil {
		return 0, err
	}
	available := maximum - w.retained
	if uint64(len(buffer)) <= available {
		return w.forward(buffer)
	}
	// This branch proves available < len(buffer), so the uint64 index is
	// representable by the platform int that already bounds the slice.
	retained, err := forwardFullWrite(w.destination, &w.retained, buffer[:available])
	if err != nil {
		return retained, err
	}
	dropped := uint64(len(buffer) - retained) // #nosec G115 -- retained is normalized to [0, len(buffer)].
	if dropped > math.MaxInt64-w.dropped {
		return retained, core.ErrNumericOverflow
	}
	w.dropped += dropped
	return len(buffer), nil
}

func (w *TruncatingWriter) forward(buffer []byte) (int, error) {
	return forwardFullWrite(w.destination, &w.retained, buffer)
}

// RetainedBytes returns the exact bytes accepted by the destination.
func (w *TruncatingWriter) RetainedBytes() (core.ByteLength, error) {
	if err := w.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return core.NewByteLength(w.retained)
}

// DroppedBytes returns the exact source bytes consumed after the limit.
func (w *TruncatingWriter) DroppedBytes() (core.ByteLength, error) {
	if err := w.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return core.NewByteLength(w.dropped)
}
