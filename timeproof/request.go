package timeproof

import (
	"bytes"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

const requestJSONMaximumBytes = 2048

// PrepareRequest identifies the digest to timestamp.
type PrepareRequest struct {
	Digest    core.SHA256Digest
	Authority Authority
}

// Validate rejects an unset digest.
func (r PrepareRequest) Validate() error {
	if err := r.Digest.Validate(); err != nil {
		return contractError(err)
	}
	return r.Authority.Validate()
}

// Request is the exact bounded RFC 3161 request callers send to FreeTSA.
type Request struct {
	body      []byte
	digest    core.SHA256Digest
	nonce     Nonce
	authority Authority
}

type requestWire struct {
	Body      string            `json:"body_base64"`
	Digest    core.SHA256Digest `json:"digest_sha256"`
	Nonce     Nonce             `json:"nonce"`
	Authority Authority         `json:"authority"`
}

type requestWireJSON requestWire

func (w requestWire) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(requestWireJSON(w))
}

// Prepare creates one fresh-nonce request without performing I/O.
func Prepare(input PrepareRequest) (Request, error) {
	if err := input.Validate(); err != nil {
		return Request{}, err
	}
	nonce, err := generateNonce()
	if err != nil {
		return Request{}, err
	}
	return newRequest(input.Digest, nonce, input.Authority)
}

func newRequest(
	digest core.SHA256Digest,
	nonce Nonce,
	authority Authority,
) (Request, error) {
	body, err := buildRequest(digest, nonce)
	if err != nil {
		return Request{}, err
	}
	request := Request{
		body: body, digest: digest, nonce: nonce, authority: authority,
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

// Validate checks the complete request and its canonical DER binding.
func (r Request) Validate() error {
	if err := r.authority.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.digest.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.nonce.Validate(); err != nil {
		return contractError(err)
	}
	canonical, err := buildRequest(r.digest, r.nonce)
	if err != nil || !bytes.Equal(r.body, canonical) {
		return contractError(err)
	}
	return nil
}

// Bytes returns an independent copy of the exact DER request.
func (r Request) Bytes() []byte { return append([]byte(nil), r.body...) }

// Digest returns the requested message imprint.
func (r Request) Digest() core.SHA256Digest { return r.digest }

// Nonce returns the exact request nonce.
func (r Request) Nonce() Nonce { return r.nonce }

// Authority returns the authority this request is bound to.
func (r Request) Authority() Authority { return r.authority }

// MarshalJSON emits canonical request custody.
func (r Request) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return core.EncodeValidatedJSON(requestWire{
		Body: base64.StdEncoding.EncodeToString(r.body), Digest: r.digest,
		Nonce: r.nonce, Authority: r.authority,
	}, core.DefaultStrictJSONLimits())
}

// UnmarshalJSON reconstructs one canonical request without mutation on error.
func (r *Request) UnmarshalJSON(data []byte) error {
	if r == nil || len(data) == 0 || len(data) > requestJSONMaximumBytes {
		return errorsJSON()
	}
	wire, err := core.DecodeStrictJSON[requestWire](
		data,
		core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return errorsJSON()
	}
	body, err := decodeEvidenceBase64(wire.Body, RequestMaximumBytes)
	if err != nil {
		return err
	}
	parsed := Request{
		body: body, digest: wire.Digest, nonce: wire.Nonce,
		authority: wire.Authority,
	}
	if err := parsed.Validate(); err != nil {
		return err
	}
	canonical, err := parsed.MarshalJSON()
	if err != nil || !bytes.Equal(data, canonical) {
		return errorsJSON()
	}
	*r = parsed
	return nil
}

func (w requestWire) Validate() error {
	if w.Body == "" {
		return contractError(nil)
	}
	if err := w.Digest.Validate(); err != nil {
		return err
	}
	if err := w.Nonce.Validate(); err != nil {
		return err
	}
	return w.Authority.Validate()
}

// messageImprint is protocol order, not memory-layout order.
// encoding/asn1 maps fields positionally; fieldalignment must not reorder it.
type messageImprint struct {
	HashAlgorithm pkix.AlgorithmIdentifier
	HashedMessage []byte
}

type timestampRequestFields struct {
	version []byte
	imprint []byte
	nonce   []byte
	certReq []byte
}

func buildRequest(digest core.SHA256Digest, nonce Nonce) ([]byte, error) {
	raw, err := digest.Bytes()
	if err != nil {
		return nil, contractError(err)
	}
	if err := nonce.Validate(); err != nil {
		return nil, err
	}
	imprint := messageImprint{
		HashAlgorithm: pkix.AlgorithmIdentifier{
			Algorithm:  oidSHA256,
			Parameters: asn1.RawValue{Tag: asn1.TagNull},
		},
		HashedMessage: raw[:],
	}
	fields, err := marshalTimestampRequestFields(imprint, nonce)
	if err != nil {
		return nil, err
	}
	body := make(
		[]byte,
		0,
		len(fields.version)+len(fields.imprint)+
			len(fields.nonce)+len(fields.certReq),
	)
	body = append(body, fields.version...)
	body = append(body, fields.imprint...)
	body = append(body, fields.nonce...)
	body = append(body, fields.certReq...)
	encoded := derTagged(byte(asn1.TagSequence)|derConstructed, body)
	if len(encoded) == 0 || len(encoded) > RequestMaximumBytes {
		return nil, contractError(core.ErrExchangeBodyLimit)
	}
	return encoded, nil
}

func marshalTimestampRequestFields(
	imprint messageImprint,
	nonce Nonce,
) (timestampRequestFields, error) {
	version, err := asn1.Marshal(1)
	if err != nil {
		return timestampRequestFields{}, contractError(err)
	}
	imprintDER, err := asn1.Marshal(imprint)
	if err != nil {
		return timestampRequestFields{}, contractError(err)
	}
	nonceDER, err := asn1.Marshal(nonce.integer())
	if err != nil {
		return timestampRequestFields{}, contractError(err)
	}
	certReq, err := asn1.Marshal(true)
	if err != nil {
		return timestampRequestFields{}, contractError(err)
	}
	return timestampRequestFields{
		version: version, imprint: imprintDER,
		nonce: nonceDER, certReq: certReq,
	}, nil
}
