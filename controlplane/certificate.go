package controlplane

import (
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// VerifiedInstallationCertificate is returned only after an installation
// certificate's authority signature has verified.
//
// Its fields are unexported and reachable only through accessors that
// revalidate, so a caller cannot construct one and possessing it is itself the
// proof that verification happened rather than a claim that it did.
//
// The type exists because a certificate is sometimes the whole authority. A
// live exchange binds a response to the request that provoked it, but an
// installation loading a credential it stored earlier has no request to bind
// against: the certificate's own signature, and the device key it names, are
// all there is to check. Making that a first-class result keeps the weaker
// situation from being served by a weaker rule.
type VerifiedInstallationCertificate struct {
	proof      attest.Verified[SigningDomain]
	deviceKeys attest.TrustedKeys
	body       InstallationCertificateBody
}

// VerifyInstallationCertificate authenticates one authority-issued installation
// certificate and returns the device key it names as a trust set.
//
// The order is the security property, and it is the reason this is one function
// rather than two steps a caller sequences. The certificate is verified against
// the authority's keys first, and only then does the device key inside it become
// an authority for anything. A caller that read the device key out of an
// unverified certificate and then checked a request against it would be letting
// a self-signed document nominate the key that validates it.
func VerifyInstallationCertificate(
	certificate InstallationCertificateDocument,
	trusted attest.TrustedKeys,
) (VerifiedInstallationCertificate, error) {
	if err := certificate.Validate(); err != nil {
		return VerifiedInstallationCertificate{}, registrationError(err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: certificate.Body, Envelope: certificate.Attestation, TrustedKeys: trusted,
	})
	if err != nil {
		return VerifiedInstallationCertificate{}, registrationError(err)
	}
	deviceKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{certificate.Body.DeviceKey},
	})
	if err != nil {
		return VerifiedInstallationCertificate{}, registrationError(err)
	}
	verified := VerifiedInstallationCertificate{
		proof: proof, deviceKeys: deviceKeys, body: certificate.Body,
	}
	if err := verified.Validate(); err != nil {
		return VerifiedInstallationCertificate{}, err
	}
	return verified, nil
}

// Validate revalidates every proof the value claims to hold, so a sealed type
// cannot be presented as evidence of something it never proved.
func (v VerifiedInstallationCertificate) Validate() error {
	if err := v.proof.Validate(); err != nil {
		return registrationError(err)
	}
	if err := v.deviceKeys.Validate(); err != nil {
		return registrationError(err)
	}
	if err := v.body.Validate(); err != nil {
		return registrationError(err)
	}
	return nil
}

// Body returns the verified certificate body.
func (v VerifiedInstallationCertificate) Body() (InstallationCertificateBody, error) {
	if err := v.Validate(); err != nil {
		return InstallationCertificateBody{}, err
	}
	return v.body, nil
}

// DeviceKeys returns the trust set holding exactly the device key the authority
// bound to this installation.
//
// It is the only key an installation's own signatures may be checked against.
// Returning a trust set rather than a bare key keeps callers from assembling
// their own, which is where a second admitted key would creep in.
func (v VerifiedInstallationCertificate) DeviceKeys() (attest.TrustedKeys, error) {
	if err := v.Validate(); err != nil {
		return attest.TrustedKeys{}, err
	}
	return v.deviceKeys, nil
}

// Proof returns the authority's verified signature over the certificate.
func (v VerifiedInstallationCertificate) Proof() (attest.Verified[SigningDomain], error) {
	if err := v.Validate(); err != nil {
		return attest.Verified[SigningDomain]{}, err
	}
	return v.proof, nil
}

var _ core.Validatable = VerifiedInstallationCertificate{}
