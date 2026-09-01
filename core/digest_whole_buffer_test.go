package core_test

import (
	"crypto/sha256"
	"math/rand/v2"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// wholeBufferCase is one hostile in-memory buffer exercised against every
// whole-buffer digest door. The rows deliberately span the empty and nil
// buffers, single low and high bytes, embedded nulls, and the sha256 block
// boundary in both directions so a door that mishandles length framing or a
// specific byte cannot pass by luck.
type wholeBufferCase struct {
	name string
	data []byte
}

func wholeBufferHostileCases() []wholeBufferCase {
	return []wholeBufferCase{
		{name: "the empty buffer hashes to the known empty digest", data: []byte{}},
		{name: "a nil buffer hashes like the empty buffer", data: nil},
		{name: "one zero byte", data: []byte{0}},
		{name: "one high byte", data: []byte{0xff}},
		{name: "a short ascii buffer", data: []byte("caller rides primitive")},
		{name: "a buffer with embedded nulls", data: []byte("a\x00b\x00c")},
		{name: "exactly one sha256 block", data: make([]byte, sha256.BlockSize)},
		{name: "one byte over a block", data: make([]byte, sha256.BlockSize+1)},
		{name: "one byte under a block", data: make([]byte, sha256.BlockSize-1)},
		{name: "a large multi-block buffer", data: make([]byte, 1<<16)},
	}
}

func TestSHA256OfMatchesTheStandardLibraryOnEveryBuffer(t *testing.T) {
	t.Parallel()

	for _, tc := range wholeBufferHostileCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := sha256.Sum256(tc.data)
			got := core.SHA256Of(tc.data)
			gotBytes, err := got.Bytes()
			if err != nil {
				t.Fatalf("SHA256Of(%q).Bytes() error = %v, want nil", tc.name, err)
			}
			if gotBytes != want {
				t.Fatalf("SHA256Of(%q) = %x, want %x", tc.name, gotBytes, want)
			}
		})
	}
}

func TestSHA256BytesOfMatchesTheStandardLibraryOnEveryBuffer(t *testing.T) {
	t.Parallel()

	for _, tc := range wholeBufferHostileCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := sha256.Sum256(tc.data)
			if got := core.SHA256BytesOf(tc.data); got != want {
				t.Fatalf("SHA256BytesOf(%q) = %x, want %x", tc.name, got, want)
			}
		})
	}
}

func TestSHA256OfAgreesWithTheStreamingWriter(t *testing.T) {
	t.Parallel()

	// The whole-buffer door and the streaming door must produce the same digest
	// for the same bytes, or a caller choosing between them by convenience would
	// change the answer.
	source := rand.NewChaCha8([32]byte{1, 2, 3, 4})
	for size := 0; size <= 4096; size += 337 {
		data := make([]byte, size)
		_, _ = source.Read(data)

		whole, err := core.SHA256Of(data).Bytes()
		if err != nil {
			t.Fatalf("SHA256Of(size=%d).Bytes() error = %v, want nil", size, err)
		}
		writer := core.NewDigestWriter()
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("DigestWriter.Write(size=%d) error = %v, want nil", size, err)
		}
		sealed, length, err := writer.Seal()
		if err != nil {
			t.Fatalf("DigestWriter.Seal(size=%d) error = %v, want nil", size, err)
		}
		streamed, err := sealed.Bytes()
		if err != nil {
			t.Fatalf("sealed digest Bytes(size=%d) error = %v, want nil", size, err)
		}
		if whole != streamed {
			t.Fatalf("size=%d whole=%x streamed=%x, want equal", size, whole, streamed)
		}
		if length.Uint64() != uint64(size) {
			t.Fatalf("size=%d sealed length = %d, want %d", size, length.Uint64(), size)
		}
	}
}

func TestSHA256BytesOfAgreesWithTheDigestBytesDoor(t *testing.T) {
	t.Parallel()

	// The infallible bytes door and the fallible SHA256Digest.Bytes door must
	// return the identical array for the same bytes.
	source := rand.NewChaCha8([32]byte{9, 10, 11, 12})
	for size := 0; size <= 4096; size += 337 {
		data := make([]byte, size)
		_, _ = source.Read(data)

		viaDigest, err := core.SHA256Of(data).Bytes()
		if err != nil {
			t.Fatalf("SHA256Of(size=%d).Bytes() error = %v, want nil", size, err)
		}
		if got := core.SHA256BytesOf(data); got != viaDigest {
			t.Fatalf("size=%d SHA256BytesOf = %x, digest Bytes = %x, want equal", size, got, viaDigest)
		}
	}
}
