package distributionauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"testing"
)

func TestCredentialNominationAndRouteLayerTriad(t *testing.T) {
	t.Parallel()
	request := newDistributionAuthFixture(t, distributionAuthFixtureRequest{})
	foreign := newDistributionAuthFixture(t, distributionAuthFixtureRequest{deviceByte: 0xee})
	publication := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	t.Run("update", func(t *testing.T) {
		t.Parallel()
		bad := request.update
		bad.Request.Attestation.Signer = foreign.update.Certificate.Body.DeviceKey
		absent := request.update
		absent.Certificate = controlplane.InstallationCertificateDocument{}
		credentialBoundary[UpdateRequestDocument, *UpdateRequestDocument](t, request.update, bad, absent, func(d UpdateRequestDocument) (UpdateRequestDocument, error) {
			return AssembleUpdate(UpdateRequestAssembly(d))
		}, func(d UpdateRequestDocument) ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(updateRequestDocumentWire(d))
		})
	})
	t.Run("upgrade", func(t *testing.T) {
		t.Parallel()
		bad := request.upgrade
		bad.Request.Attestation.Signer = foreign.upgrade.Certificate.Body.DeviceKey
		absent := request.upgrade
		absent.Certificate = controlplane.InstallationCertificateDocument{}
		credentialBoundary[UpgradeRequestDocument, *UpgradeRequestDocument](t, request.upgrade, bad, absent, func(d UpgradeRequestDocument) (UpgradeRequestDocument, error) {
			return AssembleUpgrade(UpgradeRequestAssembly(d))
		}, func(d UpgradeRequestDocument) ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(upgradeRequestDocumentWire(d))
		})
	})
	t.Run("publication", func(t *testing.T) {
		t.Parallel()
		bad := publication.document
		bad.Request.Attestation.Signer = foreign.update.Certificate.Body.DeviceKey
		absent := publication.document
		absent.Certificate = controlplane.InstallationCertificateDocument{}
		credentialBoundary[PublicationRequestDocument, *PublicationRequestDocument](t, publication.document, bad, absent, func(d PublicationRequestDocument) (PublicationRequestDocument, error) {
			return AssemblePublication(PublicationRequestAssembly(d))
		}, func(d PublicationRequestDocument) ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(publicationRequestDocumentWire(d))
		})
	})
	t.Run("completion", func(t *testing.T) {
		t.Parallel()
		bad := publication.completion
		bad.Completion.Attestation.Signer = foreign.update.Certificate.Body.DeviceKey
		absent := publication.completion
		absent.Certificate = controlplane.InstallationCertificateDocument{}
		credentialBoundary[PublicationCompletionDocument, *PublicationCompletionDocument](t, publication.completion, bad, absent, func(d PublicationCompletionDocument) (PublicationCompletionDocument, error) {
			return AssemblePublicationCompletion(PublicationCompletionAssembly(d))
		}, func(d PublicationCompletionDocument) ([]byte, error) {
			return core.MarshalCanonicalJSONDocument(publicationCompletionDocumentWire(d))
		})
	})
}
func credentialBoundary[D interface {
	comparable
	Validate() error
	MarshalJSON() ([]byte, error)
	ControlRoute() (controlwire.RouteContract, error)
}, P interface {
	*D
	UnmarshalJSON([]byte) error
}](t *testing.T, base, bad, absent D, assemble func(D) (D, error), wire func(D) ([]byte, error)) {
	t.Helper()
	var zero D
	if base == bad {
		t.Fatal("signer mutation = unchanged, want one changed key")
	}
	for _, tc := range []struct {
		name    string
		input   D
		wantErr error
	}{{"nominated signer", base, nil}, {"foreign signer", bad, core.ErrControlPlaneResponseBinding}, {"absent certificate", absent, core.ErrControlPlaneContract}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := assemble(tc.input)
			route, routeErr := tc.input.ControlRoute()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != zero {
					t.Fatalf("Assemble = %v/%v, want zero/%v", got, err, tc.wantErr)
				}
				if !errors.Is(routeErr, tc.wantErr) || route != (controlwire.RouteContract{}) {
					t.Fatalf("ControlRoute = %v/%v, want zero/%v", route, routeErr, tc.wantErr)
				}
				return
			}
			if err != nil || got != base || routeErr != nil || route.Validate() != nil {
				t.Fatalf("valid credential = %v/%v/%v, want exact assembly and route", got, err, routeErr)
			}
		})
	}
	hostile, err := wire(bad)
	if err != nil {
		t.Fatal(err)
	}
	receiver := base
	err = P(&receiver).UnmarshalJSON(hostile)
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneResponseBinding) || receiver != base {
		t.Fatalf("JSON foreign signer = %v/%v, want preserved typed binding refusal", receiver, err)
	}
	encoded, err := base.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	padded := append(bytes.Repeat([]byte(" \r\n\t"), 262144), encoded...)
	var got D
	if err := P(&got).UnmarshalJSON(padded); err != nil || got != base {
		t.Fatalf("large whitespace decode = %v/%v, want exact credential", got, err)
	}
}
func TestCompletionProjectionNominationLayerTriad(t *testing.T) {
	t.Parallel()
	base := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	other := newPublicationAuthFixture(t, publicationAuthFixtureRequest{deviceByte: 0xee})
	for _, tc := range []struct {
		name        string
		certificate controlplane.InstallationCertificateDocument
		wantErr     error
	}{
		{"nominated device", base.installation.Certificate, nil}, {"foreign device", other.installation.Certificate, core.ErrControlPlaneResponseBinding}, {"absent certificate", controlplane.InstallationCertificateDocument{}, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := AssemblePublicationCompletionProjection(PublicationCompletionProjectionAssembly{Completion: base.completionProjection.completion, Certificate: tc.certificate})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got.Validate() == nil {
					t.Fatalf("completion projection = %v, want no valid projection/%v", err, tc.wantErr)
				}
				return
			}
			wire, marshalErr := got.MarshalJSON()
			var received PublicationCompletionDocument
			decodeErr := received.UnmarshalJSON(wire)
			if err != nil || marshalErr != nil || decodeErr != nil || received != base.completion {
				t.Fatalf("completion projection = %v/%v/%v, want exact receive document", err, marshalErr, decodeErr)
			}
		})
	}
}

func TestCompletionProjectionSignerLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	got, err := fixture.completionProjection.completion.Signer()
	want := fixture.completion.Completion.Attestation.Signer
	if err != nil || got != want {
		t.Fatalf("completion Signer = (%v, %v), want (%v, nil)", got, err, want)
	}
	var absent distribution.PublicationCompletionProjection
	zero, err := absent.Signer()
	if !errors.Is(err, core.ErrDistributionContract) || zero != (core.Ed25519PublicKey{}) {
		t.Fatalf("absent Signer = (%v, %v), want zero and typed distribution refusal", zero, err)
	}
}
