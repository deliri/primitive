package paymentauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/payment"
)

type ResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   payment.CatalogDocument
}

type ResponseVerification struct {
	Document    controlplane.ResponseDocument[payment.CatalogDocument, *payment.CatalogDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i ResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[payment.CatalogDocument](i)).Validate()
}

func IssueResponse(i ResponseIssuance) (controlplane.ResponseProjection[payment.CatalogDocument], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[payment.CatalogDocument](i))
}

func (v ResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument](v)).Validate()
}

func VerifyResponse(v ResponseVerification) (controlplane.VerifiedResponse[payment.CatalogDocument, *payment.CatalogDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[payment.CatalogDocument, *payment.CatalogDocument](v),
	)
}

var (
	_ core.Validatable = ResponseIssuance{}
	_ core.Validatable = ResponseVerification{}
)
