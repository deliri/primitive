package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

type cancelingRewindSource struct {
	*bytes.Reader
	cancel  context.CancelCauseFunc
	seeks   int
	failure error
}

func (s *cancelingRewindSource) Seek(offset int64, whence int) (int64, error) {
	s.seeks++
	position, err := s.Reader.Seek(offset, whence)
	if s.cancel != nil {
		s.cancel(io.ErrClosedPipe)
	}
	return position, errors.Join(err, s.failure)
}

func TestRewindRetainsNativeCustomCancellationCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		preCanceled bool
		failure     error
		seeks       int
		remaining   int
	}{
		{name: "canceled ingress retains cause without seek", preCanceled: true, remaining: 3},
		{name: "cancellation after native seek retains moved coordinate", seeks: 1, remaining: 4},
		{name: "native seek refusal and cancellation both survive", failure: io.ErrUnexpectedEOF, seeks: 1, remaining: 4},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			source := &cancelingRewindSource{Reader: bytes.NewReader([]byte("abcd")), cancel: cancel, failure: row.failure}
			if _, err := source.Reader.ReadByte(); err != nil {
				t.Fatal(err)
			}
			if row.preCanceled {
				cancel(io.ErrClosedPipe)
			}
			got := filestore.Rewind(ctx, filestore.RewindRequest{Source: source})
			if !errors.Is(got, context.Canceled) || !errors.Is(got, io.ErrClosedPipe) || row.failure != nil && !errors.Is(got, row.failure) || source.seeks != row.seeks || source.Len() != row.remaining {
				t.Fatalf("rewind%v seeks%d remaining%d; want cancellation+cause+%v seeks%d remaining%d", got, source.seeks, source.Len(), row.failure, row.seeks, row.remaining)
			}
		})
	}
}
