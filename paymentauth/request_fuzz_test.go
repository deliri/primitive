package paymentauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
)

func FuzzCredentialedPaymentQueryJSONSemanticAndAuthorityClosure(f *testing.F) {
	request := standardPaymentQueryFixtureRequest(f)
	request.selection = paymentQuerySpecificSelection(f)
	fixture := newPaymentQueryFixture(f, request)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("RequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))
	f.Add(paymentQueryJSONAtLength(f, canonical, RequestDocumentJSONMaximumBytes-1))
	f.Add(paymentQueryJSONAtLength(f, canonical, RequestDocumentJSONMaximumBytes))
	f.Add(paymentQueryJSONAtLength(f, canonical, RequestDocumentJSONMaximumBytes+1))
	f.Add(append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"future":true}`)...))
	f.Add(append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"request":null}`)...))
	f.Add([]byte(`{"request":true,"certificate":null}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var got RequestDocument
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			preserved := fixture.document
			preservedErr := preserved.UnmarshalJSON(data)
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrControlPlaneContract) || got != (RequestDocument{}) ||
				!errors.Is(preservedErr, core.ErrJSONContract) || preserved != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(rejected) = (zero %v, zero error %v, preserved %v, preserved error %v), want zero/preserved and typed rejection",
					got, gotErr, preserved, preservedErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("RequestDocument.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RequestDocumentJSONMaximumBytes {
			t.Fatalf("RequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, RequestDocumentJSONMaximumBytes)
		}
		var roundTrip RequestDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("RequestDocument canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("RequestDocument second canonical projection = (%d bytes, %v), want byte-identical %d bytes and nil",
				len(second), err, len(encoded))
		}
		verified, verifyErr := Verify(Verification{Server: fixture.server, Document: roundTrip})
		if verifyErr != nil {
			if !errors.Is(verifyErr, core.ErrAttestVerification) || verified != (Verified{}) {
				t.Fatalf("Verify(fuzzed credential) = (%v, %v), want zero typed attestation rejection", verified, verifyErr)
			}
			return
		}
		if roundTrip != fixture.document {
			t.Fatalf("Verify authenticated a credentialed query other than the compiler-owned signed fixture")
		}
		certificate, certificateErr := fixture.server.VerifyInstallationCertificate(roundTrip.Certificate)
		if certificateErr != nil {
			t.Fatalf("Server.VerifyInstallationCertificate(authenticated fuzz input) error = %v, want nil", certificateErr)
		}
		deviceKeys, keysErr := certificate.DeviceKeys()
		if keysErr != nil {
			t.Fatalf("VerifiedInstallationCertificate.DeviceKeys() error = %v, want nil", keysErr)
		}
		query, queryErr := payment.VerifyQuery(payment.QueryVerification{
			Document: roundTrip.Request, TrustedKeys: deviceKeys,
		})
		if queryErr != nil {
			t.Fatalf("payment.VerifyQuery(independent authenticated fuzz oracle) error = %v, want nil", queryErr)
		}
		gotPayload, payloadErr := query.Payload()
		if payloadErr != nil || gotPayload != fixture.payload {
			t.Fatalf("independent payment query payload = (%v, %v), want (%v, nil)",
				gotPayload, payloadErr, fixture.payload)
		}
	})
}
