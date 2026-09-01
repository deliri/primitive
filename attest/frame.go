package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	attestationFrameGeneration = "primitive-attestation-2026"
	attestationFrameSeparator  = byte(0)
	attestationFrameMaximum    = len(attestationFrameGeneration) + 1 + 2 +
		SigningDomainMaximumBytes + ed25519.PublicKeySize + 8 + sha256.Size
)

type attestationFrame struct {
	value [attestationFrameMaximum]byte
	count int
}

func newAttestationFrame[D SigningDomain[D]](
	facts canonicalFacts[D],
	signer core.Ed25519PublicKey,
) (attestationFrame, error) {
	publicKey, err := signer.Bytes()
	if err != nil {
		return attestationFrame{}, contractError(err)
	}
	digest, err := facts.digest.Bytes()
	if err != nil {
		return attestationFrame{}, contractError(err)
	}
	length, err := facts.length.Uint64()
	if err != nil {
		return attestationFrame{}, contractError(err)
	}
	domain := facts.token.bytes()
	domainLength, err := checkedUint16FromInt(len(domain))
	if err != nil {
		return attestationFrame{}, err
	}
	var frame attestationFrame
	// An empty domain means the token failed its own validation, and an
	// over-extent frame would silently reallocate off the fixed array and then
	// panic in bytes. Reject both so framing stays the declared fixed layout.
	required := len(attestationFrameGeneration) + 1 + 2 + len(domain) +
		len(publicKey) + 8 + len(digest)
	if len(domain) == 0 || required > len(frame.value) {
		return attestationFrame{}, contractError(errors.New(frameExtentErrorText))
	}
	value := frame.value[:0]
	value = append(value, attestationFrameGeneration...)
	value = append(value, attestationFrameSeparator)
	value = binary.BigEndian.AppendUint16(value, domainLength)
	value = append(value, domain...)
	value = append(value, publicKey...)
	value = binary.BigEndian.AppendUint64(value, length)
	value = append(value, digest[:]...)
	frame.count = len(value)
	return frame, nil
}

func (f attestationFrame) bytes() []byte {
	return f.value[:f.count]
}

func checkedUint16FromInt(value int) (uint16, error) {
	converted, err := core.CheckedUint16FromInt(value)
	if err != nil {
		return 0, contractError(errors.New(domainCanonicalErrorText))
	}
	return converted, nil
}
