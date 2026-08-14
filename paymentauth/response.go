package paymentauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Body       payment.CatalogDocument
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyPayments)
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[payment.CatalogDocument], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyPayments,
	)
}

func (i ResponseIssuance) responseIssuance() controlplane.ResponseIssuance[payment.CatalogDocument] {
	return controlplane.ResponseIssuance[payment.CatalogDocument]{
		Signer: i.Signer, Header: i.Header, Body: i.Body, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[payment.CatalogDocument, *payment.CatalogDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument] {
	return controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
