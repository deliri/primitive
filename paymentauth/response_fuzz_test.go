package paymentauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
	"testing"
)

func FuzzPaymentResponseAuthorityClosure(f *testing.F) {
	fixture := newPaymentResponseFixture(f)
	projection, err := IssueResponse(ResponseIssuance{Server: fixture.server, Signer: fixture.signer, Header: fixture.header, Body: fixture.body, Assessment: acceptedPaymentResponseAssessment(f, fixture.header)})
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	wantBody, err := fixture.body.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{fixture.body.Attestation.Signer}})
	if err != nil {
		f.Fatal(err)
	}
	foreign := newPaymentResponseFixtureWithMarkers(f, 0x91, 0x92)
	for selector := range uint8(5) {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("{}"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		expected := fixture.expected
		client := fixture.client
		wantErr := error(nil)
		switch selector % 5 {
		case 0:
		case 1:
			data = canonical
			expected.RequestNonce = paymentQueryNonce(t, 0x72)
			wantErr = core.ErrControlPlaneResponseBinding
		case 2:
			data = canonical
			expected.Family = controlwire.RouteFamilyChits
			wantErr = core.ErrControlPlaneResponseBinding
		case 3:
			data = canonical
			client = foreign.client
			wantErr = core.ErrAttestVerification
		case 4:
			data = canonical
			expected.Account = paymentQueryAccount(t, 0x73)
			wantErr = core.ErrControlPlaneResponseBinding
		}
		if selector%5 != 0 && expected == fixture.expected && selector%5 != 3 {
			t.Fatal("binding mutation = unchanged, want a changed bound fact")
		}
		var document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
		if err := document.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("seed decode error = %v, want nil", err)
		}
		if err := document.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || bytes.Equal(data, canonical) {
				t.Fatalf("response decode = %v, want typed refusal only for changed bytes", err)
			}
			if !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("response decode error = %v, want typed response identity", err)
			}
			preserved, preservedErr := VerifyResponse(ResponseVerification{Client: fixture.client, Document: document, Expected: fixture.expected})
			if preservedErr != nil {
				t.Fatalf("refused decode changed receiver: error = %v, want nil", preservedErr)
			}
			proveExactPaymentCatalog(t, fixture, preserved)
			return
		}
		proof, err := VerifyResponse(ResponseVerification{Client: client, Document: document, Expected: expected})
		if err != nil {
			if !errors.Is(err, core.ErrControlPlaneContract) || (bytes.Equal(data, canonical) && !errors.Is(err, wantErr)) {
				t.Fatalf("response verification = %v, want %v with boundary identity", err, wantErr)
			}
			header, headerErr := proof.Header()
			_, bodyErr := proof.Body()
			if header != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("refused proof = %v/%v/%v, want zero and typed refusal", header, headerErr, bodyErr)
			}
			return
		}
		if wantErr != nil {
			t.Fatalf("forced mutation error = nil, want %v", wantErr)
		}
		header, err := proof.Header()
		if err != nil || header != fixture.header {
			t.Fatalf("authenticated header = %v/%v, want exact signed seed", header, err)
		}
		body, err := proof.Body()
		if err != nil {
			t.Fatal(err)
		}
		bodyJSON, err := body.MarshalJSON()
		if err != nil || !bytes.Equal(bodyJSON, wantBody) {
			t.Fatalf("authenticated catalog = %d bytes/%v, want exact %d-byte signed seed", len(bodyJSON), err, len(wantBody))
		}
		verified, err := payment.VerifyCatalog(payment.CatalogVerification{Document: body, Request: fixture.request, TrustedKeys: trusted})
		if err != nil || verified.Validate() != nil {
			t.Fatalf("independent catalog authentication = %v, want valid proof", err)
		}
		proveExactPaymentCatalog(t, fixture, proof)
	})
}
