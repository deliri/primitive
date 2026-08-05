package controlplane

import (
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// checkInBinding is the off-wire projection of the facts every product's
// check-in must agree on, whatever else that product reports.
//
// Each product reports different usage, but all of them are the same
// installation talking about the same entitlement, so the identity half is
// factored out and validated once. A product that carried its own copy of this
// rule could drift into accepting a window for one device under another
// device's certificate.
type checkInBinding struct {
	Build        core.BuildIdentity
	Subject      lease.Subject
	Installation lease.DeviceID
	RequestNonce controlwire.RequestNonce
}

func (b checkInBinding) Validate() error {
	if err := errors.Join(
		b.Build.Validate(), b.Subject.Validate(),
		b.Installation.Validate(), b.RequestNonce.Validate(),
	); err != nil {
		return checkInError(err)
	}
	product, err := lease.ProductForOffering(b.Build.Offering())
	if err != nil || b.Subject.Product != product || b.Subject.DeviceID != b.Installation {
		return consistencyError(err)
	}
	return nil
}

// validateUsageFreshness proves the reported observation instant lies inside
// the window it claims to describe.
//
// A freshness stamp outside its own bounds is either a broken clock or a window
// assembled from another interval, and both make the aggregate meaningless.
func validateUsageFreshness(bounds temporal.IntervalBounds, freshness temporal.Instant) error {
	startComparison, startErr := freshness.Compare(bounds.Start)
	endComparison, endErr := freshness.Compare(bounds.End)
	if errors.Join(startErr, endErr) != nil ||
		startComparison == core.ComparisonLess ||
		endComparison == core.ComparisonGreater {
		return usageWindowError(startErr, endErr)
	}
	return nil
}

// validateCheckInDocument closes a complete check-in: the binding, the
// credential it presents, its signature envelope, and the agreement between
// them.
func validateCheckInDocument(
	binding checkInBinding,
	certificate InstallationCertificateDocument,
	attestation attest.Envelope[SigningDomain],
	domain SigningDomain,
) error {
	if err := errors.Join(
		binding.Validate(), certificate.Validate(),
		attestation.Validate(), domain.Validate(),
	); err != nil {
		return checkInError(err)
	}
	if certificate.Body.Subject != binding.Subject ||
		certificate.Body.Build != binding.Build ||
		attestation.Domain != domain {
		return consistencyError()
	}
	return nil
}

// verifyCheckInCertificate authenticates the authority-issued credential and
// returns the device key it names as the only key admitted for the request.
//
// The order is the security property. The certificate is verified against the
// authority's trusted keys first, and only then does the device key inside it
// become an authority for anything. Verifying the request first would let a
// self-signed request nominate the key that validates it.
func verifyCheckInCertificate(
	certificate InstallationCertificateDocument,
	trusted attest.TrustedKeys,
) (attest.Verified[SigningDomain], attest.TrustedKeys, error) {
	certificateProof, err := attest.Verify(attest.VerifyRequest[SigningDomain]{
		Body: certificate.Body, Envelope: certificate.Attestation, TrustedKeys: trusted,
	})
	if err != nil {
		return attest.Verified[SigningDomain]{}, attest.TrustedKeys{}, checkInError(err)
	}
	deviceKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{certificate.Body.DeviceKey},
	})
	if err != nil {
		return attest.Verified[SigningDomain]{}, attest.TrustedKeys{}, checkInError(err)
	}
	return certificateProof, deviceKeys, nil
}

// validateCheckInProofs revalidates both proofs a verified check-in holds, so
// the sealed type cannot be presented as evidence of something it never proved.
func validateCheckInProofs(
	requestErr error,
	certificateProof attest.Verified[SigningDomain],
	requestProof attest.Verified[SigningDomain],
) error {
	if err := errors.Join(requestErr, certificateProof.Validate(), requestProof.Validate()); err != nil {
		return checkInError(err)
	}
	return nil
}

var _ core.Validatable = checkInBinding{}
