package controlplane

import (
	"crypto"
	json "encoding/json/v2"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// RegistrationRequestJSONMaximumBytes bounds an accepted request.
	RegistrationRequestJSONMaximumBytes = 16 << 10
	// InstallationCertificateBodyJSONMaximumBytes bounds a certificate body.
	InstallationCertificateBodyJSONMaximumBytes = 8 << 10
	// InstallationCertificateDocumentJSONMaximumBytes bounds a signed certificate.
	InstallationCertificateDocumentJSONMaximumBytes = 16 << 10
	// RegistrationPayloadJSONMaximumBytes bounds a response payload.
	RegistrationPayloadJSONMaximumBytes = 48 << 10
	// RegistrationDocumentJSONMaximumBytes bounds a complete signed response.
	RegistrationDocumentJSONMaximumBytes = 64 << 10
)

// RegistrationRequest is the complete first-contact body one installation
// sends.
//
// It carries no customer-controlled name, path, source, corpus, output, or run
// data. What an installation is entitled to is decided from its identity, not
// from anything it says about its work.
// Field order here is the machine's, not the protocol's. The token is the only
// pointer-bearing member, so it leads and the garbage collector scans eight
// bytes instead of the whole struct.
//
// Declaration order and protocol order are decoupled on purpose. Decoding
// matches members by name and does not care about order, and encoding states
// the protocol's order explicitly in MarshalJSON rather than inheriting it from
// whatever the layout happens to be. A struct that had to be declared in
// protocol order would force a choice between the bytes the authority defined
// and the layout the machine wants, and neither is negotiable.
type RegistrationRequest struct {
	Token        controlwire.RegistrationToken `json:"registration_token"`
	Build        core.BuildIdentity            `json:"build"`
	RequestNonce controlwire.RequestNonce      `json:"request_nonce"`
	DeviceKey    core.Ed25519PublicKey         `json:"device_public_key"`
	Installation lease.DeviceID                `json:"installation"`
	Revision     controlwire.Revision          `json:"revision"`
}

// registrationRequestWire exists only to give the strict decoder a type without
// the UnmarshalJSON method, which would otherwise recurse. It is a defined type
// of the request, so it cannot drift from it.
type registrationRequestWire RegistrationRequest

// Validate closes every ingress fact and binds the installation to its device
// key.
//
// The binding is the rule that matters. An installation identity is derived
// from a device public key, so a request naming any other identity is either a
// client defect or an attempt to enrol as somebody else. Deriving it again here
// means a mismatch is refused locally, with a reason, instead of being refused
// by the authority with nothing the caller can act on.
func (r RegistrationRequest) Validate() error {
	if err := r.validateFacts(); err != nil {
		return err
	}
	derived, err := lease.DeviceIDForPublicKey(r.DeviceKey)
	if err != nil {
		return installationBindingError(err)
	}
	if derived != r.Installation {
		return installationBindingError()
	}
	return nil
}

// ControlRoute projects the only route this document may address.
func (r RegistrationRequest) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(r.Build.Offering(), controlwire.RouteFamilyRegistrations)
}

// ControlRevision projects the exact revision carried by this request.
func (r RegistrationRequest) ControlRevision() controlwire.Revision { return r.Revision }

// ControlNonce projects the request identity already carried on the wire.
func (r RegistrationRequest) ControlNonce() controlwire.RequestNonce { return r.RequestNonce }

func (RegistrationRequest) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(RegistrationRequestJSONMaximumBytes)
}

func (r RegistrationRequest) validateFacts() error {
	if err := r.Token.Validate(); err != nil {
		return registrationError(err)
	}
	if err := r.RequestNonce.Validate(); err != nil {
		return registrationError(err)
	}
	if err := r.Installation.Validate(); err != nil {
		return registrationError(err)
	}
	if err := r.DeviceKey.Validate(); err != nil {
		return registrationError(err)
	}
	if err := r.Build.Validate(); err != nil {
		return registrationError(err)
	}
	if err := r.Revision.Validate(); err != nil {
		return registrationError(err)
	}
	return nil
}

// MarshalJSON emits one bounded canonical request in the protocol's member
// order, which is stated here and nowhere else.
func (r RegistrationRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	object := attest.BeginCanonicalObject(nil)
	object.Value(protocolMemberBuild, r.Build)
	object.Value(protocolMemberRevision, r.Revision)
	object.Value(protocolMemberRequestNonce, r.RequestNonce)
	object.Value(protocolMemberDevicePublicKey, r.DeviceKey)
	object.Value(protocolMemberRegistrationToken, r.Token)
	object.Value(protocolMemberInstallation, r.Installation)
	encoded, err := object.End()
	if err != nil || len(encoded) > RegistrationRequestJSONMaximumBytes {
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (r *RegistrationRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(RegistrationRequestJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[registrationRequestWire](data, limits)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := RegistrationRequest(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(registrationError(err))
	}
	*r = candidate
	return nil
}

// InstallationCertificateBody is the signed statement that one device key
// belongs to one entitlement under one account.
type InstallationCertificateBody struct {
	Subject   lease.Subject           `json:"subject"`
	Build     core.BuildIdentity      `json:"build"`
	IssuedAt  temporal.Instant        `json:"issued_at"`
	DeviceKey core.Ed25519PublicKey   `json:"device_public_key"`
	Account   receipt.AccountIdentity `json:"account"`
	Revision  controlwire.Revision    `json:"revision"`
}

// InstallationCertificateDocument is the certificate body with its signature.
type InstallationCertificateDocument struct {
	Body        InstallationCertificateBody    `json:"body"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

type (
	installationCertificateBodyWire     InstallationCertificateBody
	installationCertificateDocumentWire InstallationCertificateDocument
)

// Validate closes the certificate's facts and re-derives both bindings it
// asserts.
func (b InstallationCertificateBody) Validate() error {
	if err := b.validateFacts(); err != nil {
		return err
	}
	if err := b.validateOfferingBinding(); err != nil {
		return err
	}
	return b.validateDeviceBinding()
}

func (b InstallationCertificateBody) validateFacts() error {
	if err := b.IssuedAt.Validate(); err != nil {
		return registrationError(err)
	}
	if err := b.Build.Validate(); err != nil {
		return registrationError(err)
	}
	if err := b.Revision.Validate(); err != nil {
		return registrationError(err)
	}
	if err := b.Subject.Validate(); err != nil {
		return registrationError(err)
	}
	if err := b.DeviceKey.Validate(); err != nil {
		return registrationError(err)
	}
	if err := b.Account.Validate(); err != nil {
		return registrationError(err)
	}
	return nil
}

// validateOfferingBinding proves the certificate carries the exact opaque
// offering declared by its build.
func (b InstallationCertificateBody) validateOfferingBinding() error {
	if b.Build.Offering() != b.Subject.Offering {
		return consistencyError()
	}
	return nil
}

// validateDeviceBinding re-derives the device identity from the key the
// certificate carries.
func (b InstallationCertificateBody) validateDeviceBinding() error {
	derived, err := lease.DeviceIDForPublicKey(b.DeviceKey)
	if err != nil {
		return installationBindingError(err)
	}
	if derived != b.Subject.DeviceID {
		return installationBindingError()
	}
	return nil
}

// Scope derives the exact receipt namespace from the two signed certificate
// facts that own it: account and build offering.
func (b InstallationCertificateBody) Scope() (receipt.Scope, error) {
	if err := b.Validate(); err != nil {
		return receipt.Scope{}, err
	}
	return receipt.ScopeFor(b.Account, b.Build.Offering())
}

// AttestationDomain returns the certificate's exact signing namespace.
func (InstallationCertificateBody) AttestationDomain() SigningDomain {
	return SigningDomainInstallationCertificateV1
}

// WriteCanonical writes one validated compact certificate body.
func (b InstallationCertificateBody) WriteCanonical(destination io.Writer) error {
	encoded, err := b.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one bounded canonical certificate body.
func (b InstallationCertificateBody) MarshalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(installationCertificateBodyWire(b))
	if err != nil || len(encoded) > InstallationCertificateBodyJSONMaximumBytes {
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (b *InstallationCertificateBody) UnmarshalJSON(data []byte) error {
	if b == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(InstallationCertificateBodyJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[installationCertificateBodyWire](data, limits)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := InstallationCertificateBody(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*b = candidate
	return nil
}

// Validate closes the signed certificate and binds its envelope to the
// namespace the body declares.
//
// Verification recomputes the domain from the body, so a mismatch cannot
// survive attest.Verify. It can survive decoding, and a document held, logged,
// or passed on before anyone verifies it would carry an envelope claiming one
// namespace over a body declaring another. A shape check that admits a document
// its own verifier will refuse has not closed.
func (d InstallationCertificateDocument) Validate() error {
	if err := d.Body.Validate(); err != nil {
		return err
	}
	if err := d.Attestation.Validate(); err != nil {
		return registrationError(err)
	}
	if d.Attestation.Domain != d.Body.AttestationDomain() {
		return signingDomainError()
	}
	return nil
}

// MarshalJSON emits one bounded canonical signed certificate.
func (d InstallationCertificateDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(installationCertificateDocumentWire(d))
	if err != nil || len(encoded) > InstallationCertificateDocumentJSONMaximumBytes {
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (d *InstallationCertificateDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(InstallationCertificateDocumentJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[installationCertificateDocumentWire](data, limits)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := InstallationCertificateDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// IssueInstallationCertificate signs one validated certificate body.
func IssueInstallationCertificate(body InstallationCertificateBody, signer crypto.Signer) (InstallationCertificateDocument, error) {
	if err := body.Validate(); err != nil {
		return InstallationCertificateDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: body, Signer: signer})
	if err != nil {
		return InstallationCertificateDocument{}, registrationError(err)
	}
	document := InstallationCertificateDocument{Body: body, Attestation: envelope}
	return document, document.Validate()
}

// RegistrationPayload is the complete signed registration decision.
//
// Entitlement is explicit so that even a signed refusal can bind and
// authenticate its Lease subject without inventing a credential. The
// certificate is absent exactly when the decision grants nothing.
type RegistrationPayload struct {
	Certificate *InstallationCertificateDocument `json:"certificate,omitempty"`
	Header      ResponseHeader                   `json:"header"`
	Watermark   UsageWatermark                   `json:"watermark"`
	Lease       lease.Document                   `json:"lease"`
	Entitlement lease.EntitlementID              `json:"entitlement"`
}

// RegistrationDocument is the authenticated response body.
type RegistrationDocument struct {
	Payload     RegistrationPayload            `json:"payload"`
	Attestation attest.Envelope[SigningDomain] `json:"attestation"`
}

func (RegistrationDocument) ControlResponseProjection() {}
func (*RegistrationDocument) ControlResponseDocument()  {}

func (d RegistrationDocument) ValidateJSONProjection(
	encoded []byte,
	limits core.StrictJSONLimits,
) error {
	if err := validateTypedResponseProjection(d, encoded, limits); err != nil {
		return registrationError(err)
	}
	return nil
}

type (
	registrationPayloadWire  RegistrationPayload
	registrationDocumentWire RegistrationDocument
)

// Validate closes subject, offering, status, Lease outcome, and credential
// presence as one decision.
//
// These facts are checked against each other rather than only in isolation. A
// document whose parts are individually well-formed but disagree with one
// another is the shape a partially forged or partially stale response takes.
func (p RegistrationPayload) Validate() error {
	if err := p.validateParts(); err != nil {
		return registrationError(err)
	}
	header, err := p.Lease.Decision.Header()
	if err != nil {
		return registrationError(err)
	}
	if header.Generation != p.Watermark.Generation || header.IssuedAt != p.Header.ProviderTime {
		return registrationError(consistencyError())
	}
	if err := p.validateSubject(header.Subject); err != nil {
		return registrationError(err)
	}
	if err := p.validateOutcome(); err != nil {
		return registrationError(err)
	}
	return nil
}

func (p RegistrationPayload) validateParts() error {
	if err := p.Header.Validate(); err != nil {
		return err
	}
	if p.Header.Family != controlwire.RouteFamilyRegistrations {
		return registrationError(consistencyError())
	}
	if err := p.Entitlement.Validate(); err != nil {
		return registrationError(err)
	}
	if err := p.Lease.Validate(); err != nil {
		return registrationError(err)
	}
	return p.Watermark.Validate()
}

func (p RegistrationPayload) validateSubject(subject lease.Subject) error {
	if subject.Offering != p.Header.Offering || subject.EntitlementID != p.Entitlement ||
		subject.DeviceID != p.Header.Installation {
		return consistencyError()
	}
	if p.Watermark.Subject != subject {
		return consistencyError()
	}
	return p.validateCertificate(subject)
}

func (p RegistrationPayload) validateCertificate(subject lease.Subject) error {
	if p.Certificate == nil {
		return nil
	}
	if err := p.Certificate.Validate(); err != nil {
		return err
	}
	if p.Certificate.Body.Account != p.Header.Account ||
		p.Certificate.Body.Subject != subject ||
		p.Certificate.Body.Build.Offering() != p.Header.Offering ||
		p.Certificate.Body.IssuedAt != p.Header.ProviderTime ||
		p.Certificate.Body.Revision != p.Header.Revision {
		return consistencyError()
	}
	return nil
}

// validateOutcome refuses a decision whose commercial status contradicts the
// Lease outcome it arrived with.
//
// The rule has two halves with different owners. Which status may travel beside
// an outcome is ProductStatus's, and ValidateOutcome states it offering blind:
// what a status means to a given product stays with the product, never here.
// This document restates none of it. What stays here is the half only a document
// knows: a grant hands over a credential and a revocation must not, which is a
// fact about this shape rather than about any status.
//
// A weaker predicate stood here and was too weak for a signed document. It
// admitted a refusal under any valid status, so a signed refusal could name an
// active installation, which is a contradiction the authority would have put
// its own signature on. ValidateOutcome's own contract says it is the rule an
// authenticated document is held to, and the check-in response already held
// itself to it.
func (p RegistrationPayload) validateOutcome() error {
	outcome := p.Lease.Decision.Outcome()
	if err := p.Header.Status.ValidateOutcome(outcome); err != nil {
		return err
	}
	switch outcome {
	case lease.OutcomeGrant:
		if p.Certificate == nil {
			return consistencyError()
		}
		return nil
	case lease.OutcomeRefusal:
		return nil
	case lease.OutcomeRevocation:
		if p.Certificate != nil {
			return consistencyError()
		}
		return nil
	}
	return consistencyError()
}

// AttestationDomain returns the response's exact signing namespace.
func (RegistrationPayload) AttestationDomain() SigningDomain {
	return SigningDomainRegistrationV1
}

// WriteCanonical writes one validated compact response payload.
func (p RegistrationPayload) WriteCanonical(destination io.Writer) error {
	encoded, err := p.MarshalJSON()
	if err != nil {
		return err
	}
	return writeCanonical(destination, encoded)
}

// MarshalJSON emits one bounded canonical payload.
func (p RegistrationPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(registrationPayloadWire(p))
	if err != nil || len(encoded) > RegistrationPayloadJSONMaximumBytes {
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (p *RegistrationPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(RegistrationPayloadJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[registrationPayloadWire](data, limits)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := RegistrationPayload(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

// Validate closes the signed response and binds its envelope to the namespace
// the payload declares, for the reason the certificate above states.
func (d RegistrationDocument) Validate() error {
	if err := d.Payload.Validate(); err != nil {
		return err
	}
	if err := d.Attestation.Validate(); err != nil {
		return registrationError(err)
	}
	if d.Attestation.Domain != d.Payload.AttestationDomain() {
		return signingDomainError()
	}
	return nil
}

// MarshalJSON emits one bounded canonical signed response.
func (d RegistrationDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(registrationDocumentWire(d))
	if err != nil || len(encoded) > RegistrationDocumentJSONMaximumBytes {
		return nil, jsonError(registrationError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (d *RegistrationDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(registrationError())
	}
	limits, err := documentJSONLimits(RegistrationDocumentJSONMaximumBytes)
	if err != nil {
		return jsonError(registrationError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[registrationDocumentWire](data, limits)
	if err != nil {
		return jsonError(registrationError(err))
	}
	candidate := RegistrationDocument(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// IssueRegistration signs one validated registration payload.
func IssueRegistration(payload RegistrationPayload, signer crypto.Signer) (RegistrationDocument, error) {
	if err := payload.Validate(); err != nil {
		return RegistrationDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[SigningDomain]{Body: payload, Signer: signer})
	if err != nil {
		return RegistrationDocument{}, registrationError(err)
	}
	document := RegistrationDocument{Payload: payload, Attestation: envelope}
	return document, document.Validate()
}

func writeCanonical(destination io.Writer, encoded []byte) error {
	if destination == nil {
		return registrationError()
	}
	if _, err := destination.Write(encoded); err != nil {
		return registrationError(err)
	}
	return nil
}

var (
	_ controlwire.RoutedJSONRequest = RegistrationRequest{}
	_ core.Validatable              = RegistrationRequest{}
	_ core.Validatable              = InstallationCertificateBody{}
	_ core.Validatable              = InstallationCertificateDocument{}
	_ core.Validatable              = RegistrationPayload{}
	_ core.Validatable              = RegistrationDocument{}

	_ core.ValidatedJSONMarshaler = RegistrationRequest{}
	_ core.ValidatedJSONMarshaler = InstallationCertificateBody{}
	_ core.ValidatedJSONMarshaler = InstallationCertificateDocument{}
	_ core.ValidatedJSONMarshaler = RegistrationPayload{}
	_ core.ValidatedJSONMarshaler = RegistrationDocument{}

	_ json.Unmarshaler = (*RegistrationRequest)(nil)
	_ json.Unmarshaler = (*InstallationCertificateBody)(nil)
	_ json.Unmarshaler = (*InstallationCertificateDocument)(nil)
	_ json.Unmarshaler = (*RegistrationPayload)(nil)
	_ json.Unmarshaler = (*RegistrationDocument)(nil)

	_ attest.CanonicalBody[SigningDomain] = InstallationCertificateBody{}
	_ attest.CanonicalBody[SigningDomain] = RegistrationPayload{}
)
