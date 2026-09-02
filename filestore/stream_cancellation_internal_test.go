package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type cancelAfterReadSource struct {
	cancel context.CancelFunc
	cause  error
	data   []byte
	read   bool
}

func (s *cancelAfterReadSource) Read(destination []byte) (int, error) {
	if s.read {
		return 0, io.EOF
	}
	s.read = true
	count := copy(destination, s.data)
	s.cancel()
	return count, s.cause
}

func TestCopyBoundedPreservesConsumedPrefixAndNativeErrorAcrossCancellation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		readErr      error
		wantErr      error
		wantExcluded error
		name         string
	}{
		{name: "cancellation after successful read preserves consumed prefix", wantErr: context.Canceled, wantExcluded: core.ErrFilestoreSource},
		{name: "native read failure wins while its consumed prefix remains observable", readErr: io.ErrUnexpectedEOF, wantErr: io.ErrUnexpectedEOF, wantExcluded: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			var destination bytes.Buffer
			payload := []byte("consumed-prefix")
			maximum, maximumErr := core.NewByteCount(uint64(len(payload) + 1))
			if maximumErr != nil {
				t.Fatalf("core.NewByteCount() error = %v, want nil", maximumErr)
			}
			gotLength, gotErr := copyBounded(boundedCopyRequest{
				ctx: ctx, source: &cancelAfterReadSource{cancel: cancel, cause: tc.readErr, data: payload},
				destination: &destination, maximum: maximum, kind: streamDestinationCaller,
			})
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, tc.wantExcluded) {
				t.Fatalf("copyBounded() error = %v, want %v and not %v", gotErr, tc.wantErr, tc.wantExcluded)
			}
			if gotLength.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("copyBounded() = (length %d, bytes %q), want (%d, %q)", gotLength.Uint64(), destination.Bytes(), len(payload), payload)
			}
		})
	}
}
