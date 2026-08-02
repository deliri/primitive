package process

import (
	"errors"
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
	if err := limit.Validate(); err != nil {
		return nil, errors.Join(core.ErrProcessContract, err)
	}
	return &TruncatingWriter{destination: destination, limit: limit}, nil
}

// Validate rejects an unset destination or limit.
func (w *TruncatingWriter) Validate() error {
	if w == nil || w.destination == nil {
		return contractError("truncating writer is unset")
	}
	if err := w.limit.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
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
	// available is strictly below len(buffer) on this branch, so it always
	// indexes the buffer. The bound is checked rather than reasoned so a later
	// change to the branch condition cannot turn it into a silent truncation.
	if available > uint64(math.MaxInt) {
		return 0, errors.Join(core.ErrNumericOverflow, contractError("retainable extent exceeds addressable range"))
	}
	retained, err := w.forward(buffer[:int(available)])
	if err != nil {
		return retained, err
	}
	if retained < 0 || retained > len(buffer) {
		return retained, io.ErrShortWrite
	}
	dropped, err := core.CheckedUint64FromInt64(int64(len(buffer) - retained))
	if err != nil {
		return retained, err
	}
	if dropped > math.MaxUint64-w.dropped {
		return retained, core.ErrNumericOverflow
	}
	w.dropped += dropped
	return len(buffer), nil
}

func (w *TruncatingWriter) forward(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	count, err := w.destination.Write(buffer)
	if count < 0 || count > len(buffer) {
		return 0, io.ErrShortWrite
	}
	w.retained += uint64(count)
	if err != nil {
		return count, err
	}
	if count != len(buffer) {
		return count, io.ErrShortWrite
	}
	return count, nil
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
