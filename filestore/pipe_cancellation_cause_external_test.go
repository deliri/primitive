package filestore_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPipeTransferAdmissionPreservesNativeCancellationCause(t *testing.T) {
	t.Parallel()
	for _, deadline := range [...]bool{false, true} {
		name := "cancelled"
		if deadline {
			name = "expired deadline"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if deadline {
				ctx, cancel = newExpiredDeadline(t.Context(), t, io.ErrNoProgress)
				want = context.DeadlineExceeded
			} else {
				var cancelCause context.CancelCauseFunc
				ctx, cancelCause = context.WithCancelCause(t.Context())
				cancel = func() { cancelCause(io.ErrNoProgress) }
				cancel()
			}
			defer cancel()
			got, err := filestore.TransferPipe(ctx, filestore.PipeTransferRequest{Produce: func(context.Context, io.Writer) error { t.Error("refused source acquired a producer"); return nil }, Consume: func(context.Context, io.Reader) error { t.Error("refused source acquired a consumer"); return nil }})
			if !errors.Is(err, want) || !errors.Is(err, io.ErrNoProgress) || got.Validate() == nil || got.ProducerError != nil || got.ConsumerError != nil || got.CleanupError != nil {
				t.Fatalf("admission=%+v %v want zero lifetime/%v/native cause", got, err, want)
			}
		})
	}
}

func TestPipeTransferJoinedCallbacksPreserveParentCauseAndCloseNativeFiles(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	var reader, writer *os.File
	result, err := filestore.TransferPipe(ctx, filestore.PipeTransferRequest{
		Produce: func(ctx context.Context, destination io.Writer) error {
			writer = destination.(*os.File)
			if _, err := destination.Write([]byte{0x56}); err != nil {
				return err
			}
			<-ctx.Done()
			return ctx.Err()
		},
		Consume: func(ctx context.Context, source io.Reader) error {
			reader = source.(*os.File)
			var value [1]byte
			if _, err := io.ReadFull(source, value[:]); err != nil {
				return err
			}
			if value[0] != 0x56 {
				return io.ErrUnexpectedEOF
			}
			cancel(io.ErrNoProgress)
			return ctx.Err()
		},
	})
	if err != nil || result.Validate() != nil {
		t.Fatalf("joined lifetime=%+v %v", result, err)
	}
	for _, failure := range [...]error{result.ProducerError, result.ConsumerError} {
		if !errors.Is(failure, context.Canceled) || !errors.Is(failure, io.ErrNoProgress) {
			t.Fatalf("callback lost native cause: %v", failure)
		}
	}
	for _, endpoint := range [...]*os.File{reader, writer} {
		if endpoint == nil {
			t.Fatal("native transfer never acquired endpoint")
		}
		if _, err := endpoint.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("endpoint after join=%v", err)
		}
	}
}

func TestPipeTransferCauseObservationPanicPreservesTerminalIdentity(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := filestore.TransferPipe(scopeCausePanicContext{ctx}, filestore.PipeTransferRequest{Produce: func(context.Context, io.Writer) error { t.Error("cancelled producer entered"); return nil }, Consume: func(context.Context, io.Reader) error { t.Error("cancelled consumer entered"); return nil }})
	if got.Validate() == nil || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrContextObservation) {
		t.Fatalf("panic observation=%+v %v", got, err)
	}
}
