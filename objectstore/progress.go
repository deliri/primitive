package objectstore

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// TransferProgress is one exact monotonic observation of an object stream.
// It contains no product name, local path, or object content.
type TransferProgress struct {
	completed core.ByteLength
	total     core.ByteLength
	direction Direction
}

func newTransferProgress(
	direction Direction,
	completed core.ByteLength,
	total core.ByteLength,
) (TransferProgress, error) {
	progress := TransferProgress{direction: direction, completed: completed, total: total}
	if err := progress.Validate(); err != nil {
		return TransferProgress{}, err
	}
	return progress, nil
}

// Validate proves direction and bounded monotonic extent.
func (p TransferProgress) Validate() error {
	if err := errors.Join(p.direction.Validate(), p.completed.Validate(), p.total.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if p.completed.Uint64() > p.total.Uint64() {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
	}
	return nil
}

// Direction returns the observed operation.
func (p TransferProgress) Direction() Direction { return p.direction }

// Completed returns the exact content bytes accepted so far.
func (p TransferProgress) Completed() core.ByteLength { return p.completed }

// Total returns the declared exact content extent.
func (p TransferProgress) Total() core.ByteLength { return p.total }

// ProgressObserver is an optional caller-owned typed observation boundary.
// Returning an error stops the transfer rather than silently losing display
// or audit output.
type ProgressObserver func(TransferProgress) error

// Validate rejects an unset observer.
func (o ProgressObserver) Validate() error {
	if o == nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

func (o ProgressObserver) observe(progress TransferProgress) error {
	if err := errors.Join(o.Validate(), progress.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return o(progress)
}

type progressWriter struct {
	observer  ProgressObserver
	completed uint64
	total     core.ByteLength
	direction Direction
}

func (w *progressWriter) Write(payload []byte) (int, error) {
	if w == nil {
		return 0, core.ErrObjectStoreContract
	}
	next := w.completed + uint64(len(payload))
	completed, err := core.NewByteLength(next)
	if err != nil {
		return 0, progressError(w.direction, err)
	}
	progress, err := newTransferProgress(w.direction, completed, w.total)
	if err != nil {
		return 0, progressError(w.direction, err)
	}
	if err := w.observer.observe(progress); err != nil {
		return 0, progressError(w.direction, err)
	}
	w.completed = next
	return len(payload), nil
}

func progressDestination(
	observer ProgressObserver,
	direction Direction,
	total core.ByteLength,
) io.Writer {
	if observer == nil {
		return io.Discard
	}
	return &progressWriter{observer: observer, direction: direction, total: total}
}

func progressError(direction Direction, cause error) error {
	switch direction {
	case DirectionUpload:
		return errors.Join(core.ErrObjectStoreSource, cause)
	case DirectionDownload:
		return errors.Join(core.ErrObjectStoreDestination, cause)
	case DirectionUnknown, directionLimit:
		return errors.Join(core.ErrObjectStoreContract, cause)
	}
	return errors.Join(core.ErrObjectStoreContract, cause)
}

var (
	_ core.Validatable = TransferProgress{}
	_ core.Validatable = ProgressObserver(nil)
	_ io.Writer        = (*progressWriter)(nil)
)
