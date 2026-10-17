package filestore

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AppendDestination owns one append handle and a fixed Go bufio working window.
// Callers serialize use and must close the owner. No raw handle escapes; the
// root is borrowed only during acquisition or rotation. Accepted writes may
// remain buffered until Flush, Sync, Rotate or Close. Sync acknowledges the
// operating system's synchronization result, not a hardware power-loss promise.
type AppendDestination struct {
	self   *AppendDestination
	file   *os.File
	writer *bufio.Writer
}

// OpenAppendDestination acquires append custody using the existing rooted
// append contract. Its working window is independent of total stream extent.
func OpenAppendDestination(ctx context.Context, request AppendRequest) (*AppendDestination, error) {
	file, err := OpenAppend(ctx, request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.Join(err, context.Cause(ctx))
		}
		return nil, err
	}
	d := &AppendDestination{file: file, writer: bufio.NewWriter(file)}
	d.self = d
	return d, nil
}

// Validate refuses unset, copied and settled owners without touching the file.
func (d *AppendDestination) Validate() error {
	if d == nil || d.self != d || d.file == nil || d.writer == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// Inspect observes the owned native file without transferring its handle.
func (d *AppendDestination) Inspect(ctx context.Context) (Inspection, error) {
	if err := d.Validate(); err != nil {
		return Inspection{}, err
	}
	return InspectOpenFile(ctx, HandleInspectionRequest{File: d.file})
}

// ObserveStanding compares the owned inode with the path's current occupant.
// It does not infer custody from a remembered path or copied metadata.
func (d *AppendDestination) ObserveStanding(ctx context.Context, path core.AbsolutePath) (HeldStanding, error) {
	if err := d.Validate(); err != nil {
		return HeldStandingUnknown, err
	}
	return ObserveHeldStanding(ctx, d.file, path)
}

// Write delegates byte acceptance and sticky failure behavior to Go's bufio.
func (d *AppendDestination) Write(data []byte) (int, error) {
	if err := d.Validate(); err != nil {
		return 0, err
	}
	return d.writer.Write(data)
}

// Flush makes buffered bytes visible to the file without claiming durability.
func (d *AppendDestination) Flush() error {
	if err := d.Validate(); err != nil {
		return err
	}
	return d.writer.Flush()
}

// Buffered returns the bytes still in Go's working window. It does not infer
// an on-disk extent or a durability watermark.
func (d *AppendDestination) Buffered() (int, error) {
	if err := d.Validate(); err != nil {
		return 0, err
	}
	return d.writer.Buffered(), nil
}

// DiscardBuffered explicitly abandons unwritten bytes and Go's sticky writer
// error. It neither repairs the underlying file nor claims any bytes durable.
// The returned count records the exact discarded working window.
func (d *AppendDestination) DiscardBuffered() (int, error) {
	if err := d.Validate(); err != nil {
		return 0, err
	}
	n := d.writer.Buffered()
	d.writer.Reset(d.file)
	return n, nil
}

// Truncate flushes accepted writes before changing the native file extent.
// Subsequent writes still append at the new end. Sync or Close owns durability.
func (d *AppendDestination) Truncate(ctx context.Context, length core.ByteLength) error {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.Join(err, context.Cause(ctx))
		}
		return err
	}
	size, err := length.Int64()
	if err != nil {
		return err
	}
	if err := d.Flush(); err != nil {
		return err
	}
	return d.file.Truncate(size)
}

// Sync flushes before synchronizing. A failed flush does not claim durability.
func (d *AppendDestination) Sync() error {
	if err := d.Flush(); err != nil {
		return err
	}
	return d.file.Sync()
}

// Close attempts flush, synchronization and close even when an earlier step
// fails, preserving every native cause. Cleanup requires no live context. A
// repeated close on the original owner is harmless; copied owners are refused.
func (d *AppendDestination) Close() error {
	if d == nil || d.self != d {
		return core.ErrFilestoreContract
	}
	if d.file == nil {
		return nil
	}
	return d.settle()
}

func (d *AppendDestination) settle() error {
	flushErr := d.writer.Flush()
	syncErr := d.file.Sync()
	closeErr := d.file.Close()
	d.file, d.writer = nil, nil
	return errors.Join(flushErr, syncErr, closeErr)
}

// Rotate validates caller intent before touching the outgoing file. Once
// admitted it settles that file on every path, then exclusively creates the
// incoming file. Any execution failure leaves the owner settled and unable to
// accept zombie writes. A validation or cancellation refusal leaves it usable.
func (d *AppendDestination) Rotate(ctx context.Context, incoming AppendRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.Join(err, context.Cause(ctx))
		}
		return err
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := (RotationRequest{Outgoing: d.file, Incoming: incoming}).Validate(); err != nil {
		return err
	}
	if err := d.settle(); err != nil {
		return activationError(err)
	}
	file, err := createAppend(incoming)
	if err != nil {
		return err
	}
	d.file, d.writer = file, bufio.NewWriter(file)
	return nil
}

var (
	_ core.Validatable = (*AppendDestination)(nil)
	_ io.WriteCloser   = (*AppendDestination)(nil)
)
