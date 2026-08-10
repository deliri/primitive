package retrieval

import (
	"crypto"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	RequestPayloadJSONMaximumBytes       = 32 << 10
	RequestDocumentJSONMaximumBytes      = 64 << 10
	requestCommitmentDomain              = "primitive/retrieval/request-commitment/v1"
	requestCommitmentSeparator      byte = 0
)

// Selection is the exact all-or-one manifest object selection.
type Selection struct {
	Sequence chit.EntrySequence        `json:"sequence,omitempty"`
	Kind     core.CatalogSelectionKind `json:"kind"`
}

func All() Selection { return Selection{Kind: core.CatalogSelectionAll} }

func Specific(sequence chit.EntrySequence) (Selection, error) {
	candidate := Selection{Kind: core.CatalogSelectionSpecific, Sequence: sequence}
	return candidate, candidate.Validate()
}

func (s Selection) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return contractError(err)
	}
	switch s.Kind {
	case core.CatalogSelectionAll:
		if s.Sequence != (chit.EntrySequence{}) {
			return contractError(errors.New("all retrieval carries a sequence"))
		}
	case core.CatalogSelectionSpecific:
		if err := s.Sequence.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("retrieval selection escaped its domain"))
	}
	return nil
}

// MarshalJSON emits only the member owned by the selected tagged-union arm.
func (s Selection) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	if s.Kind == core.CatalogSelectionAll {
		return core.MarshalCanonicalJSONDocument(struct {
			Kind core.CatalogSelectionKind `json:"kind"`
		}{Kind: s.Kind})
	}
	return core.MarshalCanonicalJSONDocument(struct {
		Sequence chit.EntrySequence        `json:"sequence"`
		Kind     core.CatalogSelectionKind `json:"kind"`
	}{Sequence: s.Sequence, Kind: s.Kind})
}

// RequestPayload is the exact chit/object request signed by one installation.
type RequestPayload struct {
	Selection Selection                `json:"selection"`
	Build     core.BuildIdentity       `json:"build"`
	Nonce     controlwire.RequestNonce `json:"request_nonce"`
	Chit      chit.ChitID              `json:"chit_id"`
	Revision  controlwire.Revision     `json:"revision"`
}

func (p RequestPayload) Validate() error {
	if err := errors.Join(p.Build.Validate(), p.Chit.Validate(), p.Selection.Validate(), p.Revision.Validate(), p.Nonce.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (RequestPayload) AttestationDomain() SigningDomain { return SigningDomainRequestV1 }

func (p RequestPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
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

func (p RequestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire RequestPayload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > RequestPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *RequestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil retrieval request payload receiver"))
	}
	type wire RequestPayload
	decoded, err := decodeStrict[wire](data, RequestPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := RequestPayload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

type RequestDocument struct {
	Payload     RequestPayload                 `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

func (d RequestDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainRequestV1 {
		return bindingError(errors.New("retrieval request signing domain differs"))
	}
	return nil
}

func (d RequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire RequestDocument
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *RequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil retrieval request document receiver"))
	}
	type wire RequestDocument
	decoded, err := decodeStrict[wire](data, RequestDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := RequestDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type RequestIssuance struct {
	Signer  crypto.Signer
	Payload RequestPayload
}

func (i RequestIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueRequest(issuance RequestIssuance) (RequestDocument, error) {
	if err := issuance.Validate(); err != nil {
		return RequestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: issuance.Payload, Signer: issuance.Signer})
	if err != nil {
		return RequestDocument{}, contractError(err)
	}
	document := RequestDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

type RequestCommitment struct{ digest core.SHA256Digest }

func CommitRequest(payload RequestPayload) (RequestCommitment, error) {
	encoded, err := payload.MarshalJSON()
	if err != nil {
		return RequestCommitment{}, err
	}
	framed := make([]byte, 0, len(requestCommitmentDomain)+1+len(encoded))
	framed = append(framed, requestCommitmentDomain...)
	framed = append(framed, requestCommitmentSeparator)
	framed = append(framed, encoded...)
	return newRequestCommitment(core.SHA256Of(framed))
}

func newRequestCommitment(digest core.SHA256Digest) (RequestCommitment, error) {
	candidate := RequestCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return RequestCommitment{}, err
	}
	return candidate, nil
}

func (c RequestCommitment) Validate() error {
	if err := c.digest.Validate(); err != nil {
		return contractError(errors.New("retrieval request commitment is invalid"), err)
	}
	return nil
}

func (c RequestCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return c.digest.MarshalJSON()
}

func (c *RequestCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil retrieval request commitment receiver"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	candidate, err := newRequestCommitment(digest)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

func decodeStrict[T any](data []byte, maximum uint64) (T, error) {
	var zero T
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	decoded, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return decoded, nil
}

var (
	_ core.Validatable                    = Selection{}
	_ core.Validatable                    = RequestPayload{}
	_ core.Validatable                    = RequestDocument{}
	_ core.Validatable                    = RequestIssuance{}
	_ core.Validatable                    = RequestCommitment{}
	_ core.ValidatedJSONMarshaler         = RequestPayload{}
	_ core.ValidatedJSONMarshaler         = RequestDocument{}
	_ core.ValidatedJSONMarshaler         = RequestCommitment{}
	_ core.ValidatedJSONMarshaler         = Selection{}
	_ attest.CanonicalBody[SigningDomain] = RequestPayload{}
)
