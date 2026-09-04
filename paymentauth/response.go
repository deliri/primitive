package paymentauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
)

type ResponseIssuance struct {
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       payment.CatalogDocument
	Server     controlplane.Authority
	Assessment controlwire.ProtocolAssessment
}

type ResponseVerification struct {
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	Client   controlplane.Client
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
		Server: i.Server, Signer: i.Signer, Header: i.Header, Body: i.Body, Assessment: i.Assessment,
	}
}

func (v ResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyPayments)
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[payment.CatalogDocument, *payment.CatalogDocument], error) {
	return controlplane.VerifyResponseForFamily(v.responseVerification(), controlwire.RouteFamilyPayments)
}

func (v ResponseVerification) responseVerification() controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument] {
	return controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
