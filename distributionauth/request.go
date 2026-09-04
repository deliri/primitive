package distributionauth

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
)

const (
	RequestDocumentJSONMaximumBytes = distribution.RequestDocumentJSONMaximumBytes +
		controlplane.InstallationCertificateDocumentJSONMaximumBytes +
		core.CredentialedRequestDocumentSyntaxBytes + core.CredentialedDocumentWhitespaceMaximumBytes
)

type UpdateRequestDocument struct {
	Request     distribution.UpdateRequestDocument           `json:"request"`
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
}

type UpdateRequestAssembly struct {
	Request     distribution.UpdateRequestDocument
	Certificate controlplane.InstallationCertificateDocument
}

type UpdateVerification struct {
	Document UpdateRequestDocument
	Server   controlplane.Authority
}

type VerifiedUpdate struct {
	document         UpdateRequestDocument
	requestProof     distribution.VerifiedUpdateRequest
	certificateProof controlplane.VerifiedInstallationCertificate
}

type UpgradeRequestDocument struct {
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	Request     distribution.UpgradeRequestDocument          `json:"request"`
}

type UpgradeRequestAssembly struct {
	Certificate controlplane.InstallationCertificateDocument
	Request     distribution.UpgradeRequestDocument
}

type UpgradeVerification struct {
	Document UpgradeRequestDocument
	Server   controlplane.Authority
}

type VerifiedUpgrade struct {
	document         UpgradeRequestDocument
	requestProof     distribution.VerifiedUpgradeRequest
	certificateProof controlplane.VerifiedInstallationCertificate
}

type (
	updateRequestDocumentWire  UpdateRequestDocument
	upgradeRequestDocumentWire UpgradeRequestDocument
)

func (d UpdateRequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Build != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this update request.
func (d UpdateRequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(
		d.Request.Payload.Build.Offering(), controlwire.RouteFamilyUpdateChecks,
	)
}

// ControlRevision projects the exact device-signed update revision.
func (d UpdateRequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed update request identity.
func (d UpdateRequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

func (UpdateRequestDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
}

func (a UpdateRequestAssembly) Validate() error { return UpdateRequestDocument(a).Validate() }

func AssembleUpdate(assembly UpdateRequestAssembly) (UpdateRequestDocument, error) {
	if err := assembly.Validate(); err != nil {
		return UpdateRequestDocument{}, err
	}
	return UpdateRequestDocument(assembly), nil
}

func (d UpdateRequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(updateRequestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *UpdateRequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed update request receiver"))
	}
	wire, err := decodeRequest[updateRequestDocumentWire](data)
	if err != nil {
		return err
	}
	candidate := UpdateRequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (v UpdateVerification) Validate() error {
	if err := errors.Join(v.Server.Validate(), v.Document.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func VerifyUpdate(verification UpdateVerification) (VerifiedUpdate, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedUpdate{}, err
	}
	certificate, err := verification.Server.VerifyInstallationCertificate(
		verification.Document.Certificate,
	)
	if err != nil {
		return VerifiedUpdate{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return VerifiedUpdate{}, contractError(err)
	}
	request, err := distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{
		Document: verification.Document.Request, TrustedKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedUpdate{}, contractError(err)
	}
	verified := VerifiedUpdate{
		document: verification.Document, requestProof: request, certificateProof: certificate,
	}
	return verified, verified.Validate()
}

func (v VerifiedUpdate) Validate() error {
	if err := errors.Join(v.document.Validate(), v.requestProof.Validate(), v.certificateProof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedUpdate) Payload() (distribution.UpdateRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return distribution.UpdateRequestPayload{}, err
	}
	return v.requestProof.Payload()
}

func (d UpgradeRequestDocument) Validate() error {
	if err := errors.Join(d.Request.Validate(), d.Certificate.Validate()); err != nil {
		return contractError(err)
	}
	if d.Request.Payload.Available.Installed != d.Certificate.Body.Build {
		return bindingError()
	}
	return nil
}

// ControlRoute projects the sole route admitted by this upgrade request.
func (d UpgradeRequestDocument) ControlRoute() (controlwire.RouteContract, error) {
	candidate := d.Request.Payload.Available.Candidate
	return controlwire.NewRouteContract(
		candidate.Offering(), controlwire.RouteFamilyUpgrades,
	)
}

// ControlRevision projects the exact device-signed upgrade revision.
func (d UpgradeRequestDocument) ControlRevision() controlwire.Revision {
	return d.Request.Payload.Revision
}

// ControlNonce projects the signed upgrade request identity.
func (d UpgradeRequestDocument) ControlNonce() controlwire.RequestNonce {
	return d.Request.Payload.Nonce
}

func (UpgradeRequestDocument) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
}

func (a UpgradeRequestAssembly) Validate() error { return UpgradeRequestDocument(a).Validate() }

func AssembleUpgrade(assembly UpgradeRequestAssembly) (UpgradeRequestDocument, error) {
	if err := assembly.Validate(); err != nil {
		return UpgradeRequestDocument{}, err
	}
	return UpgradeRequestDocument(assembly), nil
}

func (d UpgradeRequestDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(upgradeRequestDocumentWire(d))
	if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *UpgradeRequestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil credentialed upgrade request receiver"))
	}
	wire, err := decodeRequest[upgradeRequestDocumentWire](data)
	if err != nil {
		return err
	}
	candidate := UpgradeRequestDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (v UpgradeVerification) Validate() error {
	if err := errors.Join(v.Server.Validate(), v.Document.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func VerifyUpgrade(verification UpgradeVerification) (VerifiedUpgrade, error) {
	if err := verification.Validate(); err != nil {
		return VerifiedUpgrade{}, err
	}
	certificate, err := verification.Server.VerifyInstallationCertificate(
		verification.Document.Certificate,
	)
	if err != nil {
		return VerifiedUpgrade{}, contractError(err)
	}
	deviceKeys, err := certificate.DeviceKeys()
	if err != nil {
		return VerifiedUpgrade{}, contractError(err)
	}
	request, err := distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{
		Document: verification.Document.Request, TrustedKeys: deviceKeys,
	})
	if err != nil {
		return VerifiedUpgrade{}, contractError(err)
	}
	verified := VerifiedUpgrade{
		document: verification.Document, requestProof: request, certificateProof: certificate,
	}
	return verified, verified.Validate()
}

func (v VerifiedUpgrade) Validate() error {
	if err := errors.Join(v.document.Validate(), v.requestProof.Validate(), v.certificateProof.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (v VerifiedUpgrade) Payload() (distribution.UpgradeRequestPayload, error) {
	if err := v.Validate(); err != nil {
		return distribution.UpgradeRequestPayload{}, err
	}
	return v.requestProof.Payload()
}

func decodeRequest[T any](data []byte) (T, error) {
	var zero T
	maximum, err := core.NewByteCount(uint64(RequestDocumentJSONMaximumBytes))
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	wire, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return wire, nil
}

var (
	_ controlwire.RoutedJSONRequest = UpdateRequestDocument{}
	_ controlwire.RoutedJSONRequest = UpgradeRequestDocument{}
	_ core.Validatable              = UpdateRequestDocument{}
	_ core.Validatable              = UpdateRequestAssembly{}
	_ core.Validatable              = UpdateVerification{}
	_ core.Validatable              = VerifiedUpdate{}
	_ core.Validatable              = UpgradeRequestDocument{}
	_ core.Validatable              = UpgradeRequestAssembly{}
	_ core.Validatable              = UpgradeVerification{}
	_ core.Validatable              = VerifiedUpgrade{}

	_ core.ValidatedJSONMarshaler = UpdateRequestDocument{}
	_ core.ValidatedJSONMarshaler = UpgradeRequestDocument{}
	_ json.Unmarshaler            = (*UpdateRequestDocument)(nil)
	_ json.Unmarshaler            = (*UpgradeRequestDocument)(nil)
)
