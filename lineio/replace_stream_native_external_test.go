package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func replacementMemory(t testing.TB, size uint64) core.ByteCount {
	t.Helper()
	extent, err := core.NewByteCount(size)
	if err != nil {
		t.Fatal(err)
	}
	return extent
}

func TestReplacementNativeWindowMatchesIndependentByteMeaning(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct {
		name                       string
		pattern, replacement, body []byte
	}{
		{name: "empty", pattern: []byte("home"), replacement: []byte("$HOME")},
		{name: "absent", pattern: []byte("home"), replacement: []byte("$HOME"), body: []byte("different")},
		{name: "exact", pattern: []byte("home"), replacement: []byte("$HOME"), body: []byte("home")},
		{name: "boundary", pattern: []byte("home"), replacement: []byte("$HOME"), body: []byte("prefix/home/child/home/end")},
		{name: "overlap", pattern: []byte("aba"), replacement: []byte("R"), body: []byte("abababa")},
		{name: "deletion", pattern: []byte("home"), body: []byte("home-homehome-")},
		{name: "replacement is not rescanned", pattern: []byte("home"), replacement: []byte("homehome"), body: []byte("homehomehome")},
		{name: "partial suffix", pattern: []byte("home"), replacement: []byte("$HOME"), body: []byte("homeho")},
		{name: "binary", pattern: []byte{0, 255}, replacement: []byte{255, 0, 1}, body: []byte{0, 255, 0, 255, 0}},
		{name: "native unicode bytes", pattern: []byte("/home/é"), replacement: []byte("$HOME"), body: []byte("α/home/é/δ/home/é")},
	} {
		for _, size := range []uint64{1, 2, 3, 4, 7, 16, 65536} {
			t.Run(shape.name+"/"+strconv.FormatUint(size, 10), func(t *testing.T) {
				t.Parallel()
				var out bytes.Buffer
				got, err := lineio.ReplaceStream(t.Context(), lineio.ReplaceStreamRequest{Source: iotest.OneByteReader(bytes.NewReader(shape.body)), Destination: &out, Literal: lineio.LiteralReplacement{Pattern: shape.pattern, Replacement: shape.replacement}, BufferBytes: replacementMemory(t, size)})
				want := bytes.ReplaceAll(shape.body, shape.pattern, shape.replacement)
				if err != nil || !bytes.Equal(out.Bytes(), want) {
					t.Fatalf("native replacement=%x,%v; want=%x", out.Bytes(), err, want)
				}
				input, inputErr := got.InputBytes.Int64()
				output, outputErr := got.OutputBytes.Int64()
				if inputErr != nil || outputErr != nil || input != int64(len(shape.body)) || output != int64(len(want)) || uint64(got.Replacements) != uint64(bytes.Count(shape.body, shape.pattern)) {
					t.Fatalf("counts=%+v input=%d/%v output=%d/%v", got, input, inputErr, output, outputErr)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("observation validation=%v", err)
				}
			})
		}
	}
}

type replacementFaultWriter struct {
	native error
	count  int
}

func (w *replacementFaultWriter) Write(p []byte) (int, error) {
	if w.count < 0 {
		return -1, w.native
	}
	if w.count > len(p) {
		return len(p) + 1, w.native
	}
	return w.count, w.native
}

func TestReplacementNativeFailuresWithholdReceipt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		source      io.Reader
		destination io.Writer
		native      error
	}{
		{name: "source", source: io.MultiReader(bytes.NewReader([]byte("home")), iotest.ErrReader(io.ErrUnexpectedEOF)), destination: io.Discard, native: io.ErrUnexpectedEOF},
		{name: "joined EOF", source: io.MultiReader(bytes.NewReader([]byte("home")), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe))), destination: io.Discard, native: io.ErrClosedPipe},
		{name: "destination", source: bytes.NewReader([]byte("home")), destination: &replacementFaultWriter{native: io.ErrClosedPipe}, native: io.ErrClosedPipe},
		{name: "short destination", source: bytes.NewReader([]byte("home")), destination: &replacementFaultWriter{}, native: io.ErrShortWrite},
		{name: "negative destination", source: bytes.NewReader([]byte("home")), destination: &replacementFaultWriter{count: -1}, native: core.ErrLineIOContract},
		{name: "invalid destination", source: bytes.NewReader([]byte("home")), destination: &replacementFaultWriter{count: 99}, native: core.ErrLineIOContract},
		{name: "nil source", destination: io.Discard, native: core.ErrLineIOContract},
		{name: "typed nil source", source: (*bytes.Reader)(nil), destination: io.Discard, native: core.ErrLineIOContract},
		{name: "nil destination", source: bytes.NewReader(nil), native: core.ErrLineIOContract},
		{name: "typed nil destination", source: bytes.NewReader(nil), destination: (*bytes.Buffer)(nil), native: core.ErrLineIOContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := lineio.ReplaceStream(t.Context(), lineio.ReplaceStreamRequest{Source: tc.source, Destination: tc.destination, Literal: lineio.LiteralReplacement{Pattern: []byte("home"), Replacement: []byte("$HOME")}, BufferBytes: replacementMemory(t, 3)})
			if got != (lineio.ReplaceStreamObservation{}) || !errors.Is(err, tc.native) {
				t.Fatalf("failure receipt=%+v,%v; want native=%v", got, err, tc.native)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := lineio.ReplaceStream(ctx, lineio.ReplaceStreamRequest{Source: bytes.NewReader([]byte("home")), Destination: io.Discard, Literal: lineio.LiteralReplacement{Pattern: []byte("home")}, BufferBytes: replacementMemory(t, 3)})
	if got != (lineio.ReplaceStreamObservation{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%+v,%v", got, err)
	}
}

func FuzzReplacementNativeMatchesIndependentByteOracle(f *testing.F) {
	f.Add([]byte("abababa"), []byte("aba"), []byte("X"), uint16(1))
	f.Add([]byte{0, 255, 0, 255}, []byte{0, 255}, []byte{}, uint16(16))
	f.Fuzz(func(t *testing.T, body, pattern, replacement []byte, size uint16) {
		var out bytes.Buffer
		memory := uint64(size) + 1
		got, err := lineio.ReplaceStream(t.Context(), lineio.ReplaceStreamRequest{Source: bytes.NewReader(body), Destination: &out, Literal: lineio.LiteralReplacement{Pattern: pattern, Replacement: replacement}, BufferBytes: replacementMemory(t, memory)})
		if len(pattern) == 0 {
			if got != (lineio.ReplaceStreamObservation{}) || !errors.Is(err, core.ErrLineIOContract) || out.Len() != 0 {
				t.Fatalf("empty pattern=%+v,%v output=%x", got, err, out.Bytes())
			}
			return
		}
		want := bytes.ReplaceAll(body, pattern, replacement)
		if err != nil || !bytes.Equal(out.Bytes(), want) || uint64(got.Replacements) != uint64(bytes.Count(body, pattern)) {
			t.Fatalf("byte meaning=%x,%+v,%v; want=%x", out.Bytes(), got, err, want)
		}
	})
}

// The source owns one byte value rather than a body-sized allocation.
type replacementExtentSource struct{}

func (*replacementExtentSource) Read(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = 'x'
	}
	return len(destination), nil
}

func TestReplacementNativeExtentDoesNotChangeWorkingMemoryContract(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{65535, 65536, 65537, 8*1024*1024 + 1, 64*1024*1024 + 1} {
		t.Run(strconv.FormatInt(extent, 10), func(t *testing.T) {
			t.Parallel()
			got, err := lineio.ReplaceStream(t.Context(), lineio.ReplaceStreamRequest{Source: io.LimitReader(&replacementExtentSource{}, extent), Destination: io.Discard, Literal: lineio.LiteralReplacement{Pattern: []byte("home"), Replacement: []byte("$HOME")}, BufferBytes: replacementMemory(t, 65536)})
			input, inputErr := got.InputBytes.Int64()
			output, outputErr := got.OutputBytes.Int64()
			if err != nil || inputErr != nil || outputErr != nil || input != extent || output != extent || got.Replacements != 0 {
				t.Fatalf("generated native extent receipt=%+v,%v; input=%d/%v output=%d/%v want=%d", got, err, input, inputErr, output, outputErr, extent)
			}
		})
	}
}

func BenchmarkReplacementNativeFixedWorkingMemory(b *testing.B) {
	for _, extent := range []int64{65535, 65536, 65537, 8*1024*1024 + 1, 64*1024*1024 + 1} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			memory := replacementMemory(b, 65536)
			literal := lineio.LiteralReplacement{Pattern: []byte("home"), Replacement: []byte("$HOME")}
			b.ReportAllocs()
			b.SetBytes(extent)
			b.ResetTimer()
			for range b.N {
				got, err := lineio.ReplaceStream(b.Context(), lineio.ReplaceStreamRequest{Source: io.LimitReader(&replacementExtentSource{}, extent), Destination: io.Discard, Literal: literal, BufferBytes: memory})
				input, inputErr := got.InputBytes.Int64()
				output, outputErr := got.OutputBytes.Int64()
				if err != nil || inputErr != nil || outputErr != nil || input != extent || output != extent || got.Replacements != 0 {
					b.Fatalf("native benchmark receipt=%+v,%v", got, err)
				}
			}
		})
	}
}
