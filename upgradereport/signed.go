package upgradereport

import (
	"crypto"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

type Domain uint8

const (
	DomainUnknown Domain = iota
	DomainObservation
	DomainAcknowledgment
	DomainEvidence
)

func (d Domain) Validate() error {
	if d < DomainObservation || d > DomainEvidence {
		return core.ErrReportContract
	}
	return nil
}
func (d Domain) IsValid() bool { return d.Validate() == nil }
func (Domain) OffWireEnum()    {}
func (d Domain) String() string {
	if !d.IsValid() {
		return ""
	}
	return [...]string{"", "primitive-upgrade-observation-v1", "primitive-upgrade-acknowledgment-v1", "primitive-upgrade-evidence-v1"}[d]
}
func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}
func (Domain) ParseCanonicalText(b []byte) (Domain, error) {
	for d := DomainObservation; d <= DomainEvidence; d++ {
		if string(b) == d.String() {
			return d, nil
		}
	}
	return DomainUnknown, core.ErrReportContract
}

func (Payload) AttestationDomain() Domain { return DomainObservation }
func (p Payload) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	b, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeCanonical(w, b)
}
func (Acknowledgment) AttestationDomain() Domain { return DomainAcknowledgment }
func (p Acknowledgment) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	b, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeCanonical(w, b)
}
func writeCanonical(w io.Writer, b []byte) error {
	if w == nil {
		return core.ErrReportContract
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

type Request struct {
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	Payload     Payload                                      `json:"payload"`
	Attestation attest.Envelope[Domain]                      `json:"attestation"`
}

func (r Request) Validate() error {
	if err := errors.Join(r.Payload.Validate(), r.Attestation.Validate(), r.Certificate.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	evidence := r.Payload.Evidence.Payload.Header
	if r.Attestation.Domain != DomainObservation || r.Attestation.Signer != r.Certificate.Body.DeviceKey || r.Certificate.Body.Build != r.Payload.Installed || evidence.Principal != r.Certificate.Body.Account {
		return core.ErrReportBinding
	}
	return nil
}
func Sign(p Payload, c controlplane.InstallationCertificateDocument, key crypto.Signer) (Request, error) {
	a, err := attest.Sign(attest.SignRequest[Domain]{Body: p, Signer: key})
	if err != nil {
		return Request{}, errors.Join(core.ErrReportAuthentication, err)
	}
	r := Request{Payload: p, Certificate: c, Attestation: a}
	if err := r.Validate(); err != nil {
		return Request{}, err
	}
	return r, nil
}

// Verify authenticates the installation, observation, and independent evidence
// receipt. Current revocation, attempt admission and policy remain server-owned.
func (r Request) Verify(authority controlplane.Authority, keys attest.TrustedKeys) error {
	if err := r.Validate(); err != nil {
		return err
	}
	c, err := authority.VerifyInstallationCertificate(r.Certificate)
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	device, err := c.DeviceKeys()
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	if _, err := attest.Verify(attest.VerifyRequest[Domain]{Body: r.Payload, Envelope: r.Attestation, TrustedKeys: device}); err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	e := r.Payload.Evidence
	if _, err := receipt.VerifyEvidence(receipt.VerifyEvidenceRequest{Document: e, TrustedKeys: keys, Expected: receipt.EvidenceExpectation{Principal: r.Certificate.Body.Account, Offering: r.Payload.Installed.Offering(), Body: e.Payload.Body}}); err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (r Request) ControlRoute() (controlwire.RouteContract, error) {
	if err := r.Validate(); err != nil {
		return controlwire.RouteContract{}, err
	}
	return controlwire.NewRouteContract(r.Payload.Installed.Offering(), controlwire.RouteFamilyUpgradeReports)
}
func (r Request) ControlRevision() controlwire.Revision  { return r.Payload.Revision }
func (r Request) ControlNonce() controlwire.RequestNonce { return r.Payload.Attempt }
func (r Request) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire Request
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r *Request) UnmarshalJSON(b []byte) error {
	if r == nil {
		return core.ErrReportContract
	}
	type wire Request
	v, err := core.DecodeStrictJSONStructure[wire](b, core.ExtensibleJSONLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := Request(v)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}
func (r Request) Digest() (core.SHA256Digest, error) {
	b, err := r.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(b), nil
}

type Response struct {
	Payload     Acknowledgment          `json:"payload"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

func (r Response) Validate() error {
	if err := errors.Join(r.Payload.Validate(), r.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if r.Attestation.Domain != DomainAcknowledgment {
		return core.ErrReportBinding
	}
	return nil
}
func SignAcknowledgment(p Acknowledgment, key crypto.Signer) (Response, error) {
	a, err := attest.Sign(attest.SignRequest[Domain]{Body: p, Signer: key})
	if err != nil {
		return Response{}, errors.Join(core.ErrReportAuthentication, err)
	}
	r := Response{Payload: p, Attestation: a}
	if err := r.Validate(); err != nil {
		return Response{}, err
	}
	return r, nil
}
func (r Response) Matches(request Request) error {
	if err := r.Validate(); err != nil {
		return err
	}
	digest, err := request.Digest()
	if err != nil {
		return err
	}
	if r.Payload.Attempt != request.Payload.Attempt || r.Payload.RequestDigest != digest {
		return core.ErrReportBinding
	}
	return nil
}
func (r Response) Verify(keys attest.TrustedKeys, request Request) error {
	if err := r.Matches(request); err != nil {
		return err
	}
	if _, err := attest.Verify(attest.VerifyRequest[Domain]{Body: r.Payload, Envelope: r.Attestation, TrustedKeys: keys}); err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (r Response) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire Response
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r *Response) UnmarshalJSON(b []byte) error {
	if r == nil {
		return core.ErrReportContract
	}
	type wire Response
	v, err := core.DecodeStrictJSONStructure[wire](b, core.ExtensibleJSONLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := Response(v)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}
