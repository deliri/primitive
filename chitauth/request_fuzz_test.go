package chitauth

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
)

func FuzzCredentialedChitQueryJSONSemanticAndAuthorityClosure(f *testing.F) {
	request := standardQueryFixtureRequest(f)
	request.selection = querySpecificSelection(f)
	fixture := newQueryFixture(f, request)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("RequestDocument.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))
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
		if err != nil {
			t.Fatalf("RequestDocument.MarshalJSON(accepted) = (%d bytes, %v), want canonical output and nil",
				len(encoded), err)
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
		query, queryErr := chit.VerifyQuery(chit.QueryVerification{
			Document: roundTrip.Request, TrustedKeys: deviceKeys,
		})
		if queryErr != nil {
			t.Fatalf("chit.VerifyQuery(independent authenticated fuzz oracle) error = %v, want nil", queryErr)
		}
		gotPayload, payloadErr := query.Payload()
		if payloadErr != nil || gotPayload != fixture.payload {
			t.Fatalf("independent chit query payload = (%v, %v), want (%v, nil)",
				gotPayload, payloadErr, fixture.payload)
		}
	})
}

// The selector guarantees that structurally valid but unauthenticated signed
// states reach the real verifier, rather than mutating only JSON framing.
func FuzzCredentialedChitQuerySignedMutationClosure(f *testing.F) {
	fixture := newQueryFixture(f, standardQueryFixtureRequest(f))
	for selector := range uint8(4) {
		f.Add(selector)
	}
	f.Fuzz(func(t *testing.T, selector uint8) {
		document := fixture.document
		wantErr := error(nil)
		switch selector % 4 {
		case 0:
		case 1:
			document.Request.Payload.Nonce = queryNonce(t, 0x7e)
			wantErr = core.ErrAttestVerification
		case 2:
			document.Request.Payload.Query.Selection = querySpecificSelection(t)
			wantErr = core.ErrAttestVerification
		case 3:
			document.Request.Attestation.BodySHA256 = core.SHA256Of([]byte("foreign query digest"))
			wantErr = core.ErrAttestVerification
		}
		if selector%4 != 0 && document == fixture.document {
			t.Fatal("signed mutation = unchanged, want one changed semantic fact")
		}
		wire, err := document.MarshalJSON()
		if err != nil {
			t.Fatalf("structural mutation MarshalJSON = %v, want nil before authentication", err)
		}
		var received RequestDocument
		if err := received.UnmarshalJSON(wire); err != nil || received != document {
			t.Fatalf("signed mutation decode = %v, want exact typed mutation", err)
		}
		proof, err := Verify(Verification{Server: fixture.server, Document: received})
		if wantErr != nil {
			if !errors.Is(err, wantErr) || proof != (Verified{}) {
				t.Fatalf("signed mutation Verify = %v/%v, want zero/%v", proof, err, wantErr)
			}
			return
		}
		payload, payloadErr := proof.Payload()
		if err != nil || payloadErr != nil || payload != fixture.payload {
			t.Fatalf("signed baseline = %v/%v, want exact authenticated payload", err, payloadErr)
		}
	})
}
