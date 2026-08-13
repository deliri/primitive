package submissionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
)

type SubmissionResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   submission.DecisionProjection
}

type SubmissionResponseVerification struct {
	Document    controlplane.ResponseDocument[submission.DecisionDocument, *submission.DecisionDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i SubmissionResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[submission.DecisionProjection](i)).Validate()
}

func IssueSubmissionResponse(i SubmissionResponseIssuance) (controlplane.ResponseProjection[submission.DecisionProjection], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[submission.DecisionProjection](i))
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
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   chit.Document
}

type CompletionResponseVerification struct {
	Document    controlplane.ResponseDocument[chit.Document, *chit.Document]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i CompletionResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[chit.Document](i)).Validate()
}

func IssueCompletionResponse(i CompletionResponseIssuance) (controlplane.ResponseProjection[chit.Document], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[chit.Document](i))
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
