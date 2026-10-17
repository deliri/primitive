package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPrefixReadObservesWindowWithoutConsumingTail(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 7, 8, 9, 32} {
		data := bytes.Repeat([]byte{'x'}, size)
		for _, window := range []int{0, 1, 8} {
			source := bytes.NewReader(data)
			destination := bytes.Repeat([]byte{0xcc}, window)
			got, err := filestore.ReadPrefix(t.Context(), filestore.PrefixReadRequest{Source: source, Destination: destination})
			want := min(size, window)
			if err != nil || got.Validate() != nil || got.Bytes.Uint64() != uint64(want) || !bytes.Equal(destination[:want], data[:want]) || source.Len() != size-want {
				t.Fatalf("source=%d window=%d observed=%d remaining=%d error=%v", size, window, got.Bytes.Uint64(), source.Len(), err)
			}
			if !bytes.Equal(destination[want:], bytes.Repeat([]byte{0xcc}, window-want)) {
				t.Fatal("short source overwrote bytes outside its observation")
			}
		}
	}
}

type prefixSourceRefusal struct{ complete bool }

func (r prefixSourceRefusal) Read(p []byte) (int, error) {
	if r.complete {
		for i := range p {
			p[i] = 'x'
		}
		return len(p), io.ErrClosedPipe
	}
	return 0, io.ErrClosedPipe
}

func TestPrefixReadPreservesFailureEvenWithCompleteWindow(t *testing.T) {
	t.Parallel()
	for _, complete := range []bool{false, true} {
		destination := bytes.Repeat([]byte{0xcc}, 8)
		got, err := filestore.ReadPrefix(t.Context(), filestore.PrefixReadRequest{Source: prefixSourceRefusal{complete: complete}, Destination: destination})
		if got.Bytes.Uint64() != 0 || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrFilestoreSource) || !bytes.Equal(destination, make([]byte, 8)) {
			t.Fatalf("complete=%t observed=%d provisional cleared=%t error=%v", complete, got.Bytes.Uint64(), bytes.Equal(destination, make([]byte, 8)), err)
		}
	}
}

func TestPrefixReadInvalidIngressLeavesBorrowedDataUntouched(t *testing.T) {
	t.Parallel()
	for _, source := range []io.Reader{nil, (*bytes.Reader)(nil)} {
		destination := []byte{0xcc}
		got, err := filestore.ReadPrefix(t.Context(), filestore.PrefixReadRequest{Source: source, Destination: destination})
		if !errors.Is(err, core.ErrFilestoreContract) || got.Bytes.Uint64() != 0 || destination[0] != 0xcc {
			t.Fatalf("invalid source touched borrowed data: %v", err)
		}
	}
	destination := []byte{0xcc}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	got, err := filestore.ReadPrefix(ctx, filestore.PrefixReadRequest{Source: bytes.NewReader([]byte{'x'}), Destination: destination})
	if got.Bytes.Uint64() != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) || destination[0] != 0xcc {
		t.Fatalf("canceled prefix touched borrowed data or lost cause: %v", err)
	}
	if _, err := filestore.ReadPrefix(nil, filestore.PrefixReadRequest{Source: bytes.NewReader(nil), Destination: destination}); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context admitted: %v", err)
	}
}

func FuzzPrefixReadConservesWindowAndUnconsumedSource(f *testing.F) {
	f.Add([]byte("native prefix and tail"), uint8(6))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("no read"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, window uint8) {
		before := bytes.Clone(data)
		source := bytes.NewReader(data)
		destination := bytes.Repeat([]byte{0xcc}, int(window))
		got, err := filestore.ReadPrefix(t.Context(), filestore.PrefixReadRequest{Source: source, Destination: destination})
		want := min(len(data), int(window))
		if err != nil || got.Bytes.Uint64() != uint64(want) || !bytes.Equal(destination[:want], data[:want]) || source.Len() != len(data)-want || !bytes.Equal(data, before) {
			t.Fatalf("native prefix disagrees with source/window conservation: %v", err)
		}
	})
}
