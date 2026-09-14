package permit

import (
	"crypto"
	"errors"
	"io"
	"slices"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// ReportDomain separates reports, initial permissions and accepted acknowledgments.
type ReportDomain uint8

const (
	ReportDomainUnknown ReportDomain = iota
	ReportDomainPayload
	ReportDomainPermission
	ReportDomainAcknowledgment
	ReportDomainAuthorization
)

func (d ReportDomain) Validate() error {
	if d < ReportDomainPayload || d > ReportDomainAuthorization {
		return core.ErrReportContract
	}
	return nil
}
func (ReportDomain) OffWireEnum()    {}
func (d ReportDomain) IsValid() bool { return d.Validate() == nil }
func (d ReportDomain) String() string {
	if !d.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{
		"primitive-project-report-v1",
		"primitive-project-permission-v1",
		"primitive-project-report-ack-v1",
		"primitive-project-report-authorization-v1",
	}[d-ReportDomainPayload]
}

func (d ReportDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}
func (ReportDomain) ParseCanonicalText(text []byte) (ReportDomain, error) {
	for d := ReportDomainPayload; d <= ReportDomainAuthorization; d++ {
		if string(text) == d.String() {
			return d, nil
		}
	}
	return ReportDomainUnknown, core.ErrReportContract
}

func (ReportPayload) AttestationDomain() ReportDomain { return ReportDomainPayload }
func (p ReportPayload) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeReportBytes(w, data)
}

type SignedReport struct {
	Payload     ReportPayload                 `json:"payload"`
	Attestation attest.Envelope[ReportDomain] `json:"attestation"`
}

func (d SignedReport) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if d.Attestation.Domain != ReportDomainPayload {
		return core.ErrReportBinding
	}
	return nil
}
func SignReportPayload(p ReportPayload, signer crypto.Signer) (SignedReport, error) {
	a, err := attest.Sign(attest.SignRequest[ReportDomain]{Body: p, Signer: signer})
	if err != nil {
		return SignedReport{}, errors.Join(core.ErrReportAuthentication, err)
	}
	return SignedReport{Payload: p, Attestation: a}, nil
}
func (d SignedReport) Verify(keys attest.TrustedKeys, scope ReportScope) error {
	if err := errors.Join(d.Validate(), scope.Validate()); err != nil {
		return err
	}
	if d.Payload.Scope != scope {
		return core.ErrReportBinding
	}
	_, err := attest.Verify(attest.VerifyRequest[ReportDomain]{Body: d.Payload, Envelope: d.Attestation, TrustedKeys: keys})
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (d SignedReport) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire SignedReport
	return core.MarshalCanonicalJSONDocument(wire(d))
}
func (d *SignedReport) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrReportContract
	}
	type wire SignedReport
	value, err := core.DecodeStrictJSONStructure[wire](data, reportLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := SignedReport(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*d = got
	return nil
}

func (ProjectPermission) AttestationDomain() ReportDomain { return ReportDomainPermission }
func (p ProjectPermission) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeReportBytes(w, data)
}

type SignedProjectPermission struct {
	Payload     ProjectPermission             `json:"payload"`
	Attestation attest.Envelope[ReportDomain] `json:"attestation"`
}

func (d SignedProjectPermission) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if d.Attestation.Domain != ReportDomainPermission {
		return core.ErrReportBinding
	}
	return nil
}
func SignProjectPermission(p ProjectPermission, signer crypto.Signer) (SignedProjectPermission, error) {
	a, err := attest.Sign(attest.SignRequest[ReportDomain]{Body: p, Signer: signer})
	if err != nil {
		return SignedProjectPermission{}, errors.Join(core.ErrReportAuthentication, err)
	}
	return SignedProjectPermission{Payload: p, Attestation: a}, nil
}
func (d SignedProjectPermission) Verify(keys attest.TrustedKeys, scope ReportScope) error {
	if err := errors.Join(d.Validate(), scope.Validate()); err != nil {
		return err
	}
	if d.Payload.Scope != scope {
		return core.ErrReportBinding
	}
	_, err := attest.Verify(attest.VerifyRequest[ReportDomain]{Body: d.Payload, Envelope: d.Attestation, TrustedKeys: keys})
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (d SignedProjectPermission) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire SignedProjectPermission
	return core.MarshalCanonicalJSONDocument(wire(d))
}
func (d *SignedProjectPermission) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrReportContract
	}
	type wire SignedProjectPermission
	value, err := core.DecodeStrictJSONStructure[wire](data, reportLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := SignedProjectPermission(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*d = got
	return nil
}

func (ReportAcknowledgment) AttestationDomain() ReportDomain { return ReportDomainAcknowledgment }
func (p ReportAcknowledgment) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeReportBytes(w, data)
}

type SignedReportAcknowledgment struct {
	Payload     ReportAcknowledgment          `json:"payload"`
	Attestation attest.Envelope[ReportDomain] `json:"attestation"`
}

func (d SignedReportAcknowledgment) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if d.Attestation.Domain != ReportDomainAcknowledgment {
		return core.ErrReportBinding
	}
	return nil
}
func SignReportAcknowledgment(p ReportAcknowledgment, signer crypto.Signer) (SignedReportAcknowledgment, error) {
	a, err := attest.Sign(attest.SignRequest[ReportDomain]{Body: p, Signer: signer})
	if err != nil {
		return SignedReportAcknowledgment{}, errors.Join(core.ErrReportAuthentication, err)
	}
	return SignedReportAcknowledgment{Payload: p, Attestation: a}, nil
}
func (d SignedReportAcknowledgment) Verify(keys attest.TrustedKeys, scope ReportScope) error {
	if err := errors.Join(d.Validate(), scope.Validate()); err != nil {
		return err
	}
	if d.Payload.Scope != scope {
		return core.ErrReportBinding
	}
	_, err := attest.Verify(attest.VerifyRequest[ReportDomain]{Body: d.Payload, Envelope: d.Attestation, TrustedKeys: keys})
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (d SignedReportAcknowledgment) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire SignedReportAcknowledgment
	return core.MarshalCanonicalJSONDocument(wire(d))
}
func (d *SignedReportAcknowledgment) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrReportContract
	}
	type wire SignedReportAcknowledgment
	value, err := core.DecodeStrictJSONStructure[wire](data, reportLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := SignedReportAcknowledgment(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*d = got
	return nil
}

func (d SignedReport) Digest() (core.SHA256Digest, error) {
	data, err := d.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(data), nil
}

// Clone owns all mutable counter slices before retention or asynchronous use.
func (d SignedReport) Clone() SignedReport {
	d.Payload.Window.Units = slices.Clone(d.Payload.Window.Units)
	d.Payload.Window.Outcomes = slices.Clone(d.Payload.Window.Outcomes)
	d.Payload.Window.Measurements = slices.Clone(d.Payload.Window.Measurements)
	return d
}

// Matches pins the complete pending identity; authentication is a separate gate.
func (a ReportAcknowledgment) Matches(r SignedReport) error {
	if err := errors.Join(a.Validate(), r.Validate()); err != nil {
		return err
	}
	digest, err := r.Digest()
	if err != nil {
		return err
	}
	if a.Scope != r.Payload.Scope || a.Sequence != r.Payload.Sequence || a.ReportDigest != digest {
		return core.ErrReportBinding
	}
	return nil
}
func (ReportAuthorization) AttestationDomain() ReportDomain { return ReportDomainAuthorization }
func (p ReportAuthorization) WriteCanonical(w io.Writer) error {
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := core.MarshalCanonicalJSONDocument(p)
	if err != nil {
		return err
	}
	return writeReportBytes(w, data)
}

type SignedReportAuthorization struct {
	Payload     ReportAuthorization           `json:"payload"`
	Attestation attest.Envelope[ReportDomain] `json:"attestation"`
}

func (d SignedReportAuthorization) Validate() error {
	if err := errors.Join(d.Payload.Validate(), d.Attestation.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	if d.Attestation.Domain != ReportDomainAuthorization {
		return core.ErrReportBinding
	}
	return nil
}
func SignReportAuthorization(p ReportAuthorization, signer crypto.Signer) (SignedReportAuthorization, error) {
	a, err := attest.Sign(attest.SignRequest[ReportDomain]{Body: p, Signer: signer})
	if err != nil {
		return SignedReportAuthorization{}, errors.Join(core.ErrReportAuthentication, err)
	}
	return SignedReportAuthorization{Payload: p, Attestation: a}, nil
}
func (d SignedReportAuthorization) Verify(keys attest.TrustedKeys, scope ReportScope) error {
	if err := errors.Join(d.Validate(), scope.Validate()); err != nil {
		return err
	}
	if d.Payload.Scope != scope {
		return core.ErrReportBinding
	}
	_, err := attest.Verify(attest.VerifyRequest[ReportDomain]{Body: d.Payload, Envelope: d.Attestation, TrustedKeys: keys})
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return nil
}
func (d SignedReportAuthorization) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire SignedReportAuthorization
	return core.MarshalCanonicalJSONDocument(wire(d))
}
func (d *SignedReportAuthorization) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrReportContract
	}
	type wire SignedReportAuthorization
	value, err := core.DecodeStrictJSONStructure[wire](data, reportLimits())
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := SignedReportAuthorization(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*d = got
	return nil
}

func (r ReportPermissionResponse) Verify(keys attest.TrustedKeys, scope ReportScope) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := r.Authorization.Verify(keys, scope); err != nil {
		return err
	}
	if r.InitialPermission != nil {
		return r.InitialPermission.Verify(keys, scope)
	}
	return r.Acknowledgment.Verify(keys, scope)
}
