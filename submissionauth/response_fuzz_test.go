package submissionauth

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
	"testing"
)

func FuzzSubmissionResponseAuthorityClosure(f *testing.F) {
	fixture := newAuthCompletionFixture(f, authCompletionFixtureRequest{})
	foreign := newAuthFixture(f, authFixtureRequest{authorityByte: 0x79})
	header := authResponseHeader(f, fixture, controlwire.RouteFamilySubmissions)
	server := submissionAuthServer(f, fixture.request.trusted)
	client := submissionAuthClient(f, fixture.request.trusted)
	body, err := submission.UploadDecision(fixture.grantProjection)
	if err != nil {
		f.Fatal(err)
	}
	projection, err := IssueSubmissionResponse(SubmissionResponseIssuance{Header: header, Body: body, Signer: fixture.request.authority, Server: server, Assessment: acceptedSubmissionResponseAssessment(f, header)})
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for selector := range uint8(5) {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("{}"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		expected := authResponseExpectation(header)
		trusted := client
		wantErr := error(nil)
		switch selector % 5 {
		case 1:
			data = canonical
			expected.RequestNonce = authRequestNonce(t, 0x5e)
			wantErr = core.ErrControlPlaneResponseBinding
		case 2:
			data = canonical
			expected.Offering = submissionAuthOffering(t, 0x5e)
			wantErr = core.ErrControlPlaneResponseBinding
		case 3:
			data = canonical
			trusted = submissionAuthClient(t, foreign.trusted)
			wantErr = core.ErrAttestVerification
		case 4:
			data = canonical
			trusted = controlplane.Client{}
			wantErr = core.ErrControlPlaneResponseDocument
		}
		var document controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
		if err := document.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || bytes.Equal(data, canonical) {
				t.Fatalf("response ingress error=%v, want typed refusal", err)
			}
			return
		}
		got, err := VerifySubmissionResponse(SubmissionResponseVerification{Document: document, Expected: expected, Client: trusted})
		if err != nil {
			if bytes.Equal(data, canonical) && !errors.Is(err, wantErr) {
				t.Fatalf("verification error=%v, want %v", err, wantErr)
			}
			if !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("verification lost boundary identity: %v", err)
			}
			zeroHeader, headerErr := got.Header()
			_, bodyErr := got.Body()
			if zeroHeader != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("refused response exposes header=%v errors=%v/%v", zeroHeader, headerErr, bodyErr)
			}
			return
		}
		if wantErr != nil {
			t.Fatalf("response accepted a forced %v mutation", wantErr)
		}
		gotHeader, err := got.Header()
		if err != nil || gotHeader != header {
			t.Fatalf("authenticated header=%v error=%v, want exact signed seed", gotHeader, err)
		}
		gotBody, err := got.Body()
		if err != nil {
			t.Fatal(err)
		}
		scope, err := fixture.request.certificate.Body.Scope()
		if err != nil {
			t.Fatal(err)
		}
		decision, err := submission.VerifyDecision(submission.DecisionExpectation{Decision: gotBody, Request: fixture.request.request.Payload, Scope: scope, ObservedAt: fixture.grant.Payload.IssuedAt, TrustedKeys: fixture.request.trusted})
		if err != nil {
			t.Fatal(err)
		}
		kind, err := decision.Kind()
		grant, present := decision.Grant()
		if err != nil || kind != submission.DecisionUpload || !present {
			t.Fatalf("decision=%v present=%t error=%v, want authenticated upload", kind, present, err)
		}
		payload, err := grant.Payload()
		if err != nil || payload != fixture.grant.Payload {
			t.Fatalf("grant=%v error=%v, want exact signed grant", payload, err)
		}

	})
}

func FuzzCompletionResponseAuthorityClosure(f *testing.F) {
	fixture := newAuthCompletionFixture(f, authCompletionFixtureRequest{})
	foreign := newAuthFixture(f, authFixtureRequest{authorityByte: 0x79})
	header := authResponseHeader(f, fixture, controlwire.RouteFamilySubmissionCompletions)
	server := submissionAuthServer(f, fixture.request.trusted)
	client := submissionAuthClient(f, fixture.request.trusted)
	body := authCompletionResponseBody(f, fixture)
	projection, err := IssueCompletionResponse(CompletionResponseIssuance{Header: header, Body: body, Signer: fixture.request.authority, Server: server, Assessment: acceptedSubmissionResponseAssessment(f, header)})
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	wantBody, err := body.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for selector := range uint8(5) {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("{}"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		expected := authResponseExpectation(header)
		trusted := client
		wantErr := error(nil)
		switch selector % 5 {
		case 1:
			data = canonical
			expected.RequestNonce = authRequestNonce(t, 0x5e)
			wantErr = core.ErrControlPlaneResponseBinding
		case 2:
			data = canonical
			expected.Offering = submissionAuthOffering(t, 0x5e)
			wantErr = core.ErrControlPlaneResponseBinding
		case 3:
			data = canonical
			trusted = submissionAuthClient(t, foreign.trusted)
			wantErr = core.ErrAttestVerification
		case 4:
			data = canonical
			trusted = controlplane.Client{}
			wantErr = core.ErrControlPlaneResponseDocument
		}
		var document controlplane.ResponseDocument[chit.Document, *chit.Document]
		if err := document.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || bytes.Equal(data, canonical) {
				t.Fatalf("response ingress error=%v, want typed refusal", err)
			}
			return
		}
		got, err := VerifyCompletionResponse(CompletionResponseVerification{Document: document, Expected: expected, Client: trusted})
		if err != nil {
			if bytes.Equal(data, canonical) && !errors.Is(err, wantErr) {
				t.Fatalf("verification error=%v, want %v", err, wantErr)
			}
			if !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("verification lost boundary identity: %v", err)
			}
			zeroHeader, headerErr := got.Header()
			_, bodyErr := got.Body()
			if zeroHeader != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("refused response exposes header=%v errors=%v/%v", zeroHeader, headerErr, bodyErr)
			}
			return
		}
		if wantErr != nil {
			t.Fatalf("response accepted a forced %v mutation", wantErr)
		}
		gotHeader, err := got.Header()
		if err != nil || gotHeader != header {
			t.Fatalf("authenticated header=%v error=%v, want exact signed seed", gotHeader, err)
		}
		gotBody, err := got.Body()
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := gotBody.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, wantBody) {
			t.Fatalf("authenticated body=%d bytes error=%v, want exact signed seed", len(encoded), err)
		}
	})
}
