package controlwire

import (
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// ReplayIdentityCommitmentDomain separates canonical control requests from
	// every other SHA-256 use in Primitive.
	ReplayIdentityCommitmentDomain = "primitive.controlwire.replay-request.v1"
	// ReplayIdentityFrameSeparator makes the domain/body frame injective.
	ReplayIdentityFrameSeparator byte = 0
	// ReplayIdentityJSONMaximumBytes bounds one persisted replay identity.
	ReplayIdentityJSONMaximumBytes = 4 << 10
)

// RequestCommitment is the domain-separated closure of one canonical control
// request. It contains no request data and is safe to persist.
type RequestCommitment struct {
	digest core.SHA256Digest
}

// Validate rejects an unset commitment.
func (c RequestCommitment) Validate() error {
	if err := c.digest.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// MarshalJSON emits the canonical digest token.
func (c RequestCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := c.digest.MarshalJSON()
	if err != nil {
		return nil, jsonError(contractError(err))
	}
	return encoded, nil
}

// UnmarshalJSON admits one canonical persisted commitment without mutating the
// receiver on rejection.
func (c *RequestCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(contractError())
	}
	var digest core.SHA256Digest
	if err := digest.UnmarshalJSON(data); err != nil {
		return jsonError(contractError(err))
	}
	candidate := RequestCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

// ReplayIdentity is the exact authority-side identity of one admitted control
// request. The nonce selects a replay slot; the canonical commitment prevents
// a different request from occupying that slot.
type ReplayIdentity struct {
	commitment RequestCommitment
	nonce      RequestNonce
	offering   core.Offering
	family     RouteFamily
	revision   Revision
}

type replayIdentityWire struct {
	Offering   core.Offering     `json:"offering"`
	Family     RouteFamily       `json:"route_family"`
	Revision   Revision          `json:"revision"`
	Nonce      RequestNonce      `json:"request_nonce"`
	Commitment RequestCommitment `json:"request_commitment"`
}

// CommitReplayIdentity derives one identity from the exact validated request
// the authority admitted. Callers cannot repeat route, revision, or nonce beside
// the document, so those facts cannot drift from its commitment.
func CommitReplayIdentity(request RoutedJSONRequest) (ReplayIdentity, error) {
	if request == nil {
		return ReplayIdentity{}, contractError()
	}
	if err := request.Validate(); err != nil {
		return ReplayIdentity{}, contractError(err)
	}
	route, err := request.ControlRoute()
	if err != nil {
		return ReplayIdentity{}, contractError(err)
	}
	canonical, err := request.MarshalJSON()
	if err != nil {
		return ReplayIdentity{}, contractError(err)
	}
	commitment, err := commitCanonicalRequest(canonical)
	if err != nil {
		return ReplayIdentity{}, err
	}
	identity := ReplayIdentity{
		commitment: commitment,
		nonce:      request.ControlNonce(),
		offering:   route.Offering(),
		family:     route.Family(),
		revision:   request.ControlRevision(),
	}
	if err := identity.Validate(); err != nil {
		return ReplayIdentity{}, err
	}
	return identity, nil
}

func commitCanonicalRequest(canonical []byte) (RequestCommitment, error) {
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(ReplayIdentityCommitmentDomain)); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if _, err := writer.Write([]byte{ReplayIdentityFrameSeparator}); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	if _, err := writer.Write(canonical); err != nil {
		return RequestCommitment{}, contractError(err)
	}
	digest, _, err := writer.Seal()
	if err != nil {
		return RequestCommitment{}, contractError(err)
	}
	commitment := RequestCommitment{digest: digest}
	if err := commitment.Validate(); err != nil {
		return RequestCommitment{}, err
	}
	return commitment, nil
}

// Validate closes every persisted identity fact.
func (i ReplayIdentity) Validate() error {
	if err := i.commitment.Validate(); err != nil {
		return err
	}
	if err := i.nonce.Validate(); err != nil {
		return contractError(err)
	}
	if err := i.offering.Validate(); err != nil {
		return contractError(err)
	}
	if err := i.family.Validate(); err != nil {
		return contractError(err)
	}
	if err := i.revision.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// MarshalJSON emits one bounded canonical persisted identity.
func (i ReplayIdentity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(i.wire())
	if err != nil || len(encoded) > ReplayIdentityJSONMaximumBytes {
		return nil, jsonError(contractError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes one persisted identity and preserves the
// receiver on every rejection.
func (i *ReplayIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(contractError())
	}
	maximum, err := core.NewByteCount(ReplayIdentityJSONMaximumBytes)
	if err != nil {
		return jsonError(contractError(err))
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	wire, err := core.DecodeStrictJSONStructure[replayIdentityWire](data, limits)
	if err != nil {
		return jsonError(contractError(err))
	}
	candidate := replayIdentityFromWire(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i ReplayIdentity) wire() replayIdentityWire {
	return replayIdentityWire{
		Offering: i.offering, Family: i.family, Revision: i.revision,
		Nonce: i.nonce, Commitment: i.commitment,
	}
}

func replayIdentityFromWire(wire replayIdentityWire) ReplayIdentity {
	return ReplayIdentity{
		commitment: wire.Commitment, nonce: wire.Nonce, offering: wire.Offering,
		family: wire.Family, revision: wire.Revision,
	}
}

// ReplayDisposition is the non-error result of checking one authority replay
// slot. A conflicting reuse is an error, not a result a caller can overlook.
type ReplayDisposition uint8

const (
	ReplayDispositionUnknown ReplayDisposition = iota
	// ReplayDispositionFresh means no prior request occupies the nonce.
	ReplayDispositionFresh
	// ReplayDispositionExact means the prior and incoming requests are identical.
	ReplayDispositionExact
)

func replayDispositionNames() [ReplayDispositionExact + 1]string {
	return [...]string{
		ReplayDispositionUnknown: "",
		ReplayDispositionFresh:   "fresh",
		ReplayDispositionExact:   "exact",
	}
}

// Validate rejects the zero and future dispositions.
func (d ReplayDisposition) Validate() error {
	if d < ReplayDispositionFresh || d > ReplayDispositionExact || replayDispositionNames()[d] == "" {
		return contractError()
	}
	return nil
}

// IsValid reports whether d is an admitted disposition.
func (d ReplayDisposition) IsValid() bool { return d.Validate() == nil }

// String returns the diagnostic name or empty text for an invalid value.
func (d ReplayDisposition) String() string {
	if !d.IsValid() {
		return ""
	}
	return replayDispositionNames()[d]
}

// OffWireEnum declares that disposition is an in-process authority decision.
func (ReplayDisposition) OffWireEnum() {}

// ReplayCheck supplies the record found under Incoming's nonce, if one exists.
type ReplayCheck struct {
	Existing *ReplayIdentity
	Incoming ReplayIdentity
}

// Validate closes both sides and refuses a caller that looked up the wrong
// nonce slot.
func (c ReplayCheck) Validate() error {
	if err := c.Incoming.Validate(); err != nil {
		return err
	}
	if c.Existing == nil {
		return nil
	}
	if err := c.Existing.Validate(); err != nil {
		return err
	}
	if c.Existing.nonce != c.Incoming.nonce {
		return contractError(core.ErrControlWireNonce)
	}
	return nil
}

// CheckReplay separates a fresh slot, an exact replay, and conflicting reuse.
// It reveals no conflicting field and never returns the stored identity.
func CheckReplay(check ReplayCheck) (ReplayDisposition, error) {
	if err := check.Validate(); err != nil {
		return ReplayDispositionUnknown, err
	}
	if check.Existing == nil {
		return ReplayDispositionFresh, nil
	}
	if *check.Existing != check.Incoming {
		return ReplayDispositionUnknown, replayConflictError()
	}
	return ReplayDispositionExact, nil
}

var (
	_ core.Validatable            = RequestCommitment{}
	_ core.Validatable            = ReplayIdentity{}
	_ core.Validatable            = ReplayDispositionUnknown
	_ core.Validatable            = ReplayCheck{}
	_ core.ValidatedJSONMarshaler = RequestCommitment{}
	_ core.ValidatedJSONMarshaler = ReplayIdentity{}
	_ core.OffWireEnum            = ReplayDispositionUnknown
	_ json.Unmarshaler            = (*RequestCommitment)(nil)
	_ json.Unmarshaler            = (*ReplayIdentity)(nil)
)
