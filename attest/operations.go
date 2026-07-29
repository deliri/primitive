package attest

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Sign seals one bounded typed canonical body.
func Sign[D SigningDomain[D]](request SignRequest[D]) (Envelope[D], error) {
	privateKey, signer, err := copyAndValidatePrivateKey(request.Key)
	if err != nil {
		return Envelope[D]{}, err
	}
	defer clear(privateKey[:])
	facts, err := canonicalizeBody(request.Body)
	if err != nil {
		return Envelope[D]{}, err
	}
	frame, err := newAttestationFrame(facts, signer)
	if err != nil {
		return Envelope[D]{}, err
	}
	rawSignature := ed25519.Sign(ed25519.PrivateKey(privateKey[:]), frame.bytes())
	signature, err := newSignature(rawSignature)
	clear(rawSignature)
	if err != nil {
		return Envelope[D]{}, err
	}
	publicKey, err := signer.Bytes()
	if err != nil {
		return Envelope[D]{}, contractError(err)
	}
	signatureBytes, err := signature.Bytes()
	if err != nil {
		return Envelope[D]{}, err
	}
	if !ed25519.Verify(publicKey, frame.bytes(), signatureBytes[:]) {
		return Envelope[D]{}, contractError(errors.New(postSignVerificationErrorText))
	}
	envelope := Envelope[D]{
		Domain:     facts.domain,
		Signer:     signer,
		BodyLength: facts.length,
		BodySHA256: facts.digest,
		Signature:  signature,
	}
	if err := envelope.Validate(); err != nil {
		return Envelope[D]{}, err
	}
	return envelope, nil
}

// Verify authenticates one bounded typed canonical body against trusted keys.
func Verify[D SigningDomain[D]](request VerifyRequest[D]) (Verified[D], error) {
	frame, publicKey, signature, err := prepareVerification(request)
	if err != nil {
		return Verified[D]{}, err
	}
	if !ed25519.Verify(publicKey, frame.bytes(), signature[:]) {
		return Verified[D]{}, verificationError(errors.New(envelopeSignatureErrorText))
	}
	result := Verified[D]{envelope: request.Envelope, verified: true}
	if err := result.Validate(); err != nil {
		return Verified[D]{}, err
	}
	return result, nil
}

func prepareVerification[D SigningDomain[D]](
	request VerifyRequest[D],
) (attestationFrame, ed25519.PublicKey, [ed25519.SignatureSize]byte, error) {
	var zeroSignature [ed25519.SignatureSize]byte
	if err := request.Envelope.Validate(); err != nil {
		return attestationFrame{}, nil, zeroSignature, err
	}
	if err := request.TrustedKeys.Validate(); err != nil {
		return attestationFrame{}, nil, zeroSignature, err
	}
	if !request.TrustedKeys.contains(request.Envelope.Signer) {
		return attestationFrame{}, nil, zeroSignature,
			verificationError(errors.New(envelopeSignerUntrustedErrorText))
	}
	facts, err := canonicalizeBody(request.Body)
	if err != nil {
		return attestationFrame{}, nil, zeroSignature, err
	}
	if err := validateVerifiedFacts(facts, request.Envelope); err != nil {
		return attestationFrame{}, nil, zeroSignature, err
	}
	frame, err := newAttestationFrame(facts, request.Envelope.Signer)
	if err != nil {
		return attestationFrame{}, nil, zeroSignature, verificationError(err)
	}
	publicKey, err := request.Envelope.Signer.Bytes()
	if err != nil {
		return attestationFrame{}, nil, zeroSignature, verificationError(err)
	}
	signature, err := request.Envelope.Signature.Bytes()
	if err != nil {
		return attestationFrame{}, nil, zeroSignature, verificationError(err)
	}
	return frame, publicKey, signature, nil
}

func validateVerifiedFacts[D SigningDomain[D]](
	facts canonicalFacts[D],
	envelope Envelope[D],
) error {
	if facts.domain != envelope.Domain {
		return verificationError(errors.New(envelopeDomainMismatchErrorText))
	}
	if facts.length != envelope.BodyLength {
		return verificationError(errors.New(envelopeLengthMismatchErrorText))
	}
	if facts.digest != envelope.BodySHA256 {
		return verificationError(errors.New(envelopeDigestMismatchErrorText))
	}
	return nil
}

// Verified is returned only after body, authority, and signature closure.
type Verified[D SigningDomain[D]] struct {
	envelope Envelope[D]
	verified bool
}

// Validate rejects the zero proof and validates its retained envelope.
func (v Verified[D]) Validate() error {
	if !v.verified {
		return verificationError(errors.New(verifiedProofUnsetErrorText))
	}
	if err := v.envelope.Validate(); err != nil {
		return verificationError(err)
	}
	return nil
}

// Envelope returns a copy of the verified attestation.
func (v Verified[D]) Envelope() (Envelope[D], error) {
	if err := v.Validate(); err != nil {
		return Envelope[D]{}, err
	}
	return v.envelope, nil
}

var (
	_ core.Validatable = Signature{}
	_ core.Validatable = TrustedKeysRequest{}
	_ core.Validatable = TrustedKeys{}
)
