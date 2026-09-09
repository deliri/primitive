package objectstore_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func FuzzExactReaderExtentAndBytes(f *testing.F) {
	for _, seed := range []struct {
		data            []byte
		declared, chunk uint16
	}{
		{chunk: 1},
		{data: []byte{0x81}, chunk: 1},
		{data: []byte{0x81}, declared: 1, chunk: 2},
		{data: []byte{0, 0x81, 0xff}, declared: 2, chunk: 1},
		{data: []byte{0, 0x81, 0xff}, declared: 4, chunk: 2},
		{data: bytes.Repeat([]byte{0, 0x81, 0xff}, 10923), declared: 32768, chunk: 32768},
		{data: []byte{0x81}, declared: 65535, chunk: 0},
	} {
		f.Add(seed.data, seed.declared, seed.chunk)
	}
	f.Fuzz(func(t *testing.T, data []byte, declared, chunk uint16) {
		// The campaign bounds source and destination storage; production receives
		// the exact declared extent, including its zero and uint16 ceilings.
		if len(data) > 1<<16 {
			return
		}
		size := max(1, int(chunk))
		source := bytes.NewReader(data)
		reader, err := objectstore.NewExactReader(source, mustByteLength(t, uint64(declared)))
		if err != nil {
			t.Fatal(err)
		}
		buffer := make([]byte, size)
		var got []byte
		if declared == 0 {
			err = reader.ProveEmpty()
		} else {
			for attempt := 0; ; attempt++ {
				if attempt > int(declared)+1 {
					t.Fatalf("read attempts = %d, want at most declared extent %d plus one EOF read", attempt, declared)
				}
				n, readErr := reader.Read(buffer)
				if n < 0 || n > size {
					t.Fatalf("invalid destination count %d", n)
				}
				got = append(got, buffer[:n]...)
				if readErr != nil {
					err = readErr
					break
				}
			}
		}
		wantCount := len(data)
		if int(declared) < len(data) {
			// An overlong source may expose only complete chunks preceding the
			// final declared chunk. That final chunk must be withheld entirely.
			wantCount = 0
			if declared != 0 {
				wantCount = ((int(declared) - 1) / size) * size
			}
		}
		if !bytes.Equal(got, data[:wantCount]) || source.Len() != len(data)-min(len(data), int(declared)) {
			t.Fatalf("delivered %x and remaining %d, want exact prefix %x and remaining %d", got, source.Len(), data[:wantCount], len(data)-min(len(data), int(declared)))
		}
		if int(declared) == len(data) {
			if reader.Failure() != nil || (err != nil && !errors.Is(err, io.EOF)) {
				t.Fatalf("exact stream rejected: %v / %v", err, reader.Failure())
			}
		} else if !errors.Is(err, core.ErrObjectStoreSource) || !errors.Is(err, core.ErrObjectStoreIntegrity) || !errors.Is(reader.Failure(), core.ErrObjectStoreSource) {
			t.Fatalf("mismatched extent produced no source-integrity refusal: %v / %v", err, reader.Failure())
		}
		remaining := source.Len()
		n, terminal := reader.Read(buffer)
		wantTerminal := io.EOF
		if reader.Failure() != nil {
			wantTerminal = core.ErrObjectStoreSource
		}
		if n != 0 || !errors.Is(terminal, wantTerminal) || source.Len() != remaining {
			t.Fatalf("terminal read = (%d, %v), remaining=%d; want (0, %v), remaining=%d", n, terminal, source.Len(), wantTerminal, remaining)
		}
	})
}
