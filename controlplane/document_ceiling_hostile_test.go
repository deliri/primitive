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
		source  core.ValidatedJSONMarshaler
		fresh   func() boundedJSONDocument
		name    string
		ceiling int
	}{
		{name: "check-in payload", ceiling: controlplane.CheckInPayloadJSONMaximumBytes, source: checkIn.request.Payload, fresh: func() boundedJSONDocument { return new(controlplane.CheckInPayload) }},
		{name: "check-in request", ceiling: controlplane.CheckInRequestJSONMaximumBytes, source: checkIn.request, fresh: func() boundedJSONDocument { return new(controlplane.CheckInRequest) }},
		{name: "check-in response payload", ceiling: controlplane.CheckInResponsePayloadJSONMaximumBytes, source: response.document.Payload, fresh: func() boundedJSONDocument { return new(controlplane.CheckInResponsePayload) }},
		{name: "check-in response document", ceiling: controlplane.CheckInResponseDocumentJSONMaximumBytes, source: response.document, fresh: func() boundedJSONDocument { return new(controlplane.CheckInResponseDocument) }},
		{name: "response header", ceiling: controlplane.ResponseHeaderJSONMaximumBytes, source: registration.document.Payload.Header, fresh: func() boundedJSONDocument { return new(controlplane.ResponseHeader) }},
		{name: "registration request", ceiling: controlplane.RegistrationRequestJSONMaximumBytes, source: registrationRequest, fresh: func() boundedJSONDocument { return new(controlplane.RegistrationRequest) }},
		{name: "certificate body", ceiling: controlplane.InstallationCertificateBodyJSONMaximumBytes, source: checkIn.certificate.Body, fresh: func() boundedJSONDocument { return new(controlplane.InstallationCertificateBody) }},
		{name: "certificate document", ceiling: controlplane.InstallationCertificateDocumentJSONMaximumBytes, source: checkIn.certificate, fresh: func() boundedJSONDocument { return new(controlplane.InstallationCertificateDocument) }},
		{name: "registration payload", ceiling: controlplane.RegistrationPayloadJSONMaximumBytes, source: registration.document.Payload, fresh: func() boundedJSONDocument { return new(controlplane.RegistrationPayload) }},
		{name: "registration document", ceiling: controlplane.RegistrationDocumentJSONMaximumBytes, source: registration.document, fresh: func() boundedJSONDocument { return new(controlplane.RegistrationDocument) }},
		{name: "usage watermark", ceiling: controlplane.UsageWatermarkJSONMaximumBytes, source: checkIn.request.Payload.PreviousWatermark, fresh: func() boundedJSONDocument { return new(controlplane.UsageWatermark) }},
		{name: "usage window", ceiling: controlplane.UsageWindowJSONMaximumBytes, source: checkIn.request.Payload.Window, fresh: func() boundedJSONDocument { return new(controlplane.UsageWindow) }},
		{name: "response commitment", ceiling: controlplane.ResponseCommitmentJSONMaximumBytes, source: commitment, fresh: func() boundedJSONDocument { return new(controlplane.ResponseCommitment) }},
	}
	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := door.source.MarshalJSON()
			if err != nil || len(canonical) >= door.ceiling-1 {
				t.Fatalf("canonical fixture = (%d bytes, %v), want below %d", len(canonical), err, door.ceiling-1)
			}
			for _, tc := range []struct {
				wantErr error
				name    string
				size    int
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
