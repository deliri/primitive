package chitauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func FuzzChitResponseAuthorityClosure(f *testing.F) {
	fixture := newChitResponseFixture(f)
	requestSpec := standardQueryFixtureRequest(f)
	requestSpec.authorityByte = 0x81
	requestSpec.deviceByte = 0x82
	request := newQueryFixture(f, requestSpec)
	projection, err := IssueResponse(ResponseIssuance{Server: fixture.server, Signer: fixture.signer, Header: fixture.header, Body: fixture.body, Assessment: acceptedChitResponseAssessment(f, fixture.header)})
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
	foreign := newChitResponseFixtureWithMarkers(f, 0x91, 0x92)
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
			expected.RequestNonce = queryNonce(t, 0x72)
			wantErr = core.ErrControlPlaneResponseBinding
		case 2:
			data = canonical
			expected.Family = controlwire.RouteFamilyPayments
			wantErr = core.ErrControlPlaneResponseBinding
		case 3:
			data = canonical
			client = foreign.client
			wantErr = core.ErrAttestVerification
		case 4:
			data = canonical
			expected.Account = queryAccount(t, 0x73)
			wantErr = core.ErrControlPlaneResponseBinding
		}
		if selector%5 != 0 && expected == fixture.expected && selector%5 != 3 {
			t.Fatal("binding mutation = unchanged, want a changed bound fact")
		}
		var document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
		if err := document.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || bytes.Equal(data, canonical) {
				t.Fatalf("response decode = %v, want typed refusal only for changed bytes", err)
			}
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
		verified, err := chit.VerifyCatalog(chit.CatalogVerification{Document: body, Request: request.payload, TrustedKeys: trusted})
		if err != nil || verified.Validate() != nil {
			t.Fatalf("independent catalog authentication = %v, want valid proof", err)
		}
	})
}
