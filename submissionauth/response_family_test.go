package submissionauth

import (
	"crypto"
	"errors"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
	"io"
	"testing"
)

type authCountingSigner struct {
	key   crypto.Signer
	calls int
}

func (s *authCountingSigner) Public() crypto.PublicKey { return s.key.Public() }
func (s *authCountingSigner) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.calls++
	return s.key.Sign(random, digest, opts)
}

func TestSubmissionResponseFamilyLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	body, err := submission.UploadDecision(fixture.grantProjection)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		family    controlwire.RouteFamily
		absent    bool
		wantErr   error
		wantSigns int
	}{
		{"exact_family_authenticates", controlwire.RouteFamilySubmissions, false, nil, 1},
		{"sibling_family_refused_before_signing", controlwire.RouteFamilySubmissionCompletions, false, core.ErrControlPlaneResponseBinding, 0},
		{"absent_authority_cannot_issue", controlwire.RouteFamilySubmissions, true, core.ErrControlPlaneResponseDocument, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			header := authResponseHeader(t, fixture, tc.family)
			signer := &authCountingSigner{key: fixture.request.authority}
			server := submissionAuthServer(t, fixture.request.trusted)
			if tc.absent {
				server = controlplane.Authority{}
			}
			request := SubmissionResponseIssuance{Signer: signer, Body: body, Header: header, Server: server, Assessment: acceptedSubmissionResponseAssessment(t, header)}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("issuance validation=%v, want %v", err, tc.wantErr)
			}
			projection, err := IssueSubmissionResponse(request)
			if !errors.Is(err, tc.wantErr) || signer.calls != tc.wantSigns {
				t.Fatalf("issuance error=%v sign calls=%d, want %v/%d", err, signer.calls, tc.wantErr, tc.wantSigns)
			}
			if err != nil {
				if encoded, err := projection.MarshalJSON(); len(encoded) != 0 || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
					t.Fatalf("refused issuance escaped %d bytes and %v", len(encoded), err)
				}
			}
			if tc.absent {
				return
			}
			// The shared authority is the producer for an authentic sibling-family attack.
			produced, err := controlplane.IssueResponse(controlplane.ResponseIssuance[submission.DecisionProjection]{Signer: fixture.request.authority, Body: body, Header: header, Server: server, Assessment: request.Assessment})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := produced.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var document controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
			if err := document.UnmarshalJSON(encoded); err != nil {
				t.Fatal(err)
			}
			verification := SubmissionResponseVerification{Document: document, Expected: authResponseExpectation(header), Client: submissionAuthClient(t, fixture.request.trusted)}
			if err := verification.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("verification validation=%v, want %v", err, tc.wantErr)
			}
			got, err := VerifySubmissionResponse(verification)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("verification=%v, want %v", err, tc.wantErr)
			}
			gotHeader, headerErr := got.Header()
			_, bodyErr := got.Body()
			if err != nil {
				if gotHeader != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) {
					t.Fatalf("refused response retained header=%v errors=%v/%v", gotHeader, headerErr, bodyErr)
				}
			} else if headerErr != nil || bodyErr != nil || gotHeader != header {
				t.Fatalf("authenticated header=%v errors=%v/%v, want exact %v", gotHeader, headerErr, bodyErr, header)
			}
		})
	}
}

func TestCompletionResponseFamilyLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	body := authCompletionResponseBody(t, fixture)
	for _, tc := range []struct {
		name      string
		family    controlwire.RouteFamily
		absent    bool
		wantErr   error
		wantSigns int
	}{
		{"exact_family_authenticates", controlwire.RouteFamilySubmissionCompletions, false, nil, 1},
		{"sibling_family_refused_before_signing", controlwire.RouteFamilySubmissions, false, core.ErrControlPlaneResponseBinding, 0},
		{"absent_authority_cannot_issue", controlwire.RouteFamilySubmissionCompletions, true, core.ErrControlPlaneResponseDocument, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			header := authResponseHeader(t, fixture, tc.family)
			signer := &authCountingSigner{key: fixture.request.authority}
			server := submissionAuthServer(t, fixture.request.trusted)
			if tc.absent {
				server = controlplane.Authority{}
			}
			request := CompletionResponseIssuance{Signer: signer, Body: body, Header: header, Server: server, Assessment: acceptedSubmissionResponseAssessment(t, header)}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("issuance validation=%v, want %v", err, tc.wantErr)
			}
			projection, err := IssueCompletionResponse(request)
			if !errors.Is(err, tc.wantErr) || signer.calls != tc.wantSigns {
				t.Fatalf("issuance error=%v sign calls=%d, want %v/%d", err, signer.calls, tc.wantErr, tc.wantSigns)
			}
			if err != nil {
				if encoded, err := projection.MarshalJSON(); len(encoded) != 0 || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
					t.Fatalf("refused issuance escaped %d bytes and %v", len(encoded), err)
				}
			}
			if tc.absent {
				return
			}
			// The shared authority is the producer for an authentic sibling-family attack.
			produced, err := controlplane.IssueResponse(controlplane.ResponseIssuance[chit.Document]{Signer: fixture.request.authority, Body: body, Header: header, Server: server, Assessment: request.Assessment})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := produced.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var document controlplane.ResponseDocument[chit.Document, *chit.Document]
			if err := document.UnmarshalJSON(encoded); err != nil {
				t.Fatal(err)
			}
			verification := CompletionResponseVerification{Document: document, Expected: authResponseExpectation(header), Client: submissionAuthClient(t, fixture.request.trusted)}
			if err := verification.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("verification validation=%v, want %v", err, tc.wantErr)
			}
			got, err := VerifyCompletionResponse(verification)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("verification=%v, want %v", err, tc.wantErr)
			}
			gotHeader, headerErr := got.Header()
			_, bodyErr := got.Body()
			if err != nil {
				if gotHeader != (controlplane.ResponseHeader{}) || !errors.Is(headerErr, core.ErrControlPlaneResponseDocument) || !errors.Is(bodyErr, core.ErrControlPlaneResponseDocument) {
					t.Fatalf("refused response retained header=%v errors=%v/%v", gotHeader, headerErr, bodyErr)
				}
			} else if headerErr != nil || bodyErr != nil || gotHeader != header {
				t.Fatalf("authenticated header=%v errors=%v/%v, want exact %v", gotHeader, headerErr, bodyErr, header)
			}
		})
	}
}
