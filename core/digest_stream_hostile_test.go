package core_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"slices"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func digestStreamSizes() []int {
	return []int{0, 1, 2, 3, 31, 32, 33, 55, 56, 57, 63, 64, 65, 119, 120, 121, 127, 128, 129, 255, 256, 257, 511, 512, 513, 4095, 4096, 4097}
}
func digestStreamContent(length int) []byte {
	content := make([]byte, length)
	for index := range content {
		content[index] = byte(index*7 + 11)
	}
	return content
}
func digestBoundaryOffsets(length int) []int {
	offsets := append([]int{0, length / 2, length}, digestStreamSizes()...)
	offsets = slices.DeleteFunc(offsets, func(n int) bool { return n > length })
	slices.Sort(offsets)
	return slices.Compact(offsets)
}

func TestDigestWriterMatchesGoAtEveryStreamBoundary(t *testing.T) {
	t.Parallel()
	shapes := []struct {
		name    string
		offsets func(int) []int
	}{
		{name: "whole stream", offsets: func(n int) []int { return []int{n} }},
		{name: "one byte writes", offsets: func(n int) []int {
			out := make([]int, n+1)
			for i := range out {
				out[i] = i
			}
			return out
		}},
		{name: "boundary chunks with empty writes", offsets: digestBoundaryOffsets},
	}
	for _, shape := range shapes {
		for _, size := range digestStreamSizes() {
			t.Run(shape.name+"/bytes="+strconv.Itoa(size), func(t *testing.T) {
				t.Parallel()
				content := digestStreamContent(size)
				writer := core.NewDigestWriter()
				previous := 0
				for _, offset := range shape.offsets(size) {
					chunk := content[previous:offset]
					n, err := writer.Write(chunk)
					if err != nil || n != len(chunk) {
						t.Fatalf("write prefix %d=%d, %v; want %d bytes", offset, n, err, len(chunk))
					}
					empty, err := writer.Write(nil)
					if err != nil || empty != 0 {
						t.Fatalf("empty write=%d, %v; want zero and nil", empty, err)
					}
					want := core.NewSHA256Digest(sha256.Sum256(content[:offset]))
					for repeat := range 2 {
						sum, count, err := writer.Digest()
						if err != nil || sum != want || count.Uint64() != uint64(offset) {
							t.Fatalf("peek %d at prefix %d=%v, %v, %v; want exact prefix digest %v", repeat, offset, sum, count, err, want)
						}
					}
					previous = offset
				}
				want := core.NewSHA256Digest(sha256.Sum256(content))
				for repeat := range 2 {
					sum, count, err := writer.Seal()
					peek, peekCount, peekErr := writer.Digest()
					if err != nil || sum != want || count.Uint64() != uint64(size) || peekErr != nil || peek != sum || peekCount != count {
						t.Fatalf("seal %d=%v, %v, %v; peek=%v, %v, %v; want exact %d-byte Go digest %v", repeat, sum, count, err, peek, peekCount, peekErr, size, want)
					}
				}
				n, err := writer.Write([]byte{0xff})
				if n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("sealed write=%d, %v; want no consumption and refusal", n, err)
				}
				sum, count, err := writer.Digest()
				sealed, sealedCount, sealedErr := writer.Seal()
				if sum != (core.SHA256Digest{}) || count != (core.ByteLength{}) || !errors.Is(err, core.ErrPrimitiveContract) || sealed != (core.SHA256Digest{}) || sealedCount != (core.ByteLength{}) || !errors.Is(sealedErr, core.ErrPrimitiveContract) {
					t.Fatalf("latched peek=%v, %v, %v; seal=%v, %v, %v; want zero publication", sum, count, err, sealed, sealedCount, sealedErr)
				}
			})
		}
	}
}

func TestDigestWriterResetDiscardsEveryPriorStream(t *testing.T) {
	t.Parallel()
	sizes := []int{0, 1, 32, 63, 64, 65, 128, 4096, 4097}
	states := []struct {
		name         string
		seal, refuse bool
	}{
		{name: "open"}, {name: "sealed", seal: true}, {name: "latched refusal", seal: true, refuse: true},
	}
	for _, state := range states {
		for _, firstSize := range sizes {
			for _, secondSize := range sizes {
				t.Run(state.name+"/"+strconv.Itoa(firstSize)+" to "+strconv.Itoa(secondSize), func(t *testing.T) {
					t.Parallel()
					first, second := digestStreamContent(firstSize), digestStreamContent(secondSize)
					for i := range second {
						second[i] ^= 0xff
					}
					writer := core.NewDigestWriter()
					n, err := writer.Write(first)
					if err != nil || n != len(first) {
						t.Fatalf("first write=%d, %v; want %d", n, err, len(first))
					}
					if state.seal {
						sum, count, err := writer.Seal()
						if err != nil || sum != core.NewSHA256Digest(sha256.Sum256(first)) || count.Uint64() != uint64(firstSize) {
							t.Fatalf("first seal=%v, %v, %v; want exact first stream", sum, count, err)
						}
					}
					if state.refuse {
						n, err := writer.Write(nil)
						if n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
							t.Fatalf("sealed empty write=%d, %v; want latched refusal", n, err)
						}
					}
					if err := writer.Reset(); err != nil {
						t.Fatal(err)
					}
					empty, count, err := writer.Digest()
					if err != nil || empty != core.NewSHA256Digest(sha256.Sum256(nil)) || count != (core.ByteLength{}) {
						t.Fatalf("reset digest=%v, %v, %v; want empty Go stream", empty, count, err)
					}
					n, err = writer.Write(second)
					if err != nil || n != len(second) {
						t.Fatalf("second write=%d, %v; want %d", n, err, len(second))
					}
					sum, count, err := writer.Seal()
					want := core.NewSHA256Digest(sha256.Sum256(second))
					if err != nil || sum != want || count.Uint64() != uint64(secondSize) {
						t.Fatalf("reset seal=%v, %v, %v; want only second stream %v, %d", sum, count, err, want, secondSize)
					}
				})
			}
		}
	}
}

func TestDigestWriterRefusesUnconstructedReceivers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		make func() *core.DigestWriter
	}{
		{name: "nil", make: func() *core.DigestWriter { return nil }},
		{name: "allocated zero", make: func() *core.DigestWriter { return new(core.DigestWriter) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Fresh receivers keep one refusal from masking another door's behavior.
			n, err := tc.make().Write([]byte{0xff})
			if n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("write=%d, %v; want zero and refusal", n, err)
			}
			sum, count, err := tc.make().Digest()
			if sum != (core.SHA256Digest{}) || count != (core.ByteLength{}) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("peek=%v, %v, %v; want zero and refusal", sum, count, err)
			}
			sum, count, err = tc.make().Seal()
			if sum != (core.SHA256Digest{}) || count != (core.ByteLength{}) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("seal=%v, %v, %v; want zero and refusal", sum, count, err)
			}
			writer := tc.make()
			if err := writer.Reset(); !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("reset=%v; want refusal without repair", err)
			}
			n, err = writer.Write(nil)
			if n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("write after refused reset=%d, %v; want zero and refusal", n, err)
			}
		})
	}
}

func TestDigestWriterComposesThroughGoWriters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		reader func([]byte) io.Reader
	}{
		{name: "bulk", reader: func(data []byte) io.Reader { return bytes.NewReader(data) }},
		{name: "single bytes", reader: func(data []byte) io.Reader { return &slowReader{content: data} }},
	}
	for _, tc := range cases {
		for _, size := range digestStreamSizes() {
			t.Run(tc.name+"/bytes="+strconv.Itoa(size), func(t *testing.T) {
				t.Parallel()
				data := digestStreamContent(size)
				first, second := core.NewDigestWriter(), core.NewDigestWriter()
				copied, err := io.Copy(io.MultiWriter(first, second), tc.reader(data))
				if err != nil || copied != int64(size) {
					t.Fatalf("copy=%d, %v; want %d bytes", copied, err, size)
				}
				want := core.NewSHA256Digest(sha256.Sum256(data))
				for branch, writer := range []*core.DigestWriter{first, second} {
					sum, count, err := writer.Seal()
					if err != nil || sum != want || count.Uint64() != uint64(size) {
						t.Fatalf("branch %d=%v, %v, %v; want exact Go digest %v, %d", branch, sum, count, err, want, size)
					}
				}
			})
		}
	}
}

type slowReader struct {
	content []byte
	offset  int
}

func (r *slowReader) Read(destination []byte) (int, error) {
	if r.offset >= len(r.content) {
		return 0, io.EOF
	}
	if len(destination) == 0 {
		return 0, nil
	}
	n := copy(destination[:1], r.content[r.offset:])
	r.offset += n
	return n, nil
}
