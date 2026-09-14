package permit

import (
	"errors"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// ReportRequestMaximumBytes bounds the two signed documents and their framing.
// It bounds one request, never the caller's evidence stream or corpus.
const ReportRequestMaximumBytes = ReportDocumentMaximumBytes + controlplane.InstallationCertificateDocumentJSONMaximumBytes + 128

// ReportRequest carries a signed delta and the authority certificate nominating
// its device key. The receiving product still owns current revocation, project
// authorization, admission timing, evidence custody, and atomic accounting.
type ReportRequest struct {
	Report      SignedReport                                 `json:"report"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

// Validate checks structure and exact device nomination, not authentication.
func (r ReportRequest) Validate() error {
	if err := errors.Join(r.Report.Validate(), r.Certificate.Validate()); err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	scope, subject := r.Report.Payload.Scope, r.Certificate.Body.Subject
	if scope.Installation != subject.DeviceID || scope.Offering != subject.Offering || r.Report.Attestation.Signer != r.Certificate.Body.DeviceKey {
		return core.ErrReportBinding
	}
	return nil
}

// Verify authenticates the certificate before trusting its nominated device key.
// It performs no provider reads and grants no product permission.
func (r ReportRequest) Verify(authority controlplane.Authority) error {
	if err := r.Validate(); err != nil {
		return err
	}
	certificate, err := authority.VerifyInstallationCertificate(r.Certificate)
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	keys, err := certificate.DeviceKeys()
	if err != nil {
		return errors.Join(core.ErrReportAuthentication, err)
	}
	return r.Report.Verify(keys, r.Report.Payload.Scope)
}

func (r ReportRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire ReportRequest
	encoded, err := core.MarshalCanonicalJSONDocument(wire(r))
	if err != nil || len(encoded) > ReportRequestMaximumBytes {
		return nil, errors.Join(core.ErrReportContract, err)
	}
	return encoded, nil
}

// UnmarshalJSON preserves the receiver on all malformed or unbound input.
func (r *ReportRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrReportContract
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes, _ = core.NewByteCount(ReportRequestMaximumBytes)
	type wire ReportRequest
	decoded, err := core.DecodeStrictJSONStructure[wire](data, limits)
	if err != nil {
		return errors.Join(core.ErrReportContract, err)
	}
	got := ReportRequest(decoded)
	if err := got.Validate(); err != nil {
		return err
	}
	*r = got
	return nil
}
