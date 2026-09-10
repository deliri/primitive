package submission

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// The same binding mismatch crosses a different boundary when either signature
// is unauthenticated: rejection must not classify untrusted signed facts.
func TestCompletionAuthenticationOrderLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("authentication before binding"), 0x10)
	document := receiveIssuedCompletion(t, fixture)
	foreignKey, _ := testSigningKey(t, 0x22)
	foreignTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{foreignKey}})
	if err != nil {
		t.Fatal(err)
	}
	foreignNonce := testRequestPayload(t, grantFixtureRequest{requestNonceByte: 0x23}).Nonce
	for _, tc := range []struct {
		name        string
		binding     bool
		grantTrust  bool
		deviceTrust bool
		tamper      bool
		zero        bool
		wantErr     error
	}{
		{"authentic_exact", false, false, false, false, false, nil},
		{"authentic_nonce_mismatch", true, false, false, false, false, core.ErrControlPlaneResponseBinding},
		{"untrusted_grant_exact", false, true, false, false, false, core.ErrAttestVerification},
		{"untrusted_grant_mismatch", true, true, false, false, false, core.ErrAttestVerification},
		{"untrusted_device_exact", false, false, true, false, false, core.ErrAttestVerification},
		{"untrusted_device_mismatch", true, false, true, false, false, core.ErrAttestVerification},
		{"tampered_signed_nonce", false, false, false, true, false, core.ErrAttestVerification},
		{"neutral_document", false, false, false, false, true, core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := CompletionExpectation{Document: document, Request: fixture.request, Grant: fixture.grantDocument, GrantKeys: fixture.grantKeys, CompletionKeys: fixture.deviceKeys, Nonce: fixture.nonce}
			if tc.binding {
				request.Nonce = foreignNonce
			}
			if tc.grantTrust {
				request.GrantKeys = foreignTrust
			}
			if tc.deviceTrust {
				request.CompletionKeys = foreignTrust
			}
			if tc.tamper {
				request.Document.Payload.Nonce = foreignNonce
			}
			if tc.zero {
				request.Document = CompletionDocument{}
			}
			got, err := VerifyCompletion(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyCompletion error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (VerifiedCompletion{}) {
					t.Fatalf("got completion proof=%v, want zero after refusal", got)
				}
				if errors.Is(err, core.ErrAttestVerification) != errors.Is(tc.wantErr, core.ErrAttestVerification) || errors.Is(err, core.ErrControlPlaneResponseBinding) != errors.Is(tc.wantErr, core.ErrControlPlaneResponseBinding) {
					t.Fatalf("refusal mixes authentication and binding identities: %v", err)
				}
				return
			}
			payload, err := got.Payload()
			if err != nil || payload != document.Payload {
				t.Fatalf("verified payload differs=%t error=%v", payload != document.Payload, err)
			}
		})
	}
}
