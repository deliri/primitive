package retrievalauth

import (
	"errors"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
	"testing"
)

func TestResponseSocketLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newResponseFixture(t, 0x21)
	t.Run("signed grant retains manifest membership and capability", func(t *testing.T) {
		t.Parallel()
		var document controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
		if err := document.UnmarshalJSON(fixture.canonical); err != nil {
			t.Fatalf("response decode error = %v, want nil", err)
		}
		proof, err := VerifyResponse(ResponseVerification{Client: fixture.client, Document: document, Expected: fixture.expectation})
		if err != nil {
			t.Fatalf("VerifyResponse error = %v, want nil", err)
		}
		fixture.prove(t, proof)
	})
	families := []struct {
		name   string
		family controlwire.RouteFamily
	}{
		{"registration", controlwire.RouteFamilyRegistrations}, {"check-in", controlwire.RouteFamilyCheckIns},
		{"submission", controlwire.RouteFamilySubmissions}, {"completion", controlwire.RouteFamilySubmissionCompletions},
		{"chit", controlwire.RouteFamilyChits}, {"payment", controlwire.RouteFamilyPayments},
		{"material", controlwire.RouteFamilyReleaseMaterials}, {"publication", controlwire.RouteFamilyReleasePublications},
		{"publication completion", controlwire.RouteFamilyReleasePublicationCompletions}, {"update", controlwire.RouteFamilyUpdateChecks}, {"upgrade", controlwire.RouteFamilyUpgrades},
	}
	for _, tc := range families {
		t.Run("authentic "+tc.name+" response cannot cross retrieval socket", func(t *testing.T) {
			t.Parallel()
			header := fixture.header
			header.Family = tc.family
			issuance := ResponseIssuance{Server: retrievalAuthServer(t, fixture.request.trusted), Signer: fixture.signer, Header: header, Body: fixture.grant, Assessment: responseAssessment(t, header)}
			projected, err := IssueResponse(issuance)
			if !errors.Is(err, core.ErrControlPlaneResponseBinding) || projected.Validate() == nil {
				t.Fatalf("sibling issuance = (%v, %v), want zero projection and binding refusal", projected.Validate(), err)
			}
			sibling, err := controlplane.IssueResponse(controlplane.ResponseIssuance[retrieval.GrantProjection]{Server: issuance.Server, Signer: issuance.Signer, Header: header, Body: issuance.Body, Assessment: issuance.Assessment})
			if err != nil {
				t.Fatalf("generic sibling issuance error = %v, want nil", err)
			}
			wire, err := sibling.MarshalJSON()
			if err != nil {
				t.Fatalf("sibling MarshalJSON error = %v, want nil", err)
			}
			var document controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
			if err := document.UnmarshalJSON(wire); err != nil {
				t.Fatalf("sibling decode error = %v, want nil", err)
			}
			expectation := fixture.expectation
			expectation.Family = tc.family
			proof, err := VerifyResponse(ResponseVerification{Client: fixture.client, Document: document, Expected: expectation})
			if !errors.Is(err, core.ErrControlPlaneResponseBinding) {
				t.Fatalf("sibling verification error = %v, want typed binding refusal", err)
			}
			responseRefusal(t, proof)
		})
	}
	t.Run("absent response exposes no header or capability", func(t *testing.T) {
		t.Parallel()
		proof, err := VerifyResponse(ResponseVerification{})
		if !errors.Is(err, core.ErrControlPlaneResponseDocument) {
			t.Fatalf("zero response error = %v, want response refusal", err)
		}
		responseRefusal(t, proof)
	})
}

func responseRefusal(t testing.TB, proof controlplane.VerifiedResponse[retrieval.GrantDocument, *retrieval.GrantDocument]) {
	t.Helper()
	header, headerErr := proof.Header()
	body, bodyErr := proof.Body()
	if !errors.Is(proof.Validate(), core.ErrControlPlaneResponseDocument) || header != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) || body.Validate() == nil {
		t.Fatalf("refused response = header %v/%v, body %v/%v, want zero header, absent grant and typed refusal", header, headerErr, body.Validate(), bodyErr)
	}
}

func FuzzRetrievalResponseSemanticAuthorityClosure(f *testing.F) {
	fixture := newResponseFixture(f, 0x21)
	foreign := newResponseFixture(f, 0x22)
	for selector := range uint8(5) {
		f.Add(fixture.canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("{}"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		expectation := fixture.expectation
		client := fixture.client
		var wantErr error
		switch selector % 5 {
		case 0:
		case 1:
			data = fixture.canonical
			nonce, err := controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{0x72})
			if err != nil {
				t.Fatalf("NewRequestNonce error = %v, want nil", err)
			}
			expectation.RequestNonce = nonce
			wantErr = core.ErrControlPlaneResponseBinding
		case 2:
			data = fixture.canonical
			expectation.Family = controlwire.RouteFamilyPayments
			wantErr = core.ErrControlPlaneResponseBinding
		case 3:
			data = fixture.canonical
			client = foreign.client
			wantErr = core.ErrAttestVerification
		case 4:
			data = fixture.canonical
			expectation.Offering = retrievalAuthOffering(t, 0x74)
			wantErr = core.ErrControlPlaneResponseBinding
		}
		if selector%5 != 0 && selector%5 != 3 && expectation == fixture.expectation {
			t.Fatal("response mutation = baseline, want changed bound fact")
		}
		var document controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
		if err := document.UnmarshalJSON(fixture.canonical); err != nil {
			t.Fatalf("seed decode error = %v, want nil", err)
		}
		if err := document.UnmarshalJSON(data); err != nil {
			if wantErr != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("response decode error = %v, want typed JSON refusal on raw input only", err)
			}
			proof, err := VerifyResponse(ResponseVerification{Client: fixture.client, Document: document, Expected: fixture.expectation})
			if err != nil {
				t.Fatalf("refused decode changed receiver: verify error = %v, want nil", err)
			}
			fixture.prove(t, proof)
			return
		}
		proof, err := VerifyResponse(ResponseVerification{Client: client, Document: document, Expected: expectation})
		if err != nil {
			if !errors.Is(err, core.ErrControlPlaneContract) || (wantErr != nil && !errors.Is(err, wantErr)) {
				t.Fatalf("verification error = %v, want typed boundary refusal %v", err, wantErr)
			}
			responseRefusal(t, proof)
			return
		}
		if wantErr != nil {
			t.Fatalf("mutation verification error = nil, want %v", wantErr)
		}
		fixture.prove(t, proof)
	})
}
