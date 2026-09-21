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
		marshal func() ([]byte, error)
		name    string
	}{
		{name: "signing domain", marshal: distribution.SigningDomainUnknown.MarshalJSON},
		{name: "request commitment", marshal: (distribution.RequestCommitment{}).MarshalJSON},
		{name: "publication request payload", marshal: (distribution.PublicationRequestPayload{}).MarshalJSON},
		{name: "publication request document", marshal: (distribution.PublicationRequestDocument{}).MarshalJSON},
		{name: "publication grant payload", marshal: (distribution.PublicationGrantPayload{}).MarshalJSON},
		{name: "publication grant projection", marshal: (distribution.PublicationGrantProjection{}).MarshalJSON},
		{name: "publication completion payload", marshal: (distribution.PublicationCompletionPayload{}).MarshalJSON},
		{name: "publication completion document", marshal: (distribution.PublicationCompletionDocument{}).MarshalJSON},
		{name: "publication completion projection", marshal: (distribution.PublicationCompletionProjection{}).MarshalJSON},
		{name: "update request payload", marshal: (distribution.UpdateRequestPayload{}).MarshalJSON},
		{name: "update request document", marshal: (distribution.UpdateRequestDocument{}).MarshalJSON},
		{name: "update response payload", marshal: (distribution.UpdateResponsePayload{}).MarshalJSON},
		{name: "update response document", marshal: (distribution.UpdateResponseDocument{}).MarshalJSON},
		{name: "upgrade request payload", marshal: (distribution.UpgradeRequestPayload{}).MarshalJSON},
		{name: "upgrade request document", marshal: (distribution.UpgradeRequestDocument{}).MarshalJSON},
		{name: "upgrade grant payload", marshal: (distribution.UpgradeGrantPayload{}).MarshalJSON},
		{name: "upgrade grant projection", marshal: (distribution.UpgradeGrantProjection{}).MarshalJSON},
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
		wantErr error
		marshal func() ([]byte, error)
		name    string
	}{
		{name: "publication grant foreign domain", marshal: func() ([]byte, error) {
			v := p.grantProjection
			v.Attestation.Domain = distribution.SigningDomainUpgradeGrantV1
			return v.MarshalJSON()
		}, wantErr: core.ErrDistributionBinding},
		{name: "publication grant foreign capability", marshal: func() ([]byte, error) {
			v := p.grantProjection
			v.Capabilities[0] = otherUpload
			return v.MarshalJSON()
		}, wantErr: core.ErrDistributionBinding},
		{name: "upgrade grant foreign domain", marshal: func() ([]byte, error) {
			v := u.grantProjection
			v.Attestation.Domain = distribution.SigningDomainPublicationGrantV1
			return v.MarshalJSON()
		}, wantErr: core.ErrDistributionBinding},
		{name: "upgrade grant foreign capability", marshal: func() ([]byte, error) { v := u.grantProjection; v.Capability = otherDownload; return v.MarshalJSON() }, wantErr: core.ErrDistributionBinding},
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
