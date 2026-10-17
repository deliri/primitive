package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type synchronousSourceLoan struct {
	*bytes.Reader
	closed bool
}

func (s *synchronousSourceLoan) Close() error { s.closed = true; return nil }

type synchronousDestinationLoan struct {
	bytes.Buffer
	closed  bool
	maximum int
}

func (d *synchronousDestinationLoan) Write(data []byte) (int, error) {
	d.maximum = max(d.maximum, len(data))
	return d.Buffer.Write(data)
}
func (d *synchronousDestinationLoan) Close() error { d.closed = true; return nil }

func TestSynchronousTransferOwnsNoBorrowedLifetimeAndConservesBytes(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 7, 8, 9, 8<<20 + 3} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{'x'}, size)
			source := &synchronousSourceLoan{Reader: bytes.NewReader(data)}
			destination := &synchronousDestinationLoan{}
			want := contentCopyFixture(t, data)
			var buffer [8]byte
			got, err := filestore.TransferContent(filestore.CopyContentRequest{Source: source, Destination: destination, Expected: &want, Buffer: buffer[:]})
			if err != nil || got != want || !bytes.Equal(destination.Bytes(), data) || destination.maximum > len(buffer) {
				t.Fatalf("synchronous transfer=%v/%v bytes=%d maximum=%d, want exact %d bytes in window %d", got, err, destination.Len(), destination.maximum, size, len(buffer))
			}
			if source.closed || destination.closed {
				t.Fatal("transfer closed a borrowed Go stream")
			}
		})
	}
}

func TestSynchronousTransferNativeRefusalsNeverReturnAContentProof(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name        string
		source      func() io.Reader
		destination func() io.Writer
		want        error
	}{
		{name: "nil source", source: func() io.Reader { return nil }, want: core.ErrFilestoreContract},
		{name: "typed nil source", source: func() io.Reader { return (*bytes.Reader)(nil) }, want: core.ErrFilestoreContract},
		{name: "typed nil destination", destination: func() io.Writer { return (*bytes.Buffer)(nil) }, want: core.ErrFilestoreContract},
		{name: "source native failure", source: func() io.Reader { return iotest.ErrReader(io.ErrClosedPipe) }, want: io.ErrClosedPipe},
		{name: "joined EOF does not erase a native source failure", source: func() io.Reader {
			return io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe)))
		}, want: io.ErrClosedPipe},
		{name: "native partial destination failure", destination: func() io.Writer { return copyContentFaultWriter{count: 1, cause: io.ErrClosedPipe} }, want: io.ErrClosedPipe},
		{name: "silent short write", destination: func() io.Writer { return copyContentFaultWriter{count: 1} }, want: io.ErrShortWrite},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			var source io.Reader = strings.NewReader("abc")
			var destination io.Writer = &bytes.Buffer{}
			if fixture.source != nil {
				source = fixture.source()
			}
			if fixture.destination != nil {
				destination = fixture.destination()
			}
			got, err := filestore.TransferContent(filestore.CopyContentRequest{Source: source, Destination: destination})
			if !errors.Is(err, fixture.want) || got != (filestore.ContentIndexEntry{}) {
				t.Fatalf("synchronous refusal=%v/%v, want zero proof and %v", got, err, fixture.want)
			}
		})
	}
}
