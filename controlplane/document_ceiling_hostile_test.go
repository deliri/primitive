package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type boundedJSONDocument interface {
	core.ValidatedJSONMarshaler
	UnmarshalJSON([]byte) error
}

// Padding a genuine typed document with permitted whitespace isolates its byte
// ceiling. An unknown member would reject even if production removed the limit.
func TestEveryDocumentCeilingOwnsBothSidesOfItsBoundary(t *testing.T) {
	t.Parallel()
	registration := issueTestRegistration(t)
	checkIn := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
	response := issueTestCheckInResponse(t)
	registrationRequest := registrationRequestFixture(t)
	commitment := commitmentFixture(t)
	doors := []struct {
		name    string
		ceiling int
		source  core.ValidatedJSONMarshaler
		fresh   func() boundedJSONDocument
	}{
		{"check-in payload", controlplane.CheckInPayloadJSONMaximumBytes, checkIn.request.Payload, func() boundedJSONDocument { return new(controlplane.CheckInPayload) }},
		{"check-in request", controlplane.CheckInRequestJSONMaximumBytes, checkIn.request, func() boundedJSONDocument { return new(controlplane.CheckInRequest) }},
		{"check-in response payload", controlplane.CheckInResponsePayloadJSONMaximumBytes, response.document.Payload, func() boundedJSONDocument { return new(controlplane.CheckInResponsePayload) }},
		{"check-in response document", controlplane.CheckInResponseDocumentJSONMaximumBytes, response.document, func() boundedJSONDocument { return new(controlplane.CheckInResponseDocument) }},
		{"response header", controlplane.ResponseHeaderJSONMaximumBytes, registration.document.Payload.Header, func() boundedJSONDocument { return new(controlplane.ResponseHeader) }},
		{"registration request", controlplane.RegistrationRequestJSONMaximumBytes, registrationRequest, func() boundedJSONDocument { return new(controlplane.RegistrationRequest) }},
		{"certificate body", controlplane.InstallationCertificateBodyJSONMaximumBytes, checkIn.certificate.Body, func() boundedJSONDocument { return new(controlplane.InstallationCertificateBody) }},
		{"certificate document", controlplane.InstallationCertificateDocumentJSONMaximumBytes, checkIn.certificate, func() boundedJSONDocument { return new(controlplane.InstallationCertificateDocument) }},
		{"registration payload", controlplane.RegistrationPayloadJSONMaximumBytes, registration.document.Payload, func() boundedJSONDocument { return new(controlplane.RegistrationPayload) }},
		{"registration document", controlplane.RegistrationDocumentJSONMaximumBytes, registration.document, func() boundedJSONDocument { return new(controlplane.RegistrationDocument) }},
		{"usage watermark", controlplane.UsageWatermarkJSONMaximumBytes, checkIn.request.Payload.PreviousWatermark, func() boundedJSONDocument { return new(controlplane.UsageWatermark) }},
		{"usage window", controlplane.UsageWindowJSONMaximumBytes, checkIn.request.Payload.Window, func() boundedJSONDocument { return new(controlplane.UsageWindow) }},
		{"response commitment", controlplane.ResponseCommitmentJSONMaximumBytes, commitment, func() boundedJSONDocument { return new(controlplane.ResponseCommitment) }},
	}
	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := door.source.MarshalJSON()
			if err != nil || len(canonical) >= door.ceiling-1 {
				t.Fatalf("canonical fixture = (%d bytes, %v), want below %d", len(canonical), err, door.ceiling-1)
			}
			for _, tc := range []struct {
				name    string
				size    int
				wantErr error
			}{
				{name: "one below ceiling", size: door.ceiling - 1},
				{name: "exact ceiling", size: door.ceiling},
				{name: "one above ceiling", size: door.ceiling + 1, wantErr: core.ErrJSONContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					data := append(bytes.Repeat([]byte{' '}, tc.size-len(canonical)), canonical...)
					got := door.fresh()
					if err := got.UnmarshalJSON(canonical); err != nil {
						t.Fatalf("UnmarshalJSON(initial) error = %v, want nil", err)
					}
					gotErr := got.UnmarshalJSON(data)
					if !errors.Is(gotErr, tc.wantErr) {
						t.Errorf("UnmarshalJSON(%d bytes) error = %v, want %v", len(data), gotErr, tc.wantErr)
					}
					if tc.wantErr != nil && !errors.Is(gotErr, core.ErrControlPlaneContract) {
						t.Errorf("oversized error = %v, want package identity", gotErr)
					}
					projected, err := got.MarshalJSON()
					if err != nil || !bytes.Equal(projected, canonical) {
						t.Fatalf("receiver projection = (%d bytes, %v), want exact %d bytes", len(projected), err, len(canonical))
					}
				})
			}
		})
	}
}
