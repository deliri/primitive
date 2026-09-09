package distribution_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"testing"
)

func TestUnsetJSONOwnersEmitNoWireEvidence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		marshal func() ([]byte, error)
	}{
		{"signing domain", distribution.SigningDomainUnknown.MarshalJSON},
		{"request commitment", (distribution.RequestCommitment{}).MarshalJSON},
		{"publication request payload", (distribution.PublicationRequestPayload{}).MarshalJSON},
		{"publication request document", (distribution.PublicationRequestDocument{}).MarshalJSON},
		{"publication grant payload", (distribution.PublicationGrantPayload{}).MarshalJSON},
		{"publication grant projection", (distribution.PublicationGrantProjection{}).MarshalJSON},
		{"publication completion payload", (distribution.PublicationCompletionPayload{}).MarshalJSON},
		{"publication completion document", (distribution.PublicationCompletionDocument{}).MarshalJSON},
		{"publication completion projection", (distribution.PublicationCompletionProjection{}).MarshalJSON},
		{"update request payload", (distribution.UpdateRequestPayload{}).MarshalJSON},
		{"update request document", (distribution.UpdateRequestDocument{}).MarshalJSON},
		{"update response payload", (distribution.UpdateResponsePayload{}).MarshalJSON},
		{"update response document", (distribution.UpdateResponseDocument{}).MarshalJSON},
		{"upgrade request payload", (distribution.UpgradeRequestPayload{}).MarshalJSON},
		{"upgrade request document", (distribution.UpgradeRequestDocument{}).MarshalJSON},
		{"upgrade grant payload", (distribution.UpgradeGrantPayload{}).MarshalJSON},
		{"upgrade grant projection", (distribution.UpgradeGrantProjection{}).MarshalJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.marshal()
			if got != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrDistributionContract) {
				t.Fatalf("MarshalJSON()=(%d bytes,%v), want nil and JSON/Distribution refusal", len(got), err)
			}
		})
	}
}

func TestGrantProjectionRefusesForeignDomainAndCapability(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	u := newUpgradeExchangeFixture(t)
	otherUpload, _ := uploadCapabilityProjection(t, 100)
	otherDownload, _ := downloadCapabilityProjection(t, 100)
	cases := []struct {
		name    string
		marshal func() ([]byte, error)
		wantErr error
	}{
		{"publication grant foreign domain", func() ([]byte, error) {
			v := p.grantProjection
			v.Attestation.Domain = distribution.SigningDomainUpgradeGrantV1
			return v.MarshalJSON()
		}, core.ErrDistributionBinding},
		{"publication grant foreign capability", func() ([]byte, error) {
			v := p.grantProjection
			v.Capabilities[0] = otherUpload
			return v.MarshalJSON()
		}, core.ErrDistributionBinding},
		{"upgrade grant foreign domain", func() ([]byte, error) {
			v := u.grantProjection
			v.Attestation.Domain = distribution.SigningDomainPublicationGrantV1
			return v.MarshalJSON()
		}, core.ErrDistributionBinding},
		{"upgrade grant foreign capability", func() ([]byte, error) { v := u.grantProjection; v.Capability = otherDownload; return v.MarshalJSON() }, core.ErrDistributionBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.marshal()
			if got != nil || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("MarshalJSON()=(%d,%v), want nil and %v", len(got), err, tc.wantErr)
			}
		})
	}
}
