package timeproof

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

const authorityEvidenceJSONMaximumBytes = 192 * 1024

// AuthorityEvidence owns one exact request and the exact timestamp response
// accepted for it. Verify establishes the authoritative conclusion.
type AuthorityEvidence struct {
	response []byte
	request  Request
}

func (e AuthorityEvidence) isZero() bool {
	return len(e.response) == 0 && len(e.request.body) == 0
}

type authorityEvidenceInput struct {
	Response []byte
	Request  Request
}

type authorityEvidenceWire struct {
	Response string  `json:"response_base64"`
	Request  Request `json:"request"`
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
		response: append([]byte(nil), input.Response...),
		request:  input.Request,
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
	if len(e.response) == 0 || len(e.response) > ResponseMaximumBytes {
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

// ResponseBytes returns an independent copy of the exact TimeStampResp.
func (e AuthorityEvidence) ResponseBytes() []byte {
	return append([]byte(nil), e.response...)
}

// MarshalJSON emits canonical proof custody.
func (e AuthorityEvidence) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return core.EncodeValidatedJSON(authorityEvidenceWire{
		Request:  e.request,
		Response: base64.StdEncoding.EncodeToString(e.response),
	}, core.DefaultStrictJSONLimits())
}

// UnmarshalJSON reconstructs bounded proof custody without asserting validity.
func (e *AuthorityEvidence) UnmarshalJSON(data []byte) error {
	if e == nil || len(data) == 0 || len(data) > authorityEvidenceJSONMaximumBytes {
		return errorsJSON()
	}
	wire, err := core.DecodeStrictJSON[authorityEvidenceWire](
		data, core.DefaultStrictJSONLimits(),
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
	if w.Response == "" {
		return contractError(nil)
	}
	return nil
}

func evidenceFromWire(wire authorityEvidenceWire) (AuthorityEvidence, error) {
	response, err := decodeEvidenceBase64(wire.Response, ResponseMaximumBytes)
	if err != nil {
		return AuthorityEvidence{}, err
	}
	return newAuthorityEvidence(authorityEvidenceInput{
		Request: wire.Request, Response: response,
	})
}

func decodeEvidenceBase64(value string, maximum int) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) == 0 || len(raw) > maximum ||
		base64.StdEncoding.EncodeToString(raw) != value {
		return nil, errorsJSON()
	}
	return raw, nil
}
