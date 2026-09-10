package submissionauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

func TestCredentialNominationLayerTriad(t *testing.T) {
	t.Parallel()
	base := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	foreign := newAuthFixture(t, authFixtureRequest{deviceByte: 0x69})
	if base.request.certificate.Body.Build != foreign.certificate.Body.Build || base.request.certificate.Body.DeviceKey == foreign.certificate.Body.DeviceKey {
		t.Fatalf("nominee mutation = %v, want same build and different key from %v", foreign.certificate.Body, base.request.certificate.Body)
	}
	for _, tc := range []struct {
		name        string
		certificate controlplane.InstallationCertificateDocument
		wantErr     error
	}{
		{"exact_nominated_device", base.request.certificate, nil},
		{"same_build_foreign_nominated_device", foreign.certificate, core.ErrControlPlaneResponseBinding},
		{"absent_certificate_cannot_nominate", controlplane.InstallationCertificateDocument{}, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requestAssembly := RequestAssembly{Request: base.request.request, Certificate: tc.certificate}
			completionAssembly := CompletionAssembly{Completion: base.completionDocument, Certificate: tc.certificate}
			projectionAssembly := CompletionProjectionAssembly{Completion: base.completionProjection, Certificate: tc.certificate}
			for _, door := range []struct {
				name string
				run  func() (bool, error)
			}{
				{"request_assembly", func() (bool, error) { got, err := Assemble(requestAssembly); return got == (RequestDocument{}), err }},
				{"request_validation", func() (bool, error) { return true, RequestDocument(requestAssembly).Validate() }},
				{"completion_assembly", func() (bool, error) {
					got, err := AssembleCompletion(completionAssembly)
					return got == (CompletionDocument{}), err
				}},
				{"completion_validation", func() (bool, error) { return true, CompletionDocument(completionAssembly).Validate() }},
				{"projection_assembly", func() (bool, error) {
					got, err := AssembleCompletionProjection(projectionAssembly)
					return got == (CompletionProjection{}), err
				}},
			} {
				t.Run(door.name, func(t *testing.T) {
					t.Parallel()
					zero, err := door.run()
					if !errors.Is(err, tc.wantErr) || err != nil && (!zero || !errors.Is(err, core.ErrControlPlaneContract)) {
						t.Fatalf("zero=%t error=%v, want refusal leaves zero and %v", zero, err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestCredentialRouteLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	for _, tc := range []struct {
		name       string
		request    RequestDocument
		completion CompletionDocument
		wantErr    error
	}{
		{"exact_credential_routes", fixture.request.document, fixture.credentialed, nil},
		{"missing_certificates", RequestDocument{Request: fixture.request.request}, CompletionDocument{Completion: fixture.completionDocument}, core.ErrControlPlaneContract},
		{"absent_documents", RequestDocument{}, CompletionDocument{}, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, door := range []struct {
				name   string
				route  func() (controlwire.RouteContract, error)
				family controlwire.RouteFamily
			}{
				{"request", tc.request.ControlRoute, controlwire.RouteFamilySubmissions},
				{"completion", tc.completion.ControlRoute, controlwire.RouteFamilySubmissionCompletions},
			} {
				t.Run(door.name, func(t *testing.T) {
					t.Parallel()
					got, err := door.route()
					if !errors.Is(err, tc.wantErr) || err != nil && got != (controlwire.RouteContract{}) {
						t.Fatalf("%s route=%v error=%v, want zero on %v", door.name, got, err, tc.wantErr)
					}
					if err == nil && (got.Offering() != fixture.request.request.Payload.Build.Offering() || got.Family() != door.family) {
						t.Fatalf("route=%v, want exact offering/family %v", got, door.family)
					}
				})
			}
			if tc.request.ControlRevision() != tc.request.Request.Payload.Revision || tc.completion.ControlRevision() != tc.completion.Certificate.Body.Revision {
				t.Fatalf("control revisions=%v/%v, want exact signed %v/%v", tc.request.ControlRevision(), tc.completion.ControlRevision(), tc.request.Request.Payload.Revision, tc.completion.Certificate.Body.Revision)
			}
		})
	}
}
func TestCompletionProjectionSignerLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	for _, tc := range []struct {
		name       string
		projection submission.CompletionProjection
		want       core.Ed25519PublicKey
		wantErr    error
	}{
		{"issued_key_is_exact", fixture.completionProjection, fixture.request.certificate.Body.DeviceKey, nil},
		{"absent_projection_has_no_signer", submission.CompletionProjection{}, core.Ed25519PublicKey{}, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.projection.Signer()
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("signer=%v error=%v, want %v/%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
func TestCredentialOutputRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		run     func() (bool, error)
		wantErr error
	}{
		{"absent_request_cannot_encode", func() (bool, error) { data, err := (RequestDocument{}).MarshalJSON(); return data == nil, err }, core.ErrJSONContract},
		{"absent_completion_cannot_encode", func() (bool, error) { data, err := (CompletionDocument{}).MarshalJSON(); return data == nil, err }, core.ErrJSONContract},
		{"absent_projection_cannot_encode", func() (bool, error) { data, err := (CompletionProjection{}).MarshalJSON(); return data == nil, err }, core.ErrJSONContract},
		{"absent_verified_request_cannot_expose_document", func() (bool, error) { got, err := (Verified{}).Document(); return got == (RequestDocument{}), err }, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			zero, err := tc.run()
			if !zero || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("zero=%t error=%v, want zero and %v", zero, err, tc.wantErr)
			}
		})
	}
}
