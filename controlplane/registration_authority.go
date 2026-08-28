package controlplane

import (
	"crypto"
	"errors"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

// RegistrationAuthorityVerification is the complete product-neutral input to
// one authority transaction. ExpectedVerifier is the one-way value read from
// the authority's own account record. PriorReplay is the request previously
// committed under that verifier, when it has already been consumed.
//
// A server reads those two facts and calls VerifyRegistrationAuthority inside
// its own persistence transaction. Primitive deliberately owns no database or
// account policy.
type RegistrationAuthorityVerification struct {
	PriorReplay      *controlwire.ReplayIdentity
	Request          RegistrationRequest
	ExpectedVerifier controlwire.RegistrationTokenVerifier
}

// RegistrationIdentity is the non-secret registration fact an authority may
// use after authenticating the one-use token. It cannot carry source, paths,
// output, evidence, or the registration secret.
type RegistrationIdentity struct {
	Build        core.BuildIdentity
	RequestNonce controlwire.RequestNonce
	DeviceKey    core.Ed25519PublicKey
	Installation lease.DeviceID
	Revision     controlwire.Revision
}

// VerifiedRegistrationAuthority is proof that the request matched the
// authority's persisted one-way verifier and was either fresh or byte-exact to
// the request that consumed it. Its fields are private so callers cannot claim
// that verification happened.
type VerifiedRegistrationAuthority struct {
	replay      controlwire.ReplayIdentity
	identity    RegistrationIdentity
	disposition controlwire.ReplayDisposition
}

// RegistrationCertificateIssuance binds authority-owned commercial facts to
// one verified registration. Build, device, installation, revision, and
// offering are derived from Registration and cannot be repeated beside it.
type RegistrationCertificateIssuance struct {
	Registration VerifiedRegistrationAuthority
	IssuedAt     temporal.Instant
	Account      receipt.AccountIdentity
	Entitlement  lease.EntitlementID
}

// Validate closes the authority-supplied verification facts. Authentication is
// part of validation because accepting a structurally valid but wrong token is
// not a meaningful intermediate state.
func (v RegistrationAuthorityVerification) Validate() error {
	if err := errors.Join(v.Request.Validate(), v.ExpectedVerifier.Validate()); err != nil {
		return registrationError(err)
	}
	if v.PriorReplay != nil {
		if err := v.PriorReplay.Validate(); err != nil {
			return registrationError(err)
		}
	}
	presented, err := v.Request.Token.Verifier()
	if err != nil {
		return registrationError(err)
	}
	if !v.ExpectedVerifier.Equal(presented) {
		return registrationError(core.ErrControlWireToken)
	}
	return nil
}

// Identity projects only the non-secret facts carried by a valid request.
func (r RegistrationRequest) Identity() (RegistrationIdentity, error) {
	if err := r.Validate(); err != nil {
		return RegistrationIdentity{}, err
	}
	identity := RegistrationIdentity{
		Build: r.Build, RequestNonce: r.RequestNonce, DeviceKey: r.DeviceKey,
		Installation: r.Installation, Revision: r.Revision,
	}
	return identity, identity.Validate()
}

// Validate closes the non-secret identity and re-derives its device binding.
func (i RegistrationIdentity) Validate() error {
	if err := errors.Join(
		i.Build.Validate(), i.RequestNonce.Validate(), i.DeviceKey.Validate(),
		i.Installation.Validate(), i.Revision.Validate(),
	); err != nil {
		return registrationError(err)
	}
	derived, err := lease.DeviceIDForPublicKey(i.DeviceKey)
	if err != nil || derived != i.Installation {
		return installationBindingError(err)
	}
	return nil
}

// VerifyRegistrationAuthority owns no persistence. It returns the exact replay
// record the server persists atomically with its own registration decision and
// destroys the presented token on every return path. A changed second use
// returns no proof and the replay-conflict identity, regardless of which
// request field changed.
func (s Server) VerifyRegistrationAuthority(
	verification RegistrationAuthorityVerification,
) (verified VerifiedRegistrationAuthority, resultErr error) {
	if err := s.Validate(); err != nil {
		return VerifiedRegistrationAuthority{}, registrationError(err)
	}
	defer func() {
		if err := verification.Request.Token.Destroy(); err != nil {
			verified = VerifiedRegistrationAuthority{}
			resultErr = registrationError(errors.Join(resultErr, err))
		}
	}()
	replay, disposition, err := resolveRegistrationAuthority(verification)
	if err != nil {
		return VerifiedRegistrationAuthority{}, err
	}
	identity, err := verification.Request.Identity()
	if err != nil {
		return VerifiedRegistrationAuthority{}, err
	}
	verified = VerifiedRegistrationAuthority{
		identity: identity, replay: replay, disposition: disposition,
	}
	return verified, verified.Validate()
}

func resolveRegistrationAuthority(
	verification RegistrationAuthorityVerification,
) (controlwire.ReplayIdentity, controlwire.ReplayDisposition, error) {
	if err := verification.Validate(); err != nil {
		return controlwire.ReplayIdentity{}, controlwire.ReplayDispositionUnknown, err
	}
	replay, err := controlwire.CommitReplayIdentity(verification.Request)
	if err != nil {
		return controlwire.ReplayIdentity{}, controlwire.ReplayDispositionUnknown, registrationError(err)
	}
	if verification.PriorReplay == nil {
		return replay, controlwire.ReplayDispositionFresh, nil
	}
	if !verification.PriorReplay.Equal(replay) {
		return controlwire.ReplayIdentity{}, controlwire.ReplayDispositionUnknown,
			registrationError(core.ErrControlWireReplayConflict)
	}
	return replay, controlwire.ReplayDispositionExact, nil
}

// Validate closes the sealed non-secret result. The token is deliberately not
// retained: VerifyRegistrationAuthority destroys it on every return path.
func (v VerifiedRegistrationAuthority) Validate() error {
	if err := errors.Join(v.identity.Validate(), v.replay.Validate(), v.disposition.Validate()); err != nil {
		return registrationError(err)
	}
	return nil
}

// Identity returns the authenticated request's non-secret facts.
func (v VerifiedRegistrationAuthority) Identity() (RegistrationIdentity, error) {
	if err := v.Validate(); err != nil {
		return RegistrationIdentity{}, err
	}
	return v.identity, nil
}

// Replay returns the exact record the authority persists and whether this was
// its first acceptance or an exact retry.
func (v VerifiedRegistrationAuthority) Replay() (
	controlwire.ReplayIdentity,
	controlwire.ReplayDisposition,
	error,
) {
	if err := v.Validate(); err != nil {
		return controlwire.ReplayIdentity{}, controlwire.ReplayDispositionUnknown, err
	}
	return v.replay, v.disposition, nil
}

// Validate closes the authority-owned facts needed to issue a certificate.
func (i RegistrationCertificateIssuance) Validate() error {
	if err := errors.Join(
		i.Registration.Validate(), i.IssuedAt.Validate(),
		i.Account.Validate(), i.Entitlement.Validate(),
	); err != nil {
		return registrationError(err)
	}
	return nil
}

// IssueRegisteredInstallation signs a certificate derived from one
// authenticated registration. No caller can substitute build, offering,
// device key, installation, or revision beside the verified request.
func (s Server) IssueRegisteredInstallation(
	issuance RegistrationCertificateIssuance,
	signer crypto.Signer,
) (InstallationCertificateDocument, error) {
	if err := s.Validate(); err != nil {
		return InstallationCertificateDocument{}, registrationError(err)
	}
	if err := issuance.Validate(); err != nil {
		return InstallationCertificateDocument{}, err
	}
	identity, err := issuance.Registration.Identity()
	if err != nil {
		return InstallationCertificateDocument{}, err
	}
	body := InstallationCertificateBody{
		IssuedAt: issuance.IssuedAt, Build: identity.Build, Revision: identity.Revision,
		Subject: lease.Subject{
			Offering: identity.Build.Offering(), EntitlementID: issuance.Entitlement,
			DeviceID: identity.Installation,
		},
		DeviceKey: identity.DeviceKey, Account: issuance.Account,
	}
	return issueInstallationCertificate(body, signer)
}

var (
	_ core.Validatable = RegistrationAuthorityVerification{}
	_ core.Validatable = RegistrationIdentity{}
	_ core.Validatable = VerifiedRegistrationAuthority{}
	_ core.Validatable = RegistrationCertificateIssuance{}
)
