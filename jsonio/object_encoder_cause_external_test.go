package jsonio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type causeObjectDestination struct {
	bytes.Buffer
	cancel   context.CancelCauseFunc
	cause    error
	writeErr error
}

func (w *causeObjectDestination) Write(data []byte) (int, error) {
	count, err := w.Buffer.Write(data)
	if w.cancel != nil {
		w.cancel(w.cause)
	}
	return count, errors.Join(err, w.writeErr)
}

func TestJSONObjectEncoderCancellationKeepsNativeCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                     string
		before, during, deadline bool
		cause, writeErr          error
	}{
		{name: "neutral active encoder"},
		{name: "before append", before: true, cause: io.ErrClosedPipe},
		{name: "during acknowledged append", during: true, cause: io.ErrClosedPipe},
		{name: "during refused append", during: true, cause: io.ErrClosedPipe, writeErr: io.ErrUnexpectedEOF},
		{name: "custom deadline cause", deadline: true, cause: io.ErrClosedPipe},
		{name: "plain cancellation remains native", before: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if tc.deadline {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadlineCause(ctx, time.Unix(1, 0), tc.cause)
				defer stop()
			}
			if tc.before {
				cancel(tc.cause)
			}
			destination := &causeObjectDestination{cause: tc.cause, writeErr: tc.writeErr}
			if tc.during {
				destination.cancel = cancel
			}
			encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: destination})
			if err != nil {
				t.Fatal(err)
			}
			object := scalarObject{Name: "meaning", Count: 1}
			err = encoder.Encode(ctx, object)
			if !tc.before && !tc.during && !tc.deadline {
				if err != nil {
					t.Fatal(err)
				}
				seen := 0
				for got, err := range jsonio.Objects[scalarObject](t.Context(), jsonio.ObjectSourceRequest{Source: &destination.Buffer}) {
					seen++
					if err != nil || got != object {
						t.Fatalf("neutral encode = %+v, %v", got, err)
					}
				}
				if seen != 1 {
					t.Fatalf("neutral objects = %d", seen)
				}
				return
			}
			terminal := context.Canceled
			if tc.deadline {
				terminal = context.DeadlineExceeded
			}
			if !errors.Is(err, terminal) || tc.cause != nil && !errors.Is(err, tc.cause) || tc.writeErr != nil && !errors.Is(err, tc.writeErr) || errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("encode lost native cause: %v", err)
			}
			if (tc.before || tc.deadline) && destination.Len() != 0 {
				t.Fatalf("refused admission wrote %d bytes", destination.Len())
			}
			if tc.during && destination.Len() == 0 {
				t.Fatal("fixture never reached destination")
			}
		})
	}
}
