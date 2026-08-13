package distributionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
)

type MaterialResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   release.MaterialResponse
}

type MaterialResponseVerification struct {
	Document    controlplane.ResponseDocument[release.MaterialResponse, *release.MaterialResponse]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i MaterialResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[release.MaterialResponse](i)).Validate()
}

func IssueMaterialResponse(i MaterialResponseIssuance) (controlplane.ResponseProjection[release.MaterialResponse], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[release.MaterialResponse](i))
}

func (v MaterialResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse](v)).Validate()
}

func VerifyMaterialResponse(v MaterialResponseVerification) (controlplane.VerifiedResponse[release.MaterialResponse, *release.MaterialResponse], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse](v),
	)
}

type PublicationResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   distribution.PublicationGrantProjection
}

type PublicationResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i PublicationResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[distribution.PublicationGrantProjection](i)).Validate()
}

func IssuePublicationResponse(i PublicationResponseIssuance) (controlplane.ResponseProjection[distribution.PublicationGrantProjection], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[distribution.PublicationGrantProjection](i))
}

func (v PublicationResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument](v)).Validate()
}

func VerifyPublicationResponse(v PublicationResponseVerification) (controlplane.VerifiedResponse[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument](v),
	)
}

type PublicationCompletionResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   release.LatestDocument
}

type PublicationCompletionResponseVerification struct {
	Document    controlplane.ResponseDocument[release.LatestDocument, *release.LatestDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i PublicationCompletionResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[release.LatestDocument](i)).Validate()
}

func IssuePublicationCompletionResponse(
	i PublicationCompletionResponseIssuance,
) (controlplane.ResponseProjection[release.LatestDocument], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[release.LatestDocument](i))
}

func (v PublicationCompletionResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument](v)).Validate()
}

func VerifyPublicationCompletionResponse(
	v PublicationCompletionResponseVerification,
) (controlplane.VerifiedResponse[release.LatestDocument, *release.LatestDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument](v),
	)
}

type UpdateResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   distribution.UpdateResponseDocument
}

type UpdateResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i UpdateResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[distribution.UpdateResponseDocument](i)).Validate()
}

func IssueUpdateResponse(i UpdateResponseIssuance) (controlplane.ResponseProjection[distribution.UpdateResponseDocument], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[distribution.UpdateResponseDocument](i))
}

func (v UpdateResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument](v)).Validate()
}

func VerifyUpdateResponse(v UpdateResponseVerification) (controlplane.VerifiedResponse[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument](v),
	)
}

type UpgradeResponseIssuance struct {
	Signer crypto.Signer
	Header controlplane.ResponseHeader
	Body   distribution.UpgradeGrantProjection
}

type UpgradeResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i UpgradeResponseIssuance) Validate() error {
	return (controlplane.ResponseIssuance[distribution.UpgradeGrantProjection](i)).Validate()
}

func IssueUpgradeResponse(i UpgradeResponseIssuance) (controlplane.ResponseProjection[distribution.UpgradeGrantProjection], error) {
	return controlplane.IssueResponse(controlplane.ResponseIssuance[distribution.UpgradeGrantProjection](i))
}

func (v UpgradeResponseVerification) Validate() error {
	return (controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument](v)).Validate()
}

func VerifyUpgradeResponse(v UpgradeResponseVerification) (controlplane.VerifiedResponse[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument], error) {
	return controlplane.VerifyResponse(
		controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument](v),
	)
}

var (
	_ core.Validatable = MaterialResponseIssuance{}
	_ core.Validatable = MaterialResponseVerification{}
	_ core.Validatable = PublicationResponseIssuance{}
	_ core.Validatable = PublicationResponseVerification{}
	_ core.Validatable = PublicationCompletionResponseIssuance{}
	_ core.Validatable = PublicationCompletionResponseVerification{}
	_ core.Validatable = UpdateResponseIssuance{}
	_ core.Validatable = UpdateResponseVerification{}
	_ core.Validatable = UpgradeResponseIssuance{}
	_ core.Validatable = UpgradeResponseVerification{}
)
