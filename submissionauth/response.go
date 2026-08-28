package submissionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

type SubmissionResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Body       submission.DecisionProjection
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type SubmissionResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
}

func (i SubmissionResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilySubmissions)
}

func IssueSubmissionResponse(i SubmissionResponseIssuance) (controlplane.ResponseProjection[submission.DecisionProjection], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilySubmissions,
	)
}

func (i SubmissionResponseIssuance) responseIssuance() controlplane.ResponseIssuance[submission.DecisionProjection] {
	return controlplane.ResponseIssuance[submission.DecisionProjection]{
		Server: i.Server, Signer: i.Signer, Header: i.Header, Body: i.Body, Assessment: i.Assessment,
	}
}

func (v SubmissionResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifySubmissionResponse(v SubmissionResponseVerification) (controlplane.VerifiedResponse[submission.DecisionDocument, *submission.DecisionDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v SubmissionResponseVerification) responseVerification() controlplane.ResponseVerification[submission.DecisionDocument, *submission.DecisionDocument] {
	return controlplane.ResponseVerification[submission.DecisionDocument, *submission.DecisionDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

type CompletionResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       chit.Document
	Assessment controlwire.ProtocolAssessment
}

type CompletionResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[chit.Document, *chit.Document]
}

func (i CompletionResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilySubmissionCompletions)
}

func IssueCompletionResponse(i CompletionResponseIssuance) (controlplane.ResponseProjection[chit.Document], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilySubmissionCompletions,
	)
}

func (i CompletionResponseIssuance) responseIssuance() controlplane.ResponseIssuance[chit.Document] {
	return controlplane.ResponseIssuance[chit.Document]{
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v CompletionResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyCompletionResponse(v CompletionResponseVerification) (controlplane.VerifiedResponse[chit.Document, *chit.Document], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v CompletionResponseVerification) responseVerification() controlplane.ResponseVerification[chit.Document, *chit.Document] {
	return controlplane.ResponseVerification[chit.Document, *chit.Document]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

var (
	_ core.Validatable = SubmissionResponseIssuance{}
	_ core.Validatable = SubmissionResponseVerification{}
	_ core.Validatable = CompletionResponseIssuance{}
	_ core.Validatable = CompletionResponseVerification{}
)
