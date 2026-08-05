package attest

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type canonicalFacts[D SigningDomain[D]] struct {
	domain D
	length core.ByteCount
	token  domainToken
	digest core.SHA256Digest
}

// canonicalDigestWriter adds attest's policy to Core's streaming digest: a
// canonical body has a maximum size, may not be empty, and is written exactly
// once. The hashing itself, and the refusal to hash a short write, belong to
// Core and are not repeated here.
type canonicalDigestWriter struct {
	digest *core.DigestWriter
	err    error
	count  int
	closed bool
}

func newCanonicalDigestWriter() *canonicalDigestWriter {
	return &canonicalDigestWriter{digest: core.NewDigestWriter()}
}

func (w *canonicalDigestWriter) Write(data []byte) (int, error) {
	if w.closed {
		return 0, contractError(errors.New(writerClosedErrorText))
	}
	if w.err != nil {
		return 0, w.err
	}
	if len(data) > CanonicalBodyMaximumBytes-w.count {
		w.err = contractError(errors.New(bodyLimitErrorText))
		return 0, w.err
	}
	written, err := w.digest.Write(data)
	if err != nil {
		w.err = contractError(err)
		return written, w.err
	}
	w.count += written
	return written, nil
}

func (w *canonicalDigestWriter) close(callbackErr error) (core.ByteCount, core.SHA256Digest, error) {
	w.closed = true
	if w.err != nil {
		return core.ByteCount{}, core.SHA256Digest{}, w.err
	}
	if callbackErr != nil {
		return core.ByteCount{}, core.SHA256Digest{}, contractError(callbackErr)
	}
	if w.count == 0 {
		return core.ByteCount{}, core.SHA256Digest{}, contractError(errors.New(bodyEmptyErrorText))
	}
	checkedCount, err := core.CheckedUint32FromInt(w.count)
	if err != nil {
		return core.ByteCount{}, core.SHA256Digest{}, contractError(err)
	}
	length, err := core.NewByteCount(uint64(checkedCount))
	if err != nil {
		return core.ByteCount{}, core.SHA256Digest{}, contractError(err)
	}
	digest, _, err := w.digest.Seal()
	if err != nil {
		return core.ByteCount{}, core.SHA256Digest{}, contractError(err)
	}
	// Core's byte length is discarded on purpose: attest's canonical count is a
	// ByteCount bounded by CanonicalBodyMaximumBytes, which is a narrower fact
	// about the same bytes and is checked above.
	return length, digest, nil
}

func validateBodyShape[D SigningDomain[D]](body CanonicalBody[D]) error {
	if body == nil {
		return contractError(errors.New(bodyMissingErrorText))
	}
	_, err := guardedCall(func() (struct{}, error) {
		if err := body.Validate(); err != nil {
			return struct{}{}, err
		}
		domain := body.AttestationDomain()
		if _, err := canonicalDomain(domain); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return contractError(err)
	}
	return nil
}

func canonicalizeBody[D SigningDomain[D]](body CanonicalBody[D]) (canonicalFacts[D], error) {
	var zero canonicalFacts[D]
	if body == nil {
		return zero, contractError(errors.New(bodyMissingErrorText))
	}
	prepared, err := guardedCall(func() (canonicalFacts[D], error) {
		if err := body.Validate(); err != nil {
			return zero, err
		}
		domain := body.AttestationDomain()
		token, tokenErr := canonicalDomain(domain)
		if tokenErr != nil {
			return zero, tokenErr
		}
		return canonicalFacts[D]{domain: domain, token: token}, nil
	})
	if err != nil {
		return zero, contractError(err)
	}
	writer := newCanonicalDigestWriter()
	_, writeErr := guardedCall(func() (struct{}, error) {
		return struct{}{}, body.WriteCanonical(writer)
	})
	length, digest, err := writer.close(writeErr)
	if err != nil {
		return zero, err
	}
	prepared.length = length
	prepared.digest = digest
	return prepared, nil
}

func validateBodyLength(length core.ByteCount) error {
	value, err := length.Uint64()
	if err != nil {
		return contractError(err)
	}
	if value > CanonicalBodyMaximumBytes {
		return contractError(errors.New(envelopeBodyLengthErrorText))
	}
	return nil
}
