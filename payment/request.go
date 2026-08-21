package payment

import (
	"crypto"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	QueryPayloadJSONMaximumBytes       = 32 << 10
	QueryDocumentJSONMaximumBytes      = 64 << 10
	queryCommitmentDomain              = "primitive/payment/query-commitment/v1"
	queryCommitmentSeparator      byte = 0
)

// QueryPayload is one exact payment catalog query signed by an installed device.
type QueryPayload struct {
	Build    core.BuildIdentity       `json:"build"`
	Query    Query                    `json:"query"`
	Nonce    controlwire.RequestNonce `json:"request_nonce"`
	Revision controlwire.Revision     `json:"revision"`
}

func (p QueryPayload) Validate() error {
	if err := errors.Join(p.Query.Validate(), p.Build.Validate(), p.Nonce.Validate(), p.Revision.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (QueryPayload) AttestationDomain() SigningDomain { return SigningDomainQueryV1 }

func (p QueryPayload) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return contractError(errors.New("payment query canonical destination is nil"))
	}
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

func (p QueryPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire QueryPayload
	encoded, err := core.MarshalCanonicalJSONDocument(wire(p))
	if err != nil || len(encoded) > QueryPayloadJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (p *QueryPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("nil payment query payload receiver"))
	}
	type wire QueryPayload
	decoded, err := decodeStrict[wire](data, QueryPayloadJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := QueryPayload(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// QueryDocument carries one device signature over one exact payment query.
type QueryDocument struct {
	Payload     QueryPayload                   `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

// QueryCommitment is the non-secret domain-separated closure of one exact
// device-signed payment query payload.
type QueryCommitment struct{ digest core.SHA256Digest }

// CommitQuery closes the exact selection, position, scope, build, nonce, and
// revision without retaining the encoded query.
func CommitQuery(payload QueryPayload) (QueryCommitment, error) {
	if err := payload.Validate(); err != nil {
		return QueryCommitment{}, err
	}
	digest := sha256.New()
	if _, err := io.WriteString(digest, queryCommitmentDomain); err != nil {
		return QueryCommitment{}, contractError(err)
	}
	if _, err := digest.Write([]byte{queryCommitmentSeparator}); err != nil {
		return QueryCommitment{}, contractError(err)
	}
	if err := payload.WriteCanonical(digest); err != nil {
		return QueryCommitment{}, err
	}
	var raw [sha256.Size]byte
	digest.Sum(raw[:0])
	return newQueryCommitment(core.NewSHA256Digest(raw))
}

func newQueryCommitment(digest core.SHA256Digest) (QueryCommitment, error) {
	candidate := QueryCommitment{digest: digest}
	if err := candidate.Validate(); err != nil {
		return QueryCommitment{}, err
	}
	return candidate, nil
}

func (c QueryCommitment) Validate() error {
	raw, err := c.digest.Bytes()
	if err != nil {
		return contractError(err)
	}
	for _, value := range raw {
		if value != 0 {
			return nil
		}
	}
	return contractError(errors.New("payment query commitment is all zero"))
}

func (c QueryCommitment) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return c.digest.MarshalJSON()
}

func (c *QueryCommitment) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil payment query commitment receiver"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	candidate, err := newQueryCommitment(digest)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

func (d QueryDocument) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return contractError(err)
	}
	if d.Attestation.Domain != SigningDomainQueryV1 {
		return verificationError(errors.New("payment query signing domain differs"))
	}
	return nil
}

func (d QueryDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	type wire QueryDocument
	encoded, err := core.MarshalCanonicalJSONDocument(wire(d))
	if err != nil || len(encoded) > QueryDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *QueryDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil payment query document receiver"))
	}
	type wire QueryDocument
	decoded, err := decodeStrict[wire](data, QueryDocumentJSONMaximumBytes)
	if err != nil {
		return err
	}
	candidate := QueryDocument(decoded)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type QueryIssuance struct {
	Signer  crypto.Signer
	Payload QueryPayload
}

func (i QueryIssuance) Validate() error {
	if err := (attest.SignRequest[SigningDomain]{Body: i.Payload, Signer: i.Signer}).Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func IssueQuery(issuance QueryIssuance) (QueryDocument, error) {
	if err := issuance.Validate(); err != nil {
		return QueryDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: issuance.Payload, Signer: issuance.Signer})
	if err != nil {
		return QueryDocument{}, contractError(err)
	}
	document := QueryDocument{Payload: issuance.Payload, Attestation: envelope}
	return document, document.Validate()
}

type QueryVerification struct {
	Document    QueryDocument
	TrustedKeys attest.TrustedKeys
}

func (v QueryVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type VerifiedQuery struct {
	document QueryDocument
	proof    attest.Verified[SigningDomain]
}

func VerifyQuery(verification QueryVerification) (VerifiedQuery, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedQuery{}, err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: verification.Document.Payload, Envelope: verification.Document.Attestation,
		TrustedKeys: verification.TrustedKeys,
	})
	if err != nil {
		return VerifiedQuery{}, verificationError(err)
	}
	verified := VerifiedQuery{document: verification.Document, proof: proof}
	return verified, verified.Validate()
}

func (v VerifiedQuery) Validate() error {
	if err := errors.Join(v.document.Validate(), v.proof.Validate()); err != nil {
		return verificationError(err)
	}
	return nil
}

func (v VerifiedQuery) Payload() (QueryPayload, error) {
	if err := v.Validate(); err != nil {
		return QueryPayload{}, err
	}
	return v.document.Payload, nil
}

var (
	_ core.Validatable = QueryPayload{}
	_ core.Validatable = QueryCommitment{}
	_ core.Validatable = QueryDocument{}
	_ core.Validatable = QueryIssuance{}
	_ core.Validatable = QueryVerification{}
	_ core.Validatable = VerifiedQuery{}

	_ core.ValidatedJSONMarshaler         = QueryPayload{}
	_ core.ValidatedJSONMarshaler         = QueryCommitment{}
	_ core.ValidatedJSONMarshaler         = QueryDocument{}
	_ json.Unmarshaler                    = (*QueryPayload)(nil)
	_ json.Unmarshaler                    = (*QueryCommitment)(nil)
	_ json.Unmarshaler                    = (*QueryDocument)(nil)
	_ attest.CanonicalBody[SigningDomain] = QueryPayload{}
)
