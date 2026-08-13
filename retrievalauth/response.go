package retrievalauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

type ResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   retrieval.GrantProjection
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[retrieval.GrantProjection](i)).Validate()
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[retrieval.GrantProjection], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[retrieval.GrantProjection](i))
}

func (v ResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument](v)).Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[retrieval.GrantDocument, *retrieval.GrantDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument](v),
	)
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
