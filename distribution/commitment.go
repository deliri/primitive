package distribution

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// RequestCommitment is the non-secret domain-separated closure of one exact
// signed-request payload. Domain is carried with the digest so a publication,
// update, and upgrade request cannot be substituted for one another.
type RequestCommitment struct {
	domain SigningDomain
	digest core.SHA256Digest
}

type requestCommitmentWire struct {
	Domain *SigningDomain     `json:"domain"`
	Digest *core.SHA256Digest `json:"sha256"`
}

// RequestPayload is the closed set of request bodies this protocol can commit.
// Values carry their bounds and canonical writer; arbitrary callback bodies
// cannot introduce another protocol or an unbounded writer at this boundary.
type RequestPayload interface {
	PublicationRequestPayload | UpdateRequestPayload | UpgradeRequestPayload
	attest.CanonicalBody[SigningDomain]
}

// CommitRequest streams one canonical payload into its protocol-specific
// commitment frame.
func CommitRequest[T RequestPayload](body T) (RequestCommitment, error) {
	domain := body.AttestationDomain()
	if err := validateRequestDomain(domain); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	digest := core.NewDigestWriter()
	if _, err := digest.Write([]byte(domain.String())); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if _, err := digest.Write([]byte{documentCommitmentFrameSeparator}); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if err := body.WriteCanonical(digest); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	value, _, err := digest.Seal()
	if err != nil {
		return RequestCommitment{}, contractError(err)
	}
	return newRequestCommitment(domain, value)
}

func newRequestCommitment(domain SigningDomain, digest core.SHA256Digest) (RequestCommitment, error) {
	candidate := RequestCommitment{domain: domain, digest: digest}
	if err := candidate.Validate(); err != nil {
		return RequestCommitment{}, err
	}
	return candidate, nil
}

// Validate rejects an unset domain or all-zero digest.
func (c RequestCommitment) Validate() error {
	if err := errors.Join(validateRequestDomain(c.domain), c.digest.Validate()); err != nil {
		return contractError(err)
	}
	raw, err := c.digest.Bytes()
	if err != nil {
		return contractError(err)
	}
	if raw == ([sha256.Size]byte{}) {
		return contractError(errors.New("distribution request commitment is all zero"))
	}
	return nil
}

func (c RequestCommitment) validateDomain(expected SigningDomain) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.domain != expected {
		return bindingError(errors.New("distribution request commitment has the wrong domain"))
	}
	return nil
}

// Domain returns the signed-request namespace closed by this commitment.
func (c RequestCommitment) Domain() SigningDomain { return c.domain }

// MarshalJSON emits the canonical domain and digest structure.
func (c RequestCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	domain, digest := c.domain, c.digest
	return core.MarshalCanonicalJSONDocument(requestCommitmentWire{Domain: &domain, Digest: &digest})
}

// UnmarshalJSON accepts only the exact closed structure without mutating on
// refusal.
func (c *RequestCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("distribution request commitment receiver is nil"))
	}
	wire, err := decodeStrict[requestCommitmentWire](data, RequestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	if wire.Domain == nil || wire.Digest == nil {
		return jsonError(errors.New("distribution request commitment field is missing"))
	}
	candidate, err := newRequestCommitment(*wire.Domain, *wire.Digest)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

var (
	_ core.Validatable            = RequestCommitment{}
	_ core.ValidatedJSONMarshaler = RequestCommitment{}
	_ json.Unmarshaler            = (*RequestCommitment)(nil)
)

func validateRequestDomain(domain SigningDomain) error {
	switch domain {
	case SigningDomainPublicationRequestV1, SigningDomainUpdateRequestV1, SigningDomainUpgradeRequestV1:
		return domain.Validate()
	default:
		return contractError(errors.New("request commitment domain is not a request"))
	}
}
