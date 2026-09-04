package controlplanetest

import (
	"crypto/ed25519"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	fixturePrincipalIdentity   = "11111111111111111111111111111111"
	fixtureEntitlementIdentity = "22222222222222222222222222222222"
	fixtureBuildCommit         = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fixtureIssuedAtNanoseconds = int64(1_700_000_000_000_000_000)
)

// InstallationRequest selects the product identity and exact deterministic
// test keys. The seeds are test material, never production entropy.
type InstallationRequest struct {
	Offering      core.Offering
	AuthoritySeed [ed25519.SeedSize]byte
	DeviceSeed    [ed25519.SeedSize]byte
}

// Installation is one genuinely authority-signed installation certificate
// and the exact keys and build facts that produced it.
type Installation struct {
	Build            core.BuildIdentity
	AuthorityPrivate ed25519.PrivateKey
	DevicePrivate    ed25519.PrivateKey
	Certificate      controlplane.InstallationCertificateDocument
	AuthorityPublic  core.Ed25519PublicKey
	DevicePublic     core.Ed25519PublicKey
}

// Validate rejects unset, identical, or invalid fixture inputs.
func (r InstallationRequest) Validate() error {
	if err := r.Offering.Validate(); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if seedIsZero(r.AuthoritySeed) || seedIsZero(r.DeviceSeed) || r.AuthoritySeed == r.DeviceSeed {
		return errors.Join(core.ErrPrimitiveContract, errors.New("control-plane test seeds are invalid"))
	}
	return nil
}

func seedIsZero(seed [ed25519.SeedSize]byte) bool {
	for _, value := range seed {
		if value != 0 {
			return false
		}
	}
	return true
}

// IssueInstallation signs one real certificate through Controlplane's public
// issuer after constructing every nested fact through its owning package.
func IssueInstallation(request InstallationRequest) (Installation, error) {
	if err := request.Validate(); err != nil {
		return Installation{}, err
	}
	authorityPrivate := ed25519.NewKeyFromSeed(request.AuthoritySeed[:])
	devicePrivate := ed25519.NewKeyFromSeed(request.DeviceSeed[:])
	authorityPublic, err := core.NewEd25519PublicKey(
		authorityPrivate.Public().(ed25519.PublicKey),
	)
	if err != nil {
		return Installation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	devicePublic, err := core.NewEd25519PublicKey(devicePrivate.Public().(ed25519.PublicKey))
	if err != nil {
		return Installation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	build, err := fixtureBuild(request.Offering)
	if err != nil {
		return Installation{}, err
	}
	body, err := fixtureCertificateBody(build, devicePublic)
	if err != nil {
		return Installation{}, err
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{authorityPublic},
	})
	if err != nil {
		return Installation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	server, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{
		TrustedAuthorityKeys: trusted,
	})
	if err != nil {
		return Installation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	certificate, err := server.IssueInstallationCertificate(body, authorityPrivate)
	if err != nil {
		return Installation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	fixture := Installation{
		AuthorityPrivate: authorityPrivate, DevicePrivate: devicePrivate,
		Certificate: certificate, AuthorityPublic: authorityPublic,
		DevicePublic: devicePublic, Build: build,
	}
	return fixture, fixture.Validate()
}

func fixtureBuild(offering core.Offering) (core.BuildIdentity, error) {
	commit, err := core.ParseBuildCommit(fixtureBuildCommit)
	if err != nil {
		return core.BuildIdentity{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Version: core.NewReleaseVersion(2026, 0, 53), Commit: commit,
		Platform: core.Platform{
			OperatingSystem: core.OperatingSystemDarwin,
			Architecture:    core.CPUArchitectureARM64,
		},
		Offering: offering,
	})
	if err != nil {
		return core.BuildIdentity{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	return build, nil
}

func fixtureCertificateBody(
	build core.BuildIdentity,
	devicePublic core.Ed25519PublicKey,
) (controlplane.InstallationCertificateBody, error) {
	entitlement, err := lease.ParseEntitlementID(fixtureEntitlementIdentity)
	if err != nil {
		return controlplane.InstallationCertificateBody{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	device, err := lease.DeviceIDForPublicKey(devicePublic)
	if err != nil {
		return controlplane.InstallationCertificateBody{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	account, err := receipt.ParsePrincipalIdentity(fixturePrincipalIdentity)
	if err != nil {
		return controlplane.InstallationCertificateBody{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	return controlplane.InstallationCertificateBody{
		IssuedAt: temporal.InstantFromNanoseconds(fixtureIssuedAtNanoseconds),
		Build:    build, Revision: controlwire.Revision2026V1,
		Subject: lease.Subject{
			Offering: build.Offering(), EntitlementID: entitlement, DeviceID: device,
		},
		DeviceKey: devicePublic, Account: account,
	}, nil
}

// Validate rechecks the full fixture and exact public/private key agreement.
func (i Installation) Validate() error {
	if err := errors.Join(i.Build.Validate(), i.Certificate.Validate(),
		i.AuthorityPublic.Validate(), i.DevicePublic.Validate()); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if len(i.AuthorityPrivate) != ed25519.PrivateKeySize ||
		len(i.DevicePrivate) != ed25519.PrivateKeySize {
		return errors.Join(core.ErrPrimitiveContract, errors.New("control-plane test private key is invalid"))
	}
	authorityPublic, err := core.NewEd25519PublicKey(i.AuthorityPrivate.Public().(ed25519.PublicKey))
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	devicePublic, err := core.NewEd25519PublicKey(i.DevicePrivate.Public().(ed25519.PublicKey))
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if authorityPublic != i.AuthorityPublic || devicePublic != i.DevicePublic ||
		i.Certificate.Body.Build != i.Build || i.Certificate.Body.DeviceKey != i.DevicePublic {
		return errors.Join(core.ErrPrimitiveContract, errors.New("control-plane test fixture facts disagree"))
	}
	return nil
}

var (
	_ core.Validatable = InstallationRequest{}
	_ core.Validatable = Installation{}
)
