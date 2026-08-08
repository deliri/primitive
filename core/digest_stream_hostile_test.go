package core_test

import (
	"crypto/sha256"
	"errors"
	"io"
	"sort"
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

// digestBoundaryOffsets returns the peek points a running-total digest is most
// likely to get wrong for a stream of the given length: zero bytes, the bytes
// on either side of the sha256 block and digest boundaries, the midpoint, and
// the final byte. Offsets past the length are dropped so one table drives every
// size, and the set is sorted so a chunked write advances monotonically.
func digestBoundaryOffsets(length int) []int {
	candidates := append([]int{0, length / 2, length}, digestStreamSizes()...)
	seen := make(map[int]bool, len(candidates))
	offsets := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate < 0 || candidate > length || seen[candidate] {
			continue
		}
		seen[candidate] = true
		offsets = append(offsets, candidate)
	}
	sort.Ints(offsets)
	return offsets
}

// TestDigestWriterDigestTracksTheRunningTotalAtEveryBoundary is the peek
// contract a file writer needs: expose the content hash while the file is still
// open, then keep writing. For every stream size the bytes are written in chunks
// that stop on both sides of every sha256 block boundary, and after each chunk
// the peek must equal crypto/sha256 over exactly the bytes written so far. The
// oracle is the standard library over the prefix, so the table cannot pass by
// restating the writer. It goes red if a peek latches the stream (a later chunk
// would be refused), discards it (a later peek would omit the prefix), or
// reports the wrong running length.
func TestDigestWriterDigestTracksTheRunningTotalAtEveryBoundary(t *testing.T) {
	t.Parallel()

	for _, length := range digestStreamSizes() {
		content := digestStreamContent(length)
		writer := core.NewDigestWriter()

		previous := 0
		for _, offset := range digestBoundaryOffsets(length) {
			requireDigestWrite(t, writer, content[previous:offset])
			previous = offset

			gotDigest, gotLength, err := writer.Digest()
			if err != nil {
				t.Fatalf("Digest(%d of %d bytes) error = %v, want nil", offset, length, err)
			}
			if want := core.NewSHA256Digest(sha256.Sum256(content[:offset])); gotDigest != want {
				t.Fatalf("Digest(%d of %d bytes) = %v, want the digest of exactly the prefix %v",
					offset, length, gotDigest, want)
			}
			if gotLength.Uint64() != uint64(offset) {
				t.Fatalf("Digest(%d of %d bytes) length = %d, want %d", offset, length, gotLength.Uint64(), offset)
			}
		}

		gotDigest, gotLength, err := writer.Seal()
		if err != nil {
			t.Fatalf("Seal(%d bytes) error = %v, want nil", length, err)
		}
		if want := core.NewSHA256Digest(sha256.Sum256(content)); gotDigest != want {
			t.Fatalf("Seal(%d bytes) after peeking every boundary = %v, want %v", length, gotDigest, want)
		}
		if gotLength.Uint64() != uint64(length) {
			t.Fatalf("Seal(%d bytes) length = %d, want %d", length, gotLength.Uint64(), length)
		}
	}
}

// TestDigestWriterDigestRepeatsBetweenWritesWithoutMutating proves a peek does
// not disturb the running hash: reading the current digest many times is not
// many streams. Across sizes on both sides of the block boundary it peeks
// repeatedly mid-stream, requires every repeat to match, then writes the rest
// and requires the seal to equal crypto/sha256 over the whole. It goes red if
// driving hash.Hash.Sum finalized or advanced the state, so a later write
// disagreed with the standard library.
func TestDigestWriterDigestRepeatsBetweenWritesWithoutMutating(t *testing.T) {
	t.Parallel()

	for _, length := range []int{0, 1, 32, 63, 64, 65, 4097} {
		content := digestStreamContent(length)
		middle := length / 2
		writer := core.NewDigestWriter()
		requireDigestWrite(t, writer, content[:middle])

		first, firstLength, err := writer.Digest()
		if err != nil {
			t.Fatalf("first Digest(%d of %d bytes) error = %v, want nil", middle, length, err)
		}
		for repeat := range 3 {
			again, againLength, err := writer.Digest()
			if err != nil {
				t.Fatalf("Digest(%d bytes) repeat %d error = %v, want nil", middle, repeat, err)
			}
			if again != first || againLength != firstLength {
				t.Fatalf("Digest(%d bytes) repeat %d = (%v, %v), want the unchanged running total (%v, %v)",
					middle, repeat, again, againLength, first, firstLength)
			}
		}

		requireDigestWrite(t, writer, content[middle:])
		got, gotLength, err := writer.Seal()
		if err != nil {
			t.Fatalf("Seal(%d bytes) error = %v, want nil", length, err)
		}
		if want := core.NewSHA256Digest(sha256.Sum256(content)); got != want {
			t.Fatalf("Seal(%d bytes) after repeated peeks = %v, want %v", length, got, want)
		}
		if gotLength.Uint64() != uint64(length) {
			t.Fatalf("Seal(%d bytes) length = %d, want %d", length, gotLength.Uint64(), length)
		}
	}
}

// TestDigestWriterDigestAfterSealReturnsTheSealedAnswerAtEverySize proves
// reading after the stream is closed is still allowed and still true at every
// boundary: a peek is never a mutation, so it does not trip the sealed latch
// that only refuses further writes.
func TestDigestWriterDigestAfterSealReturnsTheSealedAnswerAtEverySize(t *testing.T) {
	t.Parallel()

	for _, length := range digestStreamSizes() {
		content := digestStreamContent(length)
		writer := core.NewDigestWriter()
		requireDigestWrite(t, writer, content)

		sealDigest, sealLength, err := writer.Seal()
		if err != nil {
			t.Fatalf("Seal(%d bytes) error = %v, want nil", length, err)
		}
		peekDigest, peekLength, err := writer.Digest()
		if err != nil {
			t.Fatalf("Digest(after seal, %d bytes) error = %v, want nil because reading is not a mutation", length, err)
		}
		if peekDigest != sealDigest || peekLength != sealLength {
			t.Fatalf("Digest(after seal, %d bytes) = (%v, %v), want the sealed answer (%v, %v)",
				length, peekDigest, peekLength, sealDigest, sealLength)
		}
	}
}

// TestDigestWriterDigestRefusesANilReceiverAndALatchedError proves the peek
// carries the same refusals as Seal: a zero-usable receiver and a writer whose
// answer was already invalidated by a refused write both fail loudly with zero
// values, never a silent partial digest.
func TestDigestWriterDigestRefusesANilReceiverAndALatchedError(t *testing.T) {
	t.Parallel()

	var nilWriter *core.DigestWriter
	digest, length, err := nilWriter.Digest()
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Digest(nil receiver) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	if digest != (core.SHA256Digest{}) || length != (core.ByteLength{}) {
		t.Fatalf("Digest(nil receiver) = (%v, %v), want zero values", digest, length)
	}

	writer := core.NewDigestWriter()
	requireDigestWrite(t, writer, []byte("x"))
	if _, _, err := writer.Seal(); err != nil {
		t.Fatalf("Seal() error = %v, want nil", err)
	}
	if _, err := writer.Write([]byte("more")); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Write(after seal) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	latchedDigest, latchedLength, err := writer.Digest()
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Digest(after a refused write) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
	if latchedDigest != (core.SHA256Digest{}) || latchedLength != (core.ByteLength{}) {
		t.Fatalf("Digest(after a refused write) = (%v, %v), want zero values alongside the refusal",
			latchedDigest, latchedLength)
	}
}

// TestDigestWriterResetErasesEveryPriorStreamAcrossSizeTransitions is the
// pooling contract under pressure: after Reset the writer is the digest of no
// bytes regardless of the stream it just sealed. The matrix pairs every stream
// size with every other, in both directions, so a residue bug that only shows
// when a large stream precedes a small one, or the reverse, cannot hide. The
// oracle is crypto/sha256 over the second stream alone; it goes red if Reset
// left prior bytes behind or failed to clear the seal that would refuse the
// second write.
func TestDigestWriterResetErasesEveryPriorStreamAcrossSizeTransitions(t *testing.T) {
	t.Parallel()

	sizes := []int{0, 1, 32, 63, 64, 65, 128, 4096, 4097}
	for _, firstLength := range sizes {
		for _, secondLength := range sizes {
			first := digestStreamContent(firstLength)
			second := digestStreamContent(secondLength)

			writer := core.NewDigestWriter()
			requireDigestWrite(t, writer, first)
			if _, _, err := writer.Seal(); err != nil {
				t.Fatalf("Seal(first %d bytes) error = %v, want nil", firstLength, err)
			}
			if err := writer.Reset(); err != nil {
				t.Fatalf("Reset(after %d bytes) error = %v, want nil", firstLength, err)
			}

			requireDigestWrite(t, writer, second)
			got, gotLength, err := writer.Seal()
			if err != nil {
				t.Fatalf("Seal(second %d bytes) error = %v, want nil", secondLength, err)
			}
			if want := core.NewSHA256Digest(sha256.Sum256(second)); got != want {
				t.Fatalf("Seal after Reset(%d bytes -> %d bytes) = %v, want the digest of only the second stream %v",
					firstLength, secondLength, got, want)
			}
			if gotLength.Uint64() != uint64(secondLength) {
				t.Fatalf("Seal after Reset(%d -> %d) length = %d, want %d",
					firstLength, secondLength, gotLength.Uint64(), secondLength)
			}
		}
	}
}

// TestDigestWriterResetFromAnOpenStreamDiscardsThePartialBytes proves Reset
// works from the open state too, not only after a seal: bytes written but never
// sealed are abandoned, and the next stream starts from empty. It goes red if
// Reset only cleared the sealed latch and left the accumulated bytes in place.
func TestDigestWriterResetFromAnOpenStreamDiscardsThePartialBytes(t *testing.T) {
	t.Parallel()

	for _, length := range []int{1, 32, 64, 65, 4097} {
		partial := digestStreamContent(length)
		writer := core.NewDigestWriter()
		requireDigestWrite(t, writer, partial)

		if err := writer.Reset(); err != nil {
			t.Fatalf("Reset(open stream, %d bytes) error = %v, want nil", length, err)
		}

		requireDigestWrite(t, writer, []byte("fresh"))
		got, _, err := writer.Seal()
		if err != nil {
			t.Fatalf("Seal(after open Reset) error = %v, want nil", err)
		}
		if want := core.NewSHA256Digest(sha256.Sum256([]byte("fresh"))); got != want {
			t.Fatalf("Seal(after open Reset over %d discarded bytes) = %v, want the digest of only the fresh bytes %v",
				length, got, want)
		}
	}
}

// TestDigestWriterResetClearsALatchedRefusal proves Reset makes a writer whose
// answer was taken usable again from empty, which is what a pool needs: the
// writer it hands back must not carry a sealed latch or a stale refusal from the
// previous stream.
func TestDigestWriterResetClearsALatchedRefusal(t *testing.T) {
	t.Parallel()

	writer := core.NewDigestWriter()
	requireDigestWrite(t, writer, []byte("first"))
	if _, _, err := writer.Seal(); err != nil {
		t.Fatalf("Seal() error = %v, want nil", err)
	}
	if _, err := writer.Write([]byte("rejected")); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Write(after seal) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}

	if err := writer.Reset(); err != nil {
		t.Fatalf("Reset(after a latched refusal) error = %v, want nil", err)
	}

	requireDigestWrite(t, writer, []byte("second"))
	got, _, err := writer.Seal()
	if err != nil {
		t.Fatalf("Seal(after Reset) error = %v, want nil", err)
	}
	if want := core.NewSHA256Digest(sha256.Sum256([]byte("second"))); got != want {
		t.Fatalf("Seal(after Reset) = %v, want the digest of only the post-Reset bytes %v", got, want)
	}
}

// TestDigestWriterResetRefusesANilReceiver keeps the zero-usable state a loud
// refusal rather than a panic into a caller reaching for a pooled writer.
func TestDigestWriterResetRefusesANilReceiver(t *testing.T) {
	t.Parallel()

	var writer *core.DigestWriter
	if err := writer.Reset(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("Reset(nil receiver) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
}
