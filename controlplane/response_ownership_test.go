package controlplane_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// A Body accessor is an ownership crossing: changing one returned body must not
// rewrite a previous authentication or a sibling proof from the same document.
func TestVerifiedResponseOwnsAuthenticatedBodyAcrossMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *controlplane.RegistrationDocument)
	}{
		{name: "certificate build mutation remains structurally valid", mutate: func(t *testing.T, body *controlplane.RegistrationDocument) {
			original := body.Payload.Certificate.Body.Build
			replacement, err := core.NewBuildIdentity(core.BuildIdentityRequest{Offering: original.Offering(), Version: core.NewReleaseVersion(2026, 1, 99), Commit: original.Commit(), Platform: original.Platform()})
			if err != nil || replacement == original {
				t.Fatalf("replacement build = (%v, %v), want distinct valid build", replacement, err)
			}
			body.Payload.Certificate.Body.Build = replacement
		}},
		{name: "destroyed returned certificate cannot invalidate sibling proof", mutate: func(_ *testing.T, body *controlplane.RegistrationDocument) {
			*body.Payload.Certificate = controlplane.InstallationCertificateDocument{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := authenticatedResponseForTest(t, 30)
			document := decodeAuthenticatedResponse(t, fixture.canonical)
			request := controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Client: fixture.client, Document: document, Expected: fixture.expected}
			proof, err := controlplane.VerifyResponse(request)
			if err != nil {
				t.Fatalf("VerifyResponse() error = %v, want nil", err)
			}
			sibling, err := controlplane.VerifyResponse(request)
			if err != nil {
				t.Fatalf("VerifyResponse(sibling) error = %v, want nil", err)
			}
			body, err := proof.Body()
			if err != nil || body.Payload.Certificate == nil {
				t.Fatalf("Body() = (%v, %v), want certificate", body, err)
			}
			want, err := body.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON(authenticated) error = %v, want nil", err)
			}
			before := *body.Payload.Certificate
			tc.mutate(t, &body)
			if *body.Payload.Certificate == before {
				t.Fatal("certificate mutation = unchanged, want changed signed fact")
			}
			for _, observed := range []struct {
				name  string
				proof controlplane.VerifiedResponse[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
			}{{"original", proof}, {"sibling", sibling}} {
				got, err := observed.proof.Body()
				if err != nil {
					t.Errorf("%s Body() error = %v, want nil after external mutation", observed.name, err)
					continue
				}
				encoded, err := got.MarshalJSON()
				if err != nil || !bytes.Equal(encoded, want) {
					t.Errorf("%s authenticated bytes = (%d bytes, %v), want exact %d bytes", observed.name, len(encoded), err, len(want))
				}
			}
			again, err := controlplane.VerifyResponse(request)
			if err != nil {
				t.Fatalf("VerifyResponse(original document) error = %v, want nil", err)
			}
			got, err := again.Body()
			if err != nil {
				t.Fatalf("Body(reverified) error = %v, want nil", err)
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, want) {
				t.Fatalf("reverified bytes = (%d bytes, %v), want exact %d bytes", len(encoded), err, len(want))
			}
		})
	}
}
