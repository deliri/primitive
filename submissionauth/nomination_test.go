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
		wantErr     error
		name        string
		certificate controlplane.InstallationCertificateDocument
	}{
		{name: "exact_nominated_device", certificate: base.request.certificate, wantErr: nil},
		{name: "same_build_foreign_nominated_device", certificate: foreign.certificate, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "absent_certificate_cannot_nominate", certificate: controlplane.InstallationCertificateDocument{}, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requestAssembly := RequestAssembly{Request: base.request.request, Certificate: tc.certificate}
			completionAssembly := CompletionAssembly{Completion: base.completionDocument, Certificate: tc.certificate}
			projectionAssembly := CompletionProjectionAssembly{Completion: base.completionProjection, Certificate: tc.certificate}
			for _, door := range []struct {
				run  func() (bool, error)
				name string
			}{
				{name: "request_assembly", run: func() (bool, error) { got, err := Assemble(requestAssembly); return got == (RequestDocument{}), err }},
				{name: "request_validation", run: func() (bool, error) { return true, RequestDocument(requestAssembly).Validate() }},
				{name: "completion_assembly", run: func() (bool, error) {
					got, err := AssembleCompletion(completionAssembly)
					return got == (CompletionDocument{}), err
				}},
				{name: "completion_validation", run: func() (bool, error) { return true, CompletionDocument(completionAssembly).Validate() }},
				{name: "projection_assembly", run: func() (bool, error) {
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
		wantErr    error
		name       string
		request    RequestDocument
		completion CompletionDocument
	}{
		{name: "exact_credential_routes", request: fixture.request.document, completion: fixture.credentialed, wantErr: nil},
		{name: "missing_certificates", request: RequestDocument{Request: fixture.request.request}, completion: CompletionDocument{Completion: fixture.completionDocument}, wantErr: core.ErrControlPlaneContract},
		{name: "absent_documents", request: RequestDocument{}, completion: CompletionDocument{}, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, door := range []struct {
				route  func() (controlwire.RouteContract, error)
				name   string
				family controlwire.RouteFamily
			}{
				{name: "request", route: tc.request.ControlRoute, family: controlwire.RouteFamilySubmissions},
				{name: "completion", route: tc.completion.ControlRoute, family: controlwire.RouteFamilySubmissionCompletions},
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
		wantErr    error
		name       string
		projection submission.CompletionProjection
		want       core.Ed25519PublicKey
	}{
		{name: "issued_key_is_exact", projection: fixture.completionProjection, want: fixture.request.certificate.Body.DeviceKey, wantErr: nil},
		{name: "absent_projection_has_no_signer", projection: submission.CompletionProjection{}, want: core.Ed25519PublicKey{}, wantErr: core.ErrControlPlaneContract},
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
		wantErr error
		run     func() (bool, error)
		name    string
	}{
		{name: "absent_request_cannot_encode", run: func() (bool, error) { data, err := (RequestDocument{}).MarshalJSON(); return data == nil, err }, wantErr: core.ErrJSONContract},
		{name: "absent_completion_cannot_encode", run: func() (bool, error) { data, err := (CompletionDocument{}).MarshalJSON(); return data == nil, err }, wantErr: core.ErrJSONContract},
		{name: "absent_projection_cannot_encode", run: func() (bool, error) { data, err := (CompletionProjection{}).MarshalJSON(); return data == nil, err }, wantErr: core.ErrJSONContract},
		{name: "absent_verified_request_cannot_expose_document", run: func() (bool, error) { got, err := (Verified{}).Document(); return got == (RequestDocument{}), err }, wantErr: core.ErrControlPlaneContract},
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
