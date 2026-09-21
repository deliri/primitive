package distributionauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/permit"
	"github.com/deliri/primitive/v2026/temporal"
)

func upgradeDeliveryFixture(t testing.TB) (UpgradeDeliveryProjection, distribution.UpgradeGrantExpectation, permit.BuildTransferVerification) {
	t.Helper()
	request := newDistributionAuthFixture(t, distributionAuthFixtureRequest{}).upgrade
	f := newPublicationAuthFixture(t, publicationAuthFixtureRequest{offering: request.Certificate.Body.Build.Offering()})
	authority := distributionAuthServer(t, f.authority)
	certificate, err := authority.VerifyInstallationCertificate(request.Certificate)
	if err != nil {
		t.Fatalf("VerifyInstallationCertificate = %v, want nil", err)
	}
	generation, err := lease.NewGeneration(2)
	if err != nil {
		t.Fatalf("NewGeneration = %v, want nil", err)
	}
	retry, err := temporal.DurationFromMilliseconds(1000)
	if err != nil {
		t.Fatalf("DurationFromMilliseconds = %v, want nil", err)
	}
	terms := permit.Terms{Revision: permit.RevisionV1, RequestNonce: request.Request.Payload.Nonce, Subject: request.Certificate.Body.Subject, Build: request.Certificate.Body.Build, Generation: generation, NotBefore: f.installation.Certificate.Body.IssuedAt, ExpiresAt: f.grant.Payload.ExpiresAt, ContactAt: f.grant.Payload.ExpiresAt, RetryAfter: retry}
	signed, err := permit.Sign(terms, f.installation.AuthorityPrivate)
	if err != nil {
		t.Fatalf("Sign prior permission = %v, want nil", err)
	}
	prior, err := permit.Verify(permit.VerifyRequest{Document: signed, TrustedKeys: f.authority, RequestNonce: terms.RequestNonce, Subject: terms.Subject, Build: terms.Build, EffectiveAt: terms.NotBefore, MinimumGeneration: generation})
	if err != nil {
		t.Fatalf("Verify prior permission = %v, want nil", err)
	}
	transfer, err := permit.IssueBuildTransfer(permit.BuildTransferIssuance{Certificate: certificate, Permission: prior, Build: request.Request.Payload.Available.Candidate, Authority: authority, Signer: f.installation.AuthorityPrivate})
	if err != nil {
		t.Fatalf("IssueBuildTransfer = %v, want nil", err)
	}
	projection := UpgradeDeliveryProjection{Download: socketUpgrade(t, f), Transfer: transfer}
	return projection, distribution.UpgradeGrantExpectation{Request: request.Request.Payload, TrustedKeys: f.authority, ObservedAt: terms.NotBefore}, permit.BuildTransferVerification{PreviousCertificate: certificate, PreviousPermission: prior, TrustedKeys: f.authority, Build: request.Request.Payload.Available.Candidate, EffectiveAt: terms.NotBefore}
}

func TestUpgradeDeliveryLayerTriad(t *testing.T) {
	t.Parallel()
	projection, download, transfer := upgradeDeliveryFixture(t)
	canonical, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("projection MarshalJSON = %v, want nil", err)
	}
	var got UpgradeDeliveryDocument
	if err := got.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("UnmarshalJSON = %v, want nil", err)
	}
	download.Document, transfer.Document = got.Download, got.Transfer
	if _, err := distribution.VerifyUpgradeGrant(download); err != nil {
		t.Fatalf("VerifyUpgradeGrant = %v, want nil", err)
	}
	verified, err := permit.VerifyBuildTransfer(transfer)
	if err != nil {
		t.Fatalf("VerifyBuildTransfer = %v, want nil", err)
	}
	permission, err := verified.Permission()
	if err != nil {
		t.Fatalf("Permission = %v, want nil", err)
	}
	terms, err := permission.Terms()
	if err != nil || terms.Actions != (permit.Actions{}) || terms.Generation != got.Transfer.Permission.Terms.Generation {
		t.Fatalf("empty rights = %v/%v, want unchanged empty actions and generation", terms, err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "empty response releases no download or permission", data: nil},
		{name: "null response releases no download or permission", data: []byte("null")},
		{name: "truncated response releases neither partial component", data: canonical[:len(canonical)-1]},
		{name: "trailing response cannot replace one receipt", data: append(bytes.Clone(canonical), canonical...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := got
			err := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(err, core.ErrDistributionContract) || upgradeDeliveryFacts(t, receiver) != upgradeDeliveryFacts(t, got) {
				t.Fatalf("UnmarshalJSON refusal = %v, receiver preserved = %t, want ErrDistributionContract/true", err, upgradeDeliveryFacts(t, receiver) == upgradeDeliveryFacts(t, got))
			}
		})
	}
}

func FuzzUpgradeDeliverySemanticClosure(f *testing.F) {
	projection, download, transfer := upgradeDeliveryFixture(f)
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(canonical[:len(canonical)-1])
	f.Fuzz(func(t *testing.T, data []byte) {
		var got UpgradeDeliveryDocument
		if err := got.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("decode seed = %v, want nil", err)
		}
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrDistributionContract) {
				t.Fatalf("decode refusal = %v, want ErrDistributionContract", err)
			}
			var want UpgradeDeliveryDocument
			if seedErr := want.UnmarshalJSON(canonical); seedErr != nil || upgradeDeliveryFacts(t, got) != upgradeDeliveryFacts(t, want) {
				t.Fatalf("refused receiver preserved = %t/%v, want true/nil", upgradeDeliveryFacts(t, got) == upgradeDeliveryFacts(t, want), seedErr)
			}
			return
		}
		// witness:waiver test/fuzz-boundary -- receive-only download bearers intentionally cannot marshal; exact capability commitment and both independent signature verifiers own this oracle.
		var want UpgradeDeliveryDocument
		if err := want.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("decode canonical = %v, want nil", err)
		}
		commitment, err := got.Download.Capability.Commitment()
		if err != nil || commitment != got.Download.Payload.Capability {
			t.Fatalf("capability commitment parity = %t/%v, want true/nil", commitment == got.Download.Payload.Capability, err)
		}
		d, p := download, transfer
		d.Document, p.Document = got.Download, got.Transfer
		grant, grantErr := distribution.VerifyUpgradeGrant(d)
		permission, permissionErr := permit.VerifyBuildTransfer(p)
		if grantErr != nil && (grant.Validate() == nil || !errors.Is(grantErr, core.ErrDistributionVerification)) {
			t.Fatalf("refused download proof = valid/%v, want invalid proof", grantErr)
		}
		if permissionErr != nil && (permission.Validate() == nil || (!errors.Is(permissionErr, core.ErrPermitContract) && !errors.Is(permissionErr, core.ErrPermitBinding) && !errors.Is(permissionErr, core.ErrPermitAuthentication))) {
			t.Fatalf("refused permission proof = valid/%v, want invalid proof", permissionErr)
		}
		if grantErr == nil && permissionErr == nil {
			if upgradeDeliveryFacts(t, got) != upgradeDeliveryFacts(t, want) {
				t.Fatal("authenticated recombination = foreign, want exact signed delivery")
			}
		} else if upgradeDeliveryFacts(t, got) == upgradeDeliveryFacts(t, want) {
			t.Fatalf("canonical delivery verification = %v/%v, want nil/nil", grantErr, permissionErr)
		}
	})
}

type upgradeDeliveryFactSet struct {
	Transfer    permit.BuildTransfer
	Attestation attest.Envelope[distribution.SigningDomain]
	Payload     distribution.UpgradeGrantPayload
	Capability  objectstore.DownloadCapabilityCommitment
}

func upgradeDeliveryFacts(t testing.TB, document UpgradeDeliveryDocument) upgradeDeliveryFactSet {
	t.Helper()
	commitment, err := document.Download.Capability.Commitment()
	if err != nil {
		t.Fatalf("capability commitment = %v, want nil", err)
	}
	return upgradeDeliveryFactSet{Capability: commitment, Payload: document.Download.Payload, Attestation: document.Download.Attestation, Transfer: document.Transfer}
}
