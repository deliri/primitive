package chitauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Body       chit.CatalogDocument
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyChits)
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[chit.CatalogDocument], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyChits,
	)
}

func (i ResponseIssuance) responseIssuance() controlplane.ResponseIssuance[chit.CatalogDocument] {
	return controlplane.ResponseIssuance[chit.CatalogDocument]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[chit.CatalogDocument, *chit.CatalogDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument] {
	return controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
