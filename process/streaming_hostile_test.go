package process

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type zeroWriteRejectingDestination struct {
	emptyWriteError error
	writes          int
}

type invalidCountAndErrorDestination struct {
	cause error
}

type emptyForeverReader struct{}

func (emptyForeverReader) Read([]byte) (int, error) { return 0, nil }

func (w invalidCountAndErrorDestination) Write(buffer []byte) (int, error) {
	return len(buffer) + 1, w.cause
}

func (w *zeroWriteRejectingDestination) Write(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, w.emptyWriteError
	}
	w.writes++
	return len(buffer), nil
}

// TestBoundedWriterDoesNotForwardAnEmptyPrefix proves an exactly full output
// bound cannot be converted into a destination failure by a later write. The
// destination owns empty-write behavior; Process must not call it when no byte
// remains admissible.
func TestBoundedWriterDoesNotForwardAnEmptyPrefix(t *testing.T) {
	t.Parallel()

	emptyWriteError := errors.New("destination rejects an empty write")
	destination := &zeroWriteRejectingDestination{emptyWriteError: emptyWriteError}
	ctx, cancel := context.WithCancelCause(context.Background())
	limit, err := core.NewByteCount(3)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	failures := &streamFailures{cancel: cancel}
	writer := &boundedWriter{
		destination: destination,
		failures:    failures,
		writeMu:     &sync.Mutex{},
		limit:       limit,
		stream:      StreamStdout,
	}

	if count, writeErr := writer.Write([]byte("abc")); count != 3 || writeErr != nil {
		t.Fatalf("first bounded write = (%d, %v), want (3, nil)", count, writeErr)
	}
	count, writeErr := writer.Write([]byte("d"))
	if count != 0 || !errors.Is(writeErr, core.ErrProcessOutputLimit) {
		t.Fatalf("overflowing bounded write = (%d, %v), want zero and %v", count, writeErr, core.ErrProcessOutputLimit)
	}
	if errors.Is(writeErr, emptyWriteError) || errors.Is(context.Cause(ctx), emptyWriteError) {
		t.Fatalf("overflowing bounded write reached the empty destination path: write %v, cause %v", writeErr, context.Cause(ctx))
	}
	if destination.writes != 1 {
		t.Fatalf("destination nonempty writes = %d, want 1", destination.writes)
	}
}

// TestForwardFullWritePreservesInvalidCountAndNativeFailure proves a hostile
// destination cannot make Process discard its native cause by returning an
// impossible count at the same time. Both facts are necessary to diagnose the
// caller boundary.
func TestForwardFullWritePreservesInvalidCountAndNativeFailure(t *testing.T) {
	t.Parallel()

	native := errors.New("destination failed while reporting an invalid count")
	retained := uint64(0)
	count, err := forwardFullWrite(
		invalidCountAndErrorDestination{cause: native},
		&retained,
		[]byte("payload"),
	)
	if count != 0 || retained != 0 ||
		!errors.Is(err, io.ErrShortWrite) || !errors.Is(err, native) {
		t.Fatalf("forwardFullWrite() = (%d, retained %d, %v), want zero, zero, %v, and native cause", count, retained, err, io.ErrShortWrite)
	}
}

func TestObservedReaderRefusesUnendingEmptyStdin(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	failures := &streamFailures{cancel: cancel}
	reader := &observedReader{
		source: emptyForeverReader{}, failures: failures,
	}
	var gotErr error
	for range core.ReaderConsecutiveEmptyReadMaximum {
		_, gotErr = reader.Read(make([]byte, 1))
	}
	if !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("observedReader.Read(empty stdin) error = %v, want %v",
			gotErr, io.ErrNoProgress)
	}
	if !errors.Is(context.Cause(ctx), io.ErrNoProgress) {
		t.Fatalf("process cancellation cause = %v, want %v",
			context.Cause(ctx), io.ErrNoProgress)
	}
}
