package chitauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       chit.CatalogDocument
	Server     controlplane.Authority
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
	Client   controlplane.Client
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyChits)
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[chit.CatalogDocument, *chit.CatalogDocument], error) {
	return controlplane.VerifyResponseForFamily(v.responseVerification(), controlwire.RouteFamilyChits)
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument] {
	return controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
