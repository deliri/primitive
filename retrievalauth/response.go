package retrievalauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       retrieval.GrantProjection
	Server     controlplane.Authority
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[retrieval.GrantDocument, *retrieval.GrantDocument]
	Client   controlplane.Client
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyRetrievals)
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[retrieval.GrantDocument, *retrieval.GrantDocument], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyRetrievals,
	)
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument] {
	return controlplane.ResponseVerification[retrieval.GrantDocument, *retrieval.GrantDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
