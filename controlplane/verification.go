package controlplane

import (
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// RegistrationVerification carries the caller's exact trust and request facts
// into response authentication.
//
// Every field is required. An installation that verified a response without
// naming the request it made, the build it is, and the key it holds would be
// checking only that somebody signed something.
type RegistrationVerification struct {
	Expected    ResponseExpectation
	Build       core.BuildIdentity
	Document    RegistrationDocument
	TrustedKeys attest.TrustedKeys
	DeviceKey   core.Ed25519PublicKey
}

// VerifiedRegistration is returned only after the response, the credential when
// one is present, the Lease, and the request binding have all verified.
//
// Its fields are unexported and reachable only through accessors that revalidate.
// A caller cannot construct one, so possessing this value is itself the proof
// that verification happened rather than a claim that it did.
type VerifiedRegistration struct {
	payload          RegistrationPayload
	leaseProof       lease.Verified
	responseProof    attest.Verified[SigningDomain]
	certificateProof attest.Verified[SigningDomain]
}

// Validate closes the verification request's own shape.
func (v RegistrationVerification) Validate() error {
	if err := v.Document.Validate(); err != nil {
		return err
	}
	if err := v.Expected.Validate(); err != nil {
		return err
	}
	if err := v.Build.Validate(); err != nil {
		return registrationError(err)
	}
	if err := v.DeviceKey.Validate(); err != nil {
		return registrationError(err)
	}
	if err := v.TrustedKeys.Validate(); err != nil {
		return registrationError(err)
	}
	return nil
}

// VerifyRegistration authenticates one registration response against the exact
// request that produced it.
//
// The order is deliberate. The binding check runs before any signature work, so
// a response for somebody else's request is refused without spending
// verification on it. The response signature comes next, then the Lease, then
// the certificate: each stage only runs on bytes an earlier stage has already
// accepted.
func VerifyRegistration(request RegistrationVerification) (VerifiedRegistration, error) {
	if err := request.Validate(); err != nil {
		return VerifiedRegistration{}, err
	}
	if err := request.Document.Payload.Header.ValidateAgainst(request.Expected); err != nil {
		return VerifiedRegistration{}, err
	}
	responseProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body:        request.Document.Payload,
		Envelope:    request.Document.Attestation,
		TrustedKeys: request.TrustedKeys,
	})
	if err != nil {
		return VerifiedRegistration{}, registrationError(err)
	}
	leaseProof, err := verifyRegistrationLease(request.Document.Payload, request.TrustedKeys)
	if err != nil {
		return VerifiedRegistration{}, err
	}
	certificateProof, err := verifyRegistrationCertificate(request)
	if err != nil {
		return VerifiedRegistration{}, err
	}
	verified := VerifiedRegistration{
		payload: request.Document.Payload, responseProof: responseProof,
		certificateProof: certificateProof, leaseProof: leaseProof,
	}
	return verified, verified.Validate()
}

// verifyRegistrationLease authenticates the nested Lease against the subject
// the Lease itself names, so a valid Lease for one installation cannot be
// carried inside another installation's response.
func verifyRegistrationLease(payload RegistrationPayload, trusted attest.TrustedKeys) (lease.Verified, error) {
	header, err := payload.Lease.Decision.Header()
	if err != nil {
		return lease.Verified{}, registrationError(err)
	}
	verified, err := lease.Verify(lease.VerifyRequest{
		Document: payload.Lease, TrustedKeys: trusted, ExpectedSubject: header.Subject,
	})
	if err != nil {
		return lease.Verified{}, registrationError(err)
	}
	return verified, nil
}

// verifyRegistrationCertificate binds the credential to this exact machine and
// this exact binary before verifying its signature.
//
// A signed certificate naming another device key or another build is authentic
// and still wrong: it is somebody else's credential, or this machine's
// credential for a binary it is not running.
func verifyRegistrationCertificate(request RegistrationVerification) (attest.Verified[SigningDomain], error) {
	certificate := request.Document.Payload.Certificate
	if certificate == nil {
		return attest.Verified[SigningDomain]{}, nil
	}
	if certificate.Body.DeviceKey != request.DeviceKey || certificate.Body.Build != request.Build {
		return attest.Verified[SigningDomain]{}, installationBindingError()
	}
	verified, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body:        certificate.Body,
		Envelope:    certificate.Attestation,
		TrustedKeys: request.TrustedKeys,
	})
	if err != nil {
		return attest.Verified[SigningDomain]{}, registrationError(err)
	}
	return verified, nil
}

// Validate rejects a zero or internally contradictory authenticated result.
func (v VerifiedRegistration) Validate() error {
	if err := v.payload.Validate(); err != nil {
		return err
	}
	if err := v.responseProof.Validate(); err != nil {
		return registrationError(err)
	}
	if err := v.leaseProof.Validate(); err != nil {
		return registrationError(err)
	}
	return v.validateCertificateProof()
}

// validateCertificateProof requires a proof exactly when a certificate is
// present, so a granted registration cannot report success with an unverified
// credential.
func (v VerifiedRegistration) validateCertificateProof() error {
	if v.payload.Certificate == nil {
		return nil
	}
	if err := v.certificateProof.Validate(); err != nil {
		return registrationError(err)
	}
	return nil
}

// Payload returns the authenticated payload.
func (v VerifiedRegistration) Payload() (RegistrationPayload, error) {
	if err := v.Validate(); err != nil {
		return RegistrationPayload{}, err
	}
	return v.payload, nil
}

// Lease returns the authenticated Lease, which is what Gate decides on.
func (v VerifiedRegistration) Lease() (lease.Verified, error) {
	if err := v.Validate(); err != nil {
		return lease.Verified{}, err
	}
	return v.leaseProof, nil
}

// Certificate returns the authenticated installation certificate. A registration
// that granted nothing carries none, and reports that rather than a zero value.
func (v VerifiedRegistration) Certificate() (InstallationCertificateDocument, error) {
	if err := v.Validate(); err != nil {
		return InstallationCertificateDocument{}, err
	}
	if v.payload.Certificate == nil {
		return InstallationCertificateDocument{}, registrationError()
	}
	return *v.payload.Certificate, nil
}

var (
	_ core.Validatable = RegistrationVerification{}
	_ core.Validatable = VerifiedRegistration{}
)
