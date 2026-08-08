package core

import (
	"crypto/sha256"
	"errors"
	"hash"
	"io"
)

const (
	digestWriterNilReceiverDiagnostic = "nil digest writer receiver"
	digestWriterSealedDiagnostic      = "digest writer is sealed"
)

// DigestWriter accumulates streamed bytes into one SHA-256 digest and the exact
// count of the bytes that produced it.
//
// Hashing a stream is not a product decision, and everything that moves bytes
// has to do it: to content-address an object, to bind an artifact to its
// manifest, to seal a canonical body. Written separately each time it arrives
// as a private struct around crypto/sha256, each with its own count type and
// its own idea of when the digest becomes final. Core owns the mechanic so the
// digest and the count are always two facts about exactly the same bytes.
//
// Bounds are deliberately absent. A maximum belongs to whoever knows what is
// being hashed; one baked in here would be wrong for one caller and silently
// generous for another. A caller that needs a ceiling enforces it above and
// keeps the reason with it. A caller that needs the same stream elsewhere
// composes io.MultiWriter, which is why this is an ordinary io.Writer and not
// a private accumulator with a bespoke feed method.
type DigestWriter struct {
	digest hash.Hash
	err    error
	count  uint64
	sealed bool
}

// NewDigestWriter returns a writer positioned over an empty stream. The digest
// of no bytes is a real answer, so an empty stream seals rather than failing;
// whether emptiness is acceptable is the caller's rule, not this one's.
func NewDigestWriter() *DigestWriter {
	return &DigestWriter{digest: sha256.New()}
}

// Write accumulates data into the running digest.
//
// The hash is not asked whether it succeeded, because hash.Hash documents that
// Write never returns an error and always consumes every byte, and this writer
// always holds the sha256 implementation it constructed. Checking anyway would
// add a branch no test could ever fail, and an unfailable branch is a claim
// nobody is keeping. The one refusal that is real — writing to a sealed writer
// — is latched, so a caller cannot resume a stream whose answer was taken.
func (w *DigestWriter) Write(data []byte) (int, error) {
	if w == nil {
		return 0, digestWriterError(digestWriterNilReceiverDiagnostic)
	}
	if w.err != nil {
		return 0, w.err
	}
	if w.sealed {
		w.err = digestWriterError(digestWriterSealedDiagnostic)
		return 0, w.err
	}
	written, _ := w.digest.Write(data)
	w.count += uint64(written)
	return written, nil
}

// Seal ends the stream and returns the digest and byte count of exactly the
// bytes written.
//
// Sealing is one-way. A later write is refused rather than producing a second
// digest that disagrees with the first about the same writer, so a caller
// holding the pair can be certain no byte arrived after the answer was taken.
// Sealing twice returns the same answer, because asking again is not a
// mutation.
func (w *DigestWriter) Seal() (SHA256Digest, ByteLength, error) {
	if w == nil {
		return SHA256Digest{}, ByteLength{}, digestWriterError(digestWriterNilReceiverDiagnostic)
	}
	if w.err != nil {
		return SHA256Digest{}, ByteLength{}, w.err
	}
	length, err := NewByteLength(w.count)
	if err != nil {
		return SHA256Digest{}, ByteLength{}, err
	}
	var sum [sha256.Size]byte
	w.digest.Sum(sum[:0])
	w.sealed = true
	return NewSHA256Digest(sum), length, nil
}

// SHA256Of returns the digest of one complete in-memory buffer.
//
// It is the whole-buffer companion to DigestWriter. A caller that already
// holds every byte does not need a streaming writer to hash them, and threading
// one buffer through io.Writer plumbing to reach the same answer is ceremony,
// not safety. The streaming path stays for bytes that arrive over time or must
// be teed; this is for bytes that are already here. A whole-buffer hash cannot
// fail: len(data) is an int, so the byte count is always a legal length.
func SHA256Of(data []byte) SHA256Digest {
	return NewSHA256Digest(sha256.Sum256(data))
}

func digestWriterError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}

var _ io.Writer = (*DigestWriter)(nil)
