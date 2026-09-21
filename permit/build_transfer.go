package permit

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// BuildTransfer changes only the build bound to an existing device and permit.
// It is not enrollment or renewal. The product must authorize the destination
// release and durably record its admission before returning this document.
type BuildTransfer struct {
	Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	Permission  Document                                     `json:"permission"`
}

func (d BuildTransfer) Validate() error {
	if err := errors.Join(d.Certificate.Validate(), d.Permission.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if d.Certificate.Body.Build != d.Permission.Terms.Build || d.Certificate.Body.Subject != d.Permission.Terms.Subject {
		return core.ErrPermitBinding
	}
	return nil
}

func (d BuildTransfer) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire BuildTransfer
	return core.MarshalCanonicalJSONDocument(wire(d))
}

func (d *BuildTransfer) UnmarshalJSON(data []byte) error {
	if d == nil {
		return core.ErrPermitContract
	}
	type wire BuildTransfer
	value, err := core.DecodeStrictJSONStructure[wire](data, responseLimits())
	if err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	got := BuildTransfer(value)
	if err := got.Validate(); err != nil {
		return err
	}
	*d = got
	return nil
}

// BuildTransferIssuance consumes previously authenticated facts. The product
// selects Build; this operation cannot select a release or extend any right.
type BuildTransferIssuance struct {
	Signer      ed25519.PrivateKey
	Build       core.BuildIdentity
	Certificate controlplane.VerifiedInstallationCertificate
	Permission  Verified
	Authority   controlplane.Authority
}

func (r BuildTransferIssuance) Validate() error {
	body, err := r.Certificate.Body()
	if err != nil {
		return errors.Join(core.ErrPermitAuthentication, err)
	}
	terms, err := r.Permission.Terms()
	if err != nil {
		return err
	}
	if err := errors.Join(r.Authority.Validate(), r.Build.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	if body.Build != terms.Build || body.Subject != terms.Subject || r.Build == body.Build || r.Build.Offering() != body.Build.Offering() || r.Build.Platform() != body.Build.Platform() {
		return core.ErrPermitBinding
	}
	if len(r.Signer) != ed25519.PrivateKeySize {
		return core.ErrPermitAuthentication
	}
	return nil
}

func IssueBuildTransfer(r BuildTransferIssuance) (BuildTransfer, error) {
	if err := r.Validate(); err != nil {
		return BuildTransfer{}, err
	}
	body, _ := r.Certificate.Body()
	terms, _ := r.Permission.Terms()
	body.Build, terms.Build = r.Build, r.Build
	certificate, err := r.Authority.IssueInstallationCertificate(body, r.Signer)
	if err != nil {
		return BuildTransfer{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	permission, err := Sign(terms, r.Signer)
	if err != nil {
		return BuildTransfer{}, err
	}
	got := BuildTransfer{Certificate: certificate, Permission: permission}
	if err := got.Validate(); err != nil {
		return BuildTransfer{}, err
	}
	return got, nil
}

// BuildTransferVerification binds the response to locally authenticated prior
// facts and the exact destination build selected by the receiving product.
type BuildTransferVerification struct {
	Build               core.BuildIdentity
	PreviousCertificate controlplane.VerifiedInstallationCertificate
	Document            BuildTransfer
	PreviousPermission  Verified
	TrustedKeys         attest.TrustedKeys
	EffectiveAt         temporal.Instant
}

func (r BuildTransferVerification) Validate() error {
	if err := errors.Join(r.Document.Validate(), r.PreviousCertificate.Validate(), r.TrustedKeys.Validate(), r.Build.Validate(), r.EffectiveAt.Validate()); err != nil {
		return errors.Join(core.ErrPermitContract, err)
	}
	before, err := r.PreviousPermission.Terms()
	if err != nil {
		return err
	}
	certificate, _ := r.PreviousCertificate.Body()
	if certificate.Build != before.Build || certificate.Subject != before.Subject || r.Build == before.Build || r.Build.Offering() != before.Build.Offering() || r.Build.Platform() != before.Build.Platform() {
		return core.ErrPermitBinding
	}
	// Compare every field, including future fields, after changing only Build.
	before.Build, certificate.Build = r.Build, r.Build
	if r.Document.Permission.Terms != before || r.Document.Certificate.Body != certificate {
		return core.ErrPermitBinding
	}
	return nil
}

type VerifiedBuildTransfer struct {
	certificate controlplane.VerifiedInstallationCertificate
	permission  Verified
}

func (v VerifiedBuildTransfer) Validate() error {
	certificate, err := v.certificate.Body()
	if err != nil {
		return errors.Join(core.ErrPermitAuthentication, err)
	}
	permission, err := v.permission.Terms()
	if err != nil {
		return err
	}
	if certificate.Build != permission.Build || certificate.Subject != permission.Subject {
		return core.ErrPermitBinding
	}
	return nil
}

func (v VerifiedBuildTransfer) Certificate() (controlplane.VerifiedInstallationCertificate, error) {
	if err := v.Validate(); err != nil {
		return controlplane.VerifiedInstallationCertificate{}, err
	}
	return v.certificate, nil
}

func (v VerifiedBuildTransfer) Permission() (Verified, error) {
	if err := v.Validate(); err != nil {
		return Verified{}, err
	}
	return v.permission, nil
}

func VerifyBuildTransfer(r BuildTransferVerification) (VerifiedBuildTransfer, error) {
	if err := r.Validate(); err != nil {
		return VerifiedBuildTransfer{}, err
	}
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: r.TrustedKeys})
	if err != nil {
		return VerifiedBuildTransfer{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	certificate, err := client.VerifyInstallationCertificate(r.Document.Certificate)
	if err != nil {
		return VerifiedBuildTransfer{}, errors.Join(core.ErrPermitAuthentication, err)
	}
	before, _ := r.PreviousPermission.Terms()
	permission, err := Verify(VerifyRequest{Document: r.Document.Permission, RequestNonce: before.RequestNonce, TrustedKeys: r.TrustedKeys, Subject: before.Subject, Build: r.Build, EffectiveAt: r.EffectiveAt, MinimumGeneration: before.Generation})
	if err != nil {
		return VerifiedBuildTransfer{}, err
	}
	got := VerifiedBuildTransfer{certificate: certificate, permission: permission}
	if err := got.Validate(); err != nil {
		return VerifiedBuildTransfer{}, err
	}
	return got, nil
}
