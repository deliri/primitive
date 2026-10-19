package process

import (
	"errors"
	"math"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

// TailBuffer owns a fixed byte window over a forward stream. Its window is a
// retained-memory choice, never a source limit. Write and Snapshot are atomic
// with respect to one another; callers own every returned snapshot.
type TailBuffer struct {
	storage  []byte
	received uint64
	cursor   int
	mu       sync.Mutex
}

// TailBufferObservation conserves all consumed bytes, including the prefix
// outside Data. Data is a raw binary suffix, without text-boundary adjustment.
type TailBufferObservation struct {
	Data     []byte
	Received core.ByteLength
	Dropped  core.ByteLength
}

// Validate rejects observations that lose or invent source bytes.
func (o TailBufferObservation) Validate() error {
	if err := o.Received.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if err := o.Dropped.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if o.Dropped.Uint64() > o.Received.Uint64() ||
		uint64(len(o.Data)) != o.Received.Uint64()-o.Dropped.Uint64() {
		return core.ErrProcessContract
	}
	return nil
}

// NewTailBuffer allocates the caller's positive, natively representable window.
func NewTailBuffer(window core.ByteCount) (*TailBuffer, error) {
	maximum, err := window.Uint64()
	if err != nil {
		return nil, errors.Join(core.ErrProcessContract, err)
	}
	if maximum > uint64(math.MaxInt) {
		return nil, errors.Join(core.ErrProcessContract, core.ErrNumericOverflow)
	}
	return &TailBuffer{storage: make([]byte, int(maximum))}, nil
}

// Validate rejects an unset or corrupted capability.
func (b *TailBuffer) Validate() error {
	if b == nil {
		return core.ErrProcessContract
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.validateLocked()
}

func (b *TailBuffer) validateLocked() error {
	if len(b.storage) == 0 || b.cursor < 0 || b.cursor >= len(b.storage) ||
		b.received > math.MaxInt64 ||
		(b.received < uint64(len(b.storage)) && uint64(b.cursor) != b.received) {
		return core.ErrProcessContract
	}
	return nil
}

// Write consumes the full chunk and retains only its suffix. Numeric overflow
// refuses the entire chunk before any bytes or counters change.
func (b *TailBuffer) Write(data []byte) (int, error) {
	if b == nil {
		return 0, core.ErrProcessContract
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.validateLocked(); err != nil {
		return 0, err
	}
	if uint64(len(data)) > math.MaxInt64-b.received {
		return 0, errors.Join(core.ErrProcessContract, core.ErrNumericOverflow)
	}
	if len(data) >= len(b.storage) {
		copy(b.storage, data[len(data)-len(b.storage):])
		b.cursor = 0
	} else {
		first := copy(b.storage[b.cursor:], data)
		copy(b.storage, data[first:])
		b.cursor = (b.cursor + len(data)) % len(b.storage)
	}
	b.received += uint64(len(data))
	return len(data), nil
}

// Snapshot returns an independent, ordered copy of the current suffix and its
// exact byte accounting. A later write or reset cannot change that observation.
func (b *TailBuffer) Snapshot() (TailBufferObservation, error) {
	if b == nil {
		return TailBufferObservation{}, core.ErrProcessContract
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.validateLocked(); err != nil {
		return TailBufferObservation{}, err
	}
	retained := min(b.received, uint64(len(b.storage)))
	data := make([]byte, int(retained))
	if retained < uint64(len(b.storage)) {
		copy(data, b.storage)
	} else {
		first := copy(data, b.storage[b.cursor:])
		copy(data[first:], b.storage[:b.cursor])
	}
	received, err := core.NewByteLength(b.received)
	if err != nil {
		return TailBufferObservation{}, errors.Join(core.ErrProcessContract, err)
	}
	dropped, err := core.NewByteLength(b.received - retained)
	if err != nil {
		return TailBufferObservation{}, errors.Join(core.ErrProcessContract, err)
	}
	return TailBufferObservation{Data: data, Received: received, Dropped: dropped}, nil
}

// Reset clears old bytes and accounting while reusing the same window.
func (b *TailBuffer) Reset() error {
	if b == nil {
		return core.ErrProcessContract
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.validateLocked(); err != nil {
		return err
	}
	clear(b.storage)
	b.cursor = 0
	b.received = 0
	return nil
}
