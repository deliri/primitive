package lineio_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

const fragmentFuzzInputBudget = 4096

func FuzzReaderFragmentConservation(f *testing.F) {
	request := lineio.Request{Source: strings.NewReader("seed\r\n"), BufferBytes: mustByteCount(f, 64)}
	if err := request.Validate(); err != nil {
		f.Fatalf("seed Validate() = %v, want nil", err)
	}
	reader, err := lineio.New(request)
	if err != nil {
		f.Fatalf("seed New() = %v, want nil", err)
	}
	fragment, err := reader.ReadFragment()
	if err != nil || fragment.More {
		f.Fatalf("seed fragment = (%v,%v), want complete line", fragment, err)
	}
	f.Add(bytes.Clone(fragment.Bytes), uint8(63), uint8(1))
	for _, input := range [][]byte{
		{}, {'\n'}, {'\r'}, {'\r', '\n'}, {'\r', '\r', '\n'},
		[]byte("x\x00\xff\n\nlast"),
		bytes.Repeat([]byte{'x'}, 63), bytes.Repeat([]byte{'x'}, 64), bytes.Repeat([]byte{'x'}, 65),
		append(bytes.Repeat([]byte{'x'}, 63), '\r', '\n'), bytes.Repeat([]byte{'x'}, fragmentFuzzInputBudget),
	} {
		f.Add(input, uint8(63), uint8(0))
	}
	f.Fuzz(func(t *testing.T, input []byte, buffer, chunk uint8) {
		if len(input) > fragmentFuzzInputBudget {
			return
		}
		reader, err := lineio.New(lineio.Request{Source: &terminalReader{Reader: strings.NewReader(string(input)), cause: io.EOF, chunk: int(chunk) + 1}, BufferBytes: mustByteCount(t, uint64(buffer)+1)})
		if err != nil {
			t.Fatalf("New() = %v, want nil", err)
		}
		capacity, err := reader.Capacity()
		if err != nil {
			t.Fatalf("Capacity() = %v, want nil", err)
		}
		capacityBytes, err := capacity.Uint64()
		if err != nil {
			t.Fatalf("capacity value = %v, want nil", err)
		}
		var got []byte
		for {
			fragment, err := reader.ReadFragment()
			if validation := fragment.Validate(); validation != nil {
				t.Fatalf("fragment = %v, want valid", validation)
			}
			if uint64(len(fragment.Bytes)) > capacityBytes || (fragment.More && (uint64(len(fragment.Bytes)) != capacityBytes || err != nil)) {
				t.Fatalf("fragment = (%d bytes, more %t, %v), want bounded capacity %d", len(fragment.Bytes), fragment.More, err, capacityBytes)
			}
			if err == nil && !fragment.More && (len(fragment.Bytes) == 0 || fragment.Bytes[len(fragment.Bytes)-1] != lineio.Delimiter) {
				t.Fatalf("line completion = %q, want LF", fragment.Bytes)
			}
			got = append(got, fragment.Bytes...)
			if len(got) > len(input) {
				t.Fatalf("produced %d bytes, want at most %d", len(got), len(input))
			}
			if err != nil {
				if !errors.Is(err, io.EOF) || errors.Is(err, core.ErrLineIOScan) {
					t.Fatalf("terminal = %v, want EOF", err)
				}
				break
			}
		}
		if !bytes.Equal(got, input) {
			t.Fatalf("bytes = %q, want exact source %q", got, input)
		}
	})
}

func FuzzRequestMemoryConfiguration(f *testing.F) {
	request := lineio.Request{Source: strings.NewReader(""), BufferBytes: mustByteCount(f, 64)}
	if err := request.Validate(); err != nil {
		f.Fatalf("seed Validate() = %v, want nil", err)
	}
	size, err := request.BufferBytes.Uint64()
	if err != nil {
		f.Fatalf("seed buffer = %v, want nil", err)
	}
	f.Add(size)
	for _, size := range []uint64{0, 1, (16 << 20) + 1, math.MaxInt, uint64(math.MaxInt) + 1, math.MaxUint64} {
		f.Add(size)
	}
	f.Fuzz(func(t *testing.T, size uint64) {
		source := &observedReader{Reader: strings.NewReader("")}
		request := lineio.Request{Source: source}
		if size > 0 {
			request.BufferBytes = mustByteCount(t, size)
		}
		wantValid := size > 0 && size <= math.MaxInt
		err := request.Validate()
		if (err == nil) != wantValid || (!wantValid && !errors.Is(err, core.ErrLineIOContract)) {
			t.Fatalf("Validate(%d) = %v, want admitted %t", size, err, wantValid)
		}
		// This is a test allocation budget, never an input/production quota.
		if wantValid && size > fragmentFuzzInputBudget {
			return
		}
		reader, err := lineio.New(request)
		if (err == nil) != wantValid || (reader != nil) != wantValid || source.calls != 0 || (!wantValid && !errors.Is(err, core.ErrLineIOContract)) {
			t.Fatalf("New(%d) = (%v,%v,%d reads), want admitted %t without reading", size, reader, err, source.calls, wantValid)
		}
	})
}
