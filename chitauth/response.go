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
	Header     controlplane.ResponseHeader
	Body       chit.CatalogDocument
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[chit.CatalogDocument, *chit.CatalogDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[chit.CatalogDocument](i)).ValidateForFamily(controlwire.RouteFamilyChits)
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[chit.CatalogDocument], error) {
	return controlplane.IssueResponseForFamily(
		controlplane.ResponseIssuance[chit.CatalogDocument](i), controlwire.RouteFamilyChits,
	)
}

func (v ResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument](v)).Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[chit.CatalogDocument, *chit.CatalogDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[chit.CatalogDocument, *chit.CatalogDocument](v),
	)
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
