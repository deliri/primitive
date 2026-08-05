package core_test

import (
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// digestStreamSizes walks both sides of every boundary a streaming SHA-256
// implementation is likely to get wrong: nothing, one byte, and the bytes on
// either side of the 64-byte block, the 32-byte digest, and the buffer sizes
// callers actually stream with.
func digestStreamSizes() []int {
	return []int{
		0, 1, 2, 3,
		31, 32, 33,
		55, 56, 57,
		63, 64, 65,
		119, 120, 121,
		127, 128, 129,
		255, 256, 257,
		511, 512, 513,
		4095, 4096, 4097,
	}
}

// digestStreamContent builds deterministic bytes. It is not random: the same
// length must produce the same bytes so a failure names a length, not a seed.
func digestStreamContent(length int) []byte {
	content := make([]byte, length)
	for index := range content {
		content[index] = byte(index*7 + 11)
	}
	return content
}

// TestDigestWriterMatchesTheStandardLibraryAcrossEveryStreamBoundary is a
// differential proof. The oracle is crypto/sha256's one-shot Sum256 over the
// same bytes, so the table cannot pass by restating the implementation it is
// checking. Each length is streamed three ways, because a hasher that is
// correct only when the caller happens to write everything at once is not
// correct.
func TestDigestWriterMatchesTheStandardLibraryAcrossEveryStreamBoundary(t *testing.T) {
	t.Parallel()

	splits := []struct {
		write func(*testing.T, *core.DigestWriter, []byte)
		name  string
	}{
		{
			name: "one write of the whole stream",
			write: func(t *testing.T, writer *core.DigestWriter, content []byte) {
				requireDigestWrite(t, writer, content)
			},
		},
		{
			name: "one write per byte",
			write: func(t *testing.T, writer *core.DigestWriter, content []byte) {
				for index := range content {
					requireDigestWrite(t, writer, content[index:index+1])
				}
			},
		},
		{
			name: "split at the midpoint with an empty write between",
			write: func(t *testing.T, writer *core.DigestWriter, content []byte) {
				middle := len(content) / 2
				requireDigestWrite(t, writer, content[:middle])
				requireDigestWrite(t, writer, nil)
				requireDigestWrite(t, writer, content[middle:])
			},
		},
	}

	for _, split := range splits {
		t.Run(split.name, func(t *testing.T) {
			t.Parallel()

			for _, length := range digestStreamSizes() {
				content := digestStreamContent(length)
				writer := core.NewDigestWriter()
				split.write(t, writer, content)

				gotDigest, gotLength, err := writer.Seal()
				if err != nil {
					t.Fatalf("Seal(%d bytes) error = %v, want nil", length, err)
				}
				wantDigest := core.NewSHA256Digest(sha256.Sum256(content))
				if gotDigest != wantDigest {
					t.Fatalf("Seal(%d bytes) digest = %v, want %v", length, gotDigest, wantDigest)
				}
				if gotLength.Uint64() != uint64(length) {
					t.Fatalf("Seal(%d bytes) length = %d, want %d", length, gotLength.Uint64(), length)
				}
			}
		})
	}
}

func requireDigestWrite(t *testing.T, writer *core.DigestWriter, data []byte) {
	t.Helper()
	written, err := writer.Write(data)
	if err != nil {
		t.Fatalf("Write(%d bytes) error = %v, want nil", len(data), err)
	}
	if written != len(data) {
		t.Fatalf("Write(%d bytes) = %d, want %d", len(data), written, len(data))
	}
}

// TestDigestWriterSealIsOneWay proves the seal is a real boundary and not a
// formality. A writer that accepted bytes after answering would let two callers
// hold two different digests for the same writer, and only one of them could be
// describing what was actually hashed.
func TestDigestWriterSealIsOneWay(t *testing.T) {
	t.Parallel()

	writer := core.NewDigestWriter()
	requireDigestWrite(t, writer, []byte("sealed content"))

	firstDigest, firstLength, err := writer.Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v, want nil", err)
	}

	if _, err := writer.Write([]byte("more")); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Write(after seal) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}

	secondDigest, secondLength, err := writer.Seal()
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Seal(after a refused write) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	if secondDigest != (core.SHA256Digest{}) || secondLength != (core.ByteLength{}) {
		t.Fatalf("Seal(after a refused write) = (%v, %v), want zero values alongside the refusal",
			secondDigest, secondLength)
	}
	if firstDigest != core.NewSHA256Digest(sha256.Sum256([]byte("sealed content"))) {
		t.Fatalf("Seal() digest = %v, want the digest of exactly the sealed bytes", firstDigest)
	}
	if firstLength.Uint64() != uint64(len("sealed content")) {
		t.Fatalf("Seal() length = %d, want %d", firstLength.Uint64(), len("sealed content"))
	}
}

// TestDigestWriterSealTwiceReturnsTheSameAnswer separates asking again from
// mutating. Reading the answer twice is not a second stream, so it must not be
// refused and must not drift.
func TestDigestWriterSealTwiceReturnsTheSameAnswer(t *testing.T) {
	t.Parallel()

	writer := core.NewDigestWriter()
	requireDigestWrite(t, writer, []byte("idempotent"))

	firstDigest, firstLength, err := writer.Seal()
	if err != nil {
		t.Fatalf("first Seal() error = %v, want nil", err)
	}
	secondDigest, secondLength, err := writer.Seal()
	if err != nil {
		t.Fatalf("second Seal() error = %v, want nil", err)
	}
	if firstDigest != secondDigest || firstLength != secondLength {
		t.Fatalf("Seal() twice = (%v, %v) then (%v, %v), want identical answers",
			firstDigest, firstLength, secondDigest, secondLength)
	}
}

// TestDigestWriterRefusesANilReceiver proves the zero-usable state is refused
// rather than panicking into a caller that is streaming evidence.
func TestDigestWriterRefusesANilReceiver(t *testing.T) {
	t.Parallel()

	var writer *core.DigestWriter

	written, err := writer.Write([]byte("content"))
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Write(nil receiver) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	if written != 0 {
		t.Fatalf("Write(nil receiver) = %d, want 0", written)
	}

	digest, length, err := writer.Seal()
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Seal(nil receiver) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	if digest != (core.SHA256Digest{}) || length != (core.ByteLength{}) {
		t.Fatalf("Seal(nil receiver) = (%v, %v), want zero values", digest, length)
	}
}

// TestDigestWriterEmptyStreamSealsRatherThanFailing pins the neutral case. The
// digest of no bytes is a real, well-known value, and whether an empty body is
// acceptable is the caller's rule: attest refuses one, upgrade never sees one.
// Deciding it here would put a product rule in Core.
func TestDigestWriterEmptyStreamSealsRatherThanFailing(t *testing.T) {
	t.Parallel()

	digest, length, err := core.NewDigestWriter().Seal()
	if err != nil {
		t.Fatalf("Seal(empty stream) error = %v, want nil", err)
	}
	if digest != core.NewSHA256Digest(sha256.Sum256(nil)) {
		t.Fatalf("Seal(empty stream) digest = %v, want the digest of no bytes", digest)
	}
	if length.Uint64() != 0 {
		t.Fatalf("Seal(empty stream) length = %d, want 0", length.Uint64())
	}
}

// TestDigestWriterComposesThroughStandardLibraryWriters proves the type is an
// ordinary io.Writer, which is the whole reason it is not a private
// accumulator: upgrade hashes and CRCs one stream through io.MultiWriter, and
// every consumer streams into it with io.Copy.
func TestDigestWriterComposesThroughStandardLibraryWriters(t *testing.T) {
	t.Parallel()

	content := digestStreamContent(4097)

	direct := core.NewDigestWriter()
	requireDigestWrite(t, direct, content)
	wantDigest, wantLength, err := direct.Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v, want nil", err)
	}

	through := core.NewDigestWriter()
	beside := core.NewDigestWriter()
	copied, err := io.Copy(io.MultiWriter(through, beside), newSlowReader(content))
	if err != nil {
		t.Fatalf("Copy() error = %v, want nil", err)
	}
	if copied != int64(len(content)) {
		t.Fatalf("Copy() = %d, want %d", copied, len(content))
	}

	gotDigest, gotLength, err := through.Seal()
	if err != nil {
		t.Fatalf("Seal(through MultiWriter) error = %v, want nil", err)
	}
	if gotDigest != wantDigest || gotLength != wantLength {
		t.Fatalf("Seal(through MultiWriter) = (%v, %v), want (%v, %v)",
			gotDigest, gotLength, wantDigest, wantLength)
	}
	besideDigest, _, err := beside.Seal()
	if err != nil || besideDigest != wantDigest {
		t.Fatalf("Seal(second MultiWriter branch) = (%v, %v), want (%v, nil)",
			besideDigest, err, wantDigest)
	}
}

// slowReader hands back one short read at a time so io.Copy is forced to make
// many small writes. A hasher that only survives whole-buffer writes is not
// usable over a network or a file stream.
type slowReader struct {
	content []byte
	offset  int
}

func newSlowReader(content []byte) *slowReader {
	return &slowReader{content: content}
}

func (r *slowReader) Read(destination []byte) (int, error) {
	if r.offset >= len(r.content) {
		return 0, io.EOF
	}
	if len(destination) == 0 {
		return 0, nil
	}
	written := copy(destination[:1], r.content[r.offset:])
	r.offset += written
	return written, nil
}
