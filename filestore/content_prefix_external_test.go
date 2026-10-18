package filestore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCopyPrefixConservesNativeBytesAndLeavesTailUnread(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 31, 4095, 4096, 4097, 32767, 32768, 32769, 65537, 7 << 20} {
		for _, window := range []int{0, 1, 31, 4096, 32768} {
			t.Run(fmt.Sprintf("bytes_%d/window_%d", size, window), func(t *testing.T) {
				t.Parallel()
				payload := make([]byte, size+3)
				for i := range payload {
					payload[i] = byte(i)
				}
				source := bytes.NewReader(payload)
				extent, err := core.NewByteLength(uint64(size))
				if err != nil {
					t.Fatal(err)
				}
				var destination bytes.Buffer
				got, err := filestore.CopyPrefix(t.Context(), filestore.CopyPrefixRequest{Source: source, Destination: &destination, Bytes: extent, Buffer: make([]byte, window)})
				if err != nil || got.Extent != extent || got.Digest != core.NewSHA256Digest(sha256.Sum256(payload[:size])) || !bytes.Equal(destination.Bytes(), payload[:size]) {
					t.Fatalf("prefix observation=%+v/%v, bytes=%d,want exact %d", got, err, destination.Len(), size)
				}
				next, err := source.ReadByte()
				if err != nil || next != payload[size] || source.Len() != 2 {
					t.Fatalf("prefix consumed tail: next=%d/%v,remaining=%d", next, err, source.Len())
				}
			})
		}
	}
}

type prefixNativeRefusalReader struct {
	data  []byte
	cause error
}

func (r *prefixNativeRefusalReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, r.cause
}

type prefixNativeRefusalWriter struct{ cause error }

func (w prefixNativeRefusalWriter) Write([]byte) (int, error) { return 0, w.cause }

func TestCopyPrefixWithdrawsIncompleteOrRefusedNativeProof(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		source      io.Reader
		destination io.Writer
		extent      uint64
		want        error
	}{
		{name: "short source", source: bytes.NewReader([]byte("abc")), destination: io.Discard, extent: 4, want: io.ErrUnexpectedEOF},
		{name: "native failure beside complete prefix", source: &prefixNativeRefusalReader{data: []byte("abc"), cause: io.ErrNoProgress}, destination: io.Discard, extent: 3, want: io.ErrNoProgress},
		{name: "native failure before prefix", source: &prefixNativeRefusalReader{cause: io.ErrNoProgress}, destination: io.Discard, extent: 3, want: io.ErrNoProgress},
		{name: "native destination failure", source: bytes.NewReader([]byte("abc")), destination: prefixNativeRefusalWriter{cause: io.ErrClosedPipe}, extent: 3, want: io.ErrClosedPipe},
		{name: "absent source", destination: io.Discard, extent: 3, want: core.ErrFilestoreContract},
		{name: "absent destination", source: bytes.NewReader([]byte("abc")), extent: 3, want: core.ErrFilestoreContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			extent, err := core.NewByteLength(row.extent)
			if err != nil {
				t.Fatal(err)
			}
			got, err := filestore.CopyPrefix(t.Context(), filestore.CopyPrefixRequest{Source: row.source, Destination: row.destination, Bytes: extent})
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, row.want) {
				t.Fatalf("refused prefix=%+v/%v,want no proof and %v", got, err, row.want)
			}
		})
	}
}

func TestCopyPrefixStoppedContextNeverConsumesOrPublishes(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	for _, row := range []struct {
		name string
		ctx  context.Context
		want error
	}{{name: "cancelled", ctx: ctx, want: io.ErrNoProgress}, {name: "absent", want: core.ErrNilContext}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := bytes.NewReader([]byte("source"))
			var destination bytes.Buffer
			extent, err := core.NewByteLength(3)
			if err != nil {
				t.Fatal(err)
			}
			got, err := filestore.CopyPrefix(row.ctx, filestore.CopyPrefixRequest{Source: source, Destination: &destination, Bytes: extent})
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, row.want) || source.Len() != 6 || destination.Len() != 0 {
				t.Fatalf("stopped prefix=%+v/%v,source=%d,destination=%d", got, err, source.Len(), destination.Len())
			}
		})
	}
}

func FuzzCopyPrefixNativeAgreement(f *testing.F) {
	f.Add([]byte("prefix and tail"), uint16(6))
	f.Add([]byte{}, uint16(0))
	f.Fuzz(func(t *testing.T, data []byte, selected uint16) {
		extent, err := core.NewByteLength(uint64(selected))
		if err != nil {
			t.Fatal(err)
		}
		source := bytes.NewReader(data)
		var destination bytes.Buffer
		got, err := filestore.CopyPrefix(t.Context(), filestore.CopyPrefixRequest{Source: source, Destination: &destination, Bytes: extent})
		if int(selected) > len(data) {
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("short prefix=%+v/%v,want withdrawn unexpected EOF", got, err)
			}
			return
		}
		prefix := data[:int(selected)]
		if err != nil || got.Extent != extent || got.Digest != core.NewSHA256Digest(sha256.Sum256(prefix)) || !bytes.Equal(destination.Bytes(), prefix) || source.Len() != len(data)-len(prefix) {
			t.Fatalf("accepted prefix=%+v/%v,remaining=%d,want exact bytes and untouched tail", got, err, source.Len())
		}
	})
}
