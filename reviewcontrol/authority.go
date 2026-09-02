package reviewcontrol

import (
	"bytes"
	"encoding"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const HumanAuthoritySigningDomainV1Token = "primitive-review-control-human-authority-2026-1"

type HumanAuthoritySigningDomain uint8

const (
	HumanAuthoritySigningDomainUnknown HumanAuthoritySigningDomain = iota
	HumanAuthoritySigningDomainV1
)

func (d HumanAuthoritySigningDomain) Validate() error {
	if d != HumanAuthoritySigningDomainV1 {
		return contractError()
	}
	return nil
}

func (d HumanAuthoritySigningDomain) IsValid() bool { return d.Validate() == nil }
func (d HumanAuthoritySigningDomain) String() string {
	if !d.IsValid() {
		return ""
	}
	return HumanAuthoritySigningDomainV1Token
}
func (d HumanAuthoritySigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}
func (d *HumanAuthoritySigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError()
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := HumanAuthoritySigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return jsonError(err)
	}
	*d = parsed
	return nil
}

func (d HumanAuthoritySigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(HumanAuthoritySigningDomainV1Token), nil
}

func (HumanAuthoritySigningDomain) ParseCanonicalText(text []byte) (HumanAuthoritySigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes || !bytes.Equal(text, []byte(HumanAuthoritySigningDomainV1Token)) {
		return HumanAuthoritySigningDomainUnknown, contractError()
	}
	return HumanAuthoritySigningDomainV1, nil
}

type HumanAuthorityClaim struct {
	Principal PrincipalIdentity `json:"principal"`
	Authority AuthorityIdentity `json:"authority"`
	Kind      AuthorityKind     `json:"kind"`
}

func (c HumanAuthorityClaim) Validate() error {
	return validateContract(c.Principal.Validate(), c.Authority.Validate(), c.Kind.Validate())
}

func (HumanAuthorityClaim) AttestationDomain() HumanAuthoritySigningDomain {
	return HumanAuthoritySigningDomainV1
}

func (c HumanAuthorityClaim) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("review control human authority canonical destination is nil"))
	}
	encoded, err := c.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return contractError(err)
	}
	if written != len(encoded) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}

type humanAuthorityClaimWire HumanAuthorityClaim

func (c HumanAuthorityClaim) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONDocument(humanAuthorityClaimWire(c))
}

func (c *HumanAuthorityClaim) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError()
	}
	wire, err := core.DecodeStrictJSONStructure[humanAuthorityClaimWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := HumanAuthorityClaim(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

type VerifiedHumanAuthority struct {
	claim HumanAuthorityClaim
	proof attest.Verified[HumanAuthoritySigningDomain]
}

func NewVerifiedHumanAuthority(claim HumanAuthorityClaim, proof attest.Verified[HumanAuthoritySigningDomain]) (VerifiedHumanAuthority, error) {
	candidate := VerifiedHumanAuthority{claim: claim, proof: proof}
	if err := candidate.Validate(); err != nil {
		return VerifiedHumanAuthority{}, err
	}
	return candidate, nil
}

func (a VerifiedHumanAuthority) Validate() error {
	if err := errors.Join(a.claim.Validate(), a.proof.Validate()); err != nil {
		return errors.Join(core.ErrReviewControlUnauthorizedAuthority, contractError(err))
	}
	if a.claim.Kind != AuthorityHuman {
		return errors.Join(core.ErrReviewControlNonHumanAuthority, contractError())
	}
	envelope, err := a.proof.Envelope()
	if err != nil {
		return errors.Join(core.ErrReviewControlUnauthorizedAuthority, contractError(err))
	}
	encoded, err := a.claim.MarshalJSON()
	if err != nil {
		return err
	}
	length, err := core.NewByteCount(uint64(len(encoded)))
	if err != nil {
		return contractError(err)
	}
	if envelope.Domain != HumanAuthoritySigningDomainV1 || envelope.BodyLength != length || envelope.BodySHA256 != core.SHA256Of(encoded) {
		return errors.Join(core.ErrReviewControlUnauthorizedAuthority, contractError())
	}
	return nil
}

func (a VerifiedHumanAuthority) Principal() PrincipalIdentity { return a.claim.Principal }
func (a VerifiedHumanAuthority) Authority() AuthorityIdentity { return a.claim.Authority }

type AuthorityReference struct {
	Principal PrincipalIdentity `json:"principal"`
	Authority AuthorityIdentity `json:"authority"`
}

func (r AuthorityReference) Validate() error {
	return validateContract(r.Principal.Validate(), r.Authority.Validate())
}

func (a VerifiedHumanAuthority) Reference() (AuthorityReference, error) {
	if err := a.Validate(); err != nil {
		return AuthorityReference{}, err
	}
	return AuthorityReference{Principal: a.claim.Principal, Authority: a.claim.Authority}, nil
}

var (
	_ attest.CanonicalBody[HumanAuthoritySigningDomain] = HumanAuthorityClaim{}
	_ encoding.TextMarshaler                            = HumanAuthoritySigningDomain(0)
)
