package chitauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type ResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       chit.CatalogDocument
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
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
	return v.responseVerification().Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[chit.CatalogDocument, *chit.CatalogDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
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
