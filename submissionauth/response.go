package submissionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

type SubmissionResponseIssuance struct {
	Signer     crypto.Signer
	Body       submission.DecisionProjection
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type SubmissionResponseVerification struct {
	Document    controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
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
		Signer: i.Signer, Header: i.Header, Body: i.Body, Assessment: i.Assessment,
	}
}

func (v SubmissionResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[submission.DecisionDocument, *submission.DecisionDocument](v)).Validate()
}

func VerifySubmissionResponse(v SubmissionResponseVerification) (controlplane.VerifiedResponse[submission.DecisionDocument, *submission.DecisionDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[submission.DecisionDocument, *submission.DecisionDocument](v),
	)
}

type CompletionResponseIssuance struct {
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       chit.Document
	Assessment controlwire.ProtocolAssessment
}

type CompletionResponseVerification struct {
	Document    controlplane.ResponseDocument[chit.Document, *chit.Document]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i CompletionResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[chit.Document](i)).ValidateForFamily(controlwire.RouteFamilySubmissionCompletions)
}

func IssueCompletionResponse(i CompletionResponseIssuance) (controlplane.ResponseProjection[chit.Document], error) {
	return controlplane.IssueResponseForFamily(
		controlplane.ResponseIssuance[chit.Document](i), controlwire.RouteFamilySubmissionCompletions,
	)
}

func (v CompletionResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[chit.Document, *chit.Document](v)).Validate()
}

func VerifyCompletionResponse(v CompletionResponseVerification) (controlplane.VerifiedResponse[chit.Document, *chit.Document], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[chit.Document, *chit.Document](v),
	)
}

var (
	_ core.Validatable = SubmissionResponseIssuance{}
	_ core.Validatable = SubmissionResponseVerification{}
	_ core.Validatable = CompletionResponseIssuance{}
	_ core.Validatable = CompletionResponseVerification{}
)
