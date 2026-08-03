package attest

import (
	"crypto"
	"encoding"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// SigningDomainMaximumBytes is the maximum canonical domain-text extent.
	SigningDomainMaximumBytes = 64
	// CanonicalBodyMaximumBytes is the maximum canonical body extent.
	CanonicalBodyMaximumBytes = 1 << 20
	// TrustedKeyMaximumCount is the maximum caller-selected trust-set size.
	TrustedKeyMaximumCount = 16
	// EnvelopeCanonicalJSONMaximumBytes is the exact maximum canonical envelope
	// extent. MarshalJSON never emits more.
	EnvelopeCanonicalJSONMaximumBytes = 405
	// EnvelopeJSONMaximumBytes is the maximum accepted envelope document extent.
	// It is the canonical extent plus a bounded insignificant-whitespace
	// allowance, so an envelope that a pretty-printer has indented still
	// decodes and renormalizes to its canonical projection.
	EnvelopeJSONMaximumBytes = EnvelopeCanonicalJSONMaximumBytes +
		envelopeJSONWhitespaceAllowanceBytes
	// envelopeJSONWhitespaceAllowanceBytes is the insignificant whitespace a
	// decoded envelope may carry above its canonical extent. It admits deep
	// indentation while keeping the decode input bounded.
	envelopeJSONWhitespaceAllowanceBytes = 1 << 10
)

// SigningDomain is implemented by a protocol owner's closed domain enum.
// ParseCanonicalText must reconstruct the value whose canonical text is the
// supplied text.
type SigningDomain[D any] interface {
	comparable
	core.Validatable
	encoding.TextMarshaler
	ParseCanonicalText([]byte) (D, error)
}

// CanonicalBody writes one typed, byte-exact canonical representation.
type CanonicalBody[D SigningDomain[D]] interface {
	core.Validatable
	AttestationDomain() D
	WriteCanonical(io.Writer) error
}

// TrustedKeysRequest carries caller-selected public keys into fixed storage.
type TrustedKeysRequest struct {
	Keys []core.Ed25519PublicKey
}

// SignRequest carries one canonical body and one standard-library Ed25519
// signing capability. An ed25519.PrivateKey satisfies crypto.Signer directly;
// remote KMS and HSM implementations can supply the same interface without
// exposing private key bytes.
type SignRequest[D SigningDomain[D]] struct {
	Body   CanonicalBody[D]
	Signer crypto.Signer
}

// VerifyRequest carries one canonical body, envelope, and trust set.
type VerifyRequest[D SigningDomain[D]] struct {
	Body        CanonicalBody[D]
	Envelope    Envelope[D]
	TrustedKeys TrustedKeys
}

// Validate checks trust-set input without retaining caller storage.
func (r TrustedKeysRequest) Validate() error {
	return validateTrustedKeyInput(r.Keys)
}

// Validate checks the signing request shape without signing or writing the
// canonical body.
func (r SignRequest[D]) Validate() error {
	capability, err := newSigningCapability(r.Signer)
	if err != nil {
		return err
	}
	capability.close()
	return validateBodyShape(r.Body)
}

// Validate checks verification request structure without claiming that the
// signer is trusted or that the signature matches.
func (r VerifyRequest[D]) Validate() error {
	if err := r.Envelope.Validate(); err != nil {
		return err
	}
	if err := r.TrustedKeys.Validate(); err != nil {
		return err
	}
	return validateBodyShape(r.Body)
}
