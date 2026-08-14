package retrievalauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Body       retrieval.GrantProjection
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyRetrievals)
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[retrieval.GrantProjection], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyRetrievals,
	)
}

func (i ResponseIssuance) responseIssuance() controlplane.ResponseIssuance[retrieval.GrantProjection] {
	return controlplane.ResponseIssuance[retrieval.GrantProjection]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[retrieval.GrantDocument, *retrieval.GrantDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument] {
	return controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
