package timeproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

// AuthorityEvidence seals the exact request and the response digest and extent.
// The caller owns response bytes; Verify establishes the authoritative conclusion.
type AuthorityEvidence struct {
	request        Request
	responseDigest core.SHA256Digest
	responseBytes  uint64
}

type authorityEvidenceInput struct {
	Response []byte
	Request  Request
}

type authorityEvidenceWire struct {
	Request        Request           `json:"request"`
	ResponseDigest core.SHA256Digest `json:"response_sha256"`
	ResponseBytes  uint64            `json:"response_bytes"`
}

type authorityEvidenceWireJSON authorityEvidenceWire

func (w authorityEvidenceWire) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(authorityEvidenceWireJSON(w))
}

func newAuthorityEvidence(input authorityEvidenceInput) (AuthorityEvidence, error) {
	evidence := AuthorityEvidence{
		responseDigest: core.NewSHA256Digest(sha256.Sum256(input.Response)),
		responseBytes:  uint64(len(input.Response)),
		request:        input.Request,
	}
	if err := evidence.Validate(); err != nil {
		return AuthorityEvidence{}, err
	}
	return evidence, nil
}

// Validate checks bounded structural custody. It deliberately does not claim
// cryptographic validity; callers use Verify for that conclusion.
func (e AuthorityEvidence) Validate() error {
	if err := e.request.Validate(); err != nil {
		return contractError(err)
	}
	if e.responseBytes == 0 || e.responseDigest.Validate() != nil {
		return contractError(nil)
	}
	return nil
}

// Request returns the exact request bound to the response.
func (e AuthorityEvidence) Request() Request { return e.request }

// Authority returns the evidence authority.
func (e AuthorityEvidence) Authority() Authority {
	return e.request.Authority()
}

// Digest returns the request digest.
func (e AuthorityEvidence) Digest() core.SHA256Digest {
	return e.request.Digest()
}

// Nonce returns the request nonce.
func (e AuthorityEvidence) Nonce() Nonce { return e.request.Nonce() }

// ResponseDigest returns SHA-256 of the exact verified response bytes.
func (e AuthorityEvidence) ResponseDigest() core.SHA256Digest { return e.responseDigest }

// ResponseSize returns the exact response extent, independently of any declaration.
func (e AuthorityEvidence) ResponseSize() uint64 { return e.responseBytes }

// MarshalJSON emits canonical proof custody.
func (e AuthorityEvidence) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return core.EncodeValidatedJSON(authorityEvidenceWire{
		Request:        e.request,
		ResponseDigest: e.responseDigest,
		ResponseBytes:  e.responseBytes,
	}, core.DefaultStrictJSONLimits())
}

// UnmarshalJSON reconstructs bounded proof custody without asserting validity.
func (e *AuthorityEvidence) UnmarshalJSON(data []byte) error {
	if e == nil || len(data) == 0 {
		return errorsJSON()
	}
	wire, err := core.DecodeStrictJSON[authorityEvidenceWire](
		bytes.NewReader(data), core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return errorsJSON()
	}
	parsed, err := evidenceFromWire(wire)
	if err != nil {
		return errorsJSON(err)
	}
	canonical, err := parsed.MarshalJSON()
	if err != nil || !bytes.Equal(data, canonical) {
		return errorsJSON()
	}
	*e = parsed
	return nil
}

func (w authorityEvidenceWire) Validate() error {
	if err := w.Request.Validate(); err != nil {
		return err
	}
	if w.ResponseBytes == 0 || w.ResponseDigest.Validate() != nil {
		return contractError(nil)
	}
	return nil
}

func evidenceFromWire(wire authorityEvidenceWire) (AuthorityEvidence, error) {
	evidence := AuthorityEvidence{request: wire.Request, responseDigest: wire.ResponseDigest, responseBytes: wire.ResponseBytes}
	if err := evidence.Validate(); err != nil {
		return AuthorityEvidence{}, err
	}
	return evidence, nil
}

func decodeEvidenceBase64(value string, maximum int) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) == 0 || len(raw) > maximum ||
		base64.StdEncoding.EncodeToString(raw) != value {
		return nil, errorsJSON()
	}
	return raw, nil
}
