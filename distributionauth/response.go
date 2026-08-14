package distributionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
)

type MaterialResponseIssuance struct {
	Signer     crypto.Signer
	Body       release.MaterialResponse
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type MaterialResponseVerification struct {
	Document    controlplane.ResponseDocument[release.MaterialResponse, *release.MaterialResponse]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i MaterialResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyReleaseMaterials)
}

func IssueMaterialResponse(i MaterialResponseIssuance) (controlplane.ResponseProjection[release.MaterialResponse], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyReleaseMaterials,
	)
}

func (i MaterialResponseIssuance) responseIssuance() controlplane.ResponseIssuance[release.MaterialResponse] {
	return controlplane.ResponseIssuance[release.MaterialResponse]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v MaterialResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyMaterialResponse(v MaterialResponseVerification) (controlplane.VerifiedResponse[release.MaterialResponse, *release.MaterialResponse], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v MaterialResponseVerification) responseVerification() controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse] {
	return controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

type PublicationResponseIssuance struct {
	Signer     crypto.Signer
	Body       distribution.PublicationGrantProjection
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type PublicationResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i PublicationResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyReleasePublications)
}

func IssuePublicationResponse(i PublicationResponseIssuance) (controlplane.ResponseProjection[distribution.PublicationGrantProjection], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyReleasePublications,
	)
}

func (i PublicationResponseIssuance) responseIssuance() controlplane.ResponseIssuance[distribution.PublicationGrantProjection] {
	return controlplane.ResponseIssuance[distribution.PublicationGrantProjection]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v PublicationResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyPublicationResponse(v PublicationResponseVerification) (controlplane.VerifiedResponse[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v PublicationResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument] {
	return controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

type PublicationCompletionResponseIssuance struct {
	Signer     crypto.Signer
	Body       release.LatestDocument
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type PublicationCompletionResponseVerification struct {
	Document    controlplane.ResponseDocument[release.LatestDocument, *release.LatestDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i PublicationCompletionResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyReleasePublicationCompletions)
}

func IssuePublicationCompletionResponse(
	i PublicationCompletionResponseIssuance,
) (controlplane.ResponseProjection[release.LatestDocument], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyReleasePublicationCompletions,
	)
}

func (i PublicationCompletionResponseIssuance) responseIssuance() controlplane.ResponseIssuance[release.LatestDocument] {
	return controlplane.ResponseIssuance[release.LatestDocument]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v PublicationCompletionResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyPublicationCompletionResponse(
	v PublicationCompletionResponseVerification,
) (controlplane.VerifiedResponse[release.LatestDocument, *release.LatestDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v PublicationCompletionResponseVerification) responseVerification() controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument] {
	return controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

type UpdateResponseIssuance struct {
	Signer     crypto.Signer
	Body       distribution.UpdateResponseDocument
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type UpdateResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i UpdateResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyUpdateChecks)
}

func IssueUpdateResponse(i UpdateResponseIssuance) (controlplane.ResponseProjection[distribution.UpdateResponseDocument], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyUpdateChecks,
	)
}

func (i UpdateResponseIssuance) responseIssuance() controlplane.ResponseIssuance[distribution.UpdateResponseDocument] {
	return controlplane.ResponseIssuance[distribution.UpdateResponseDocument]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v UpdateResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyUpdateResponse(v UpdateResponseVerification) (controlplane.VerifiedResponse[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v UpdateResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument] {
	return controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
}

type UpgradeResponseIssuance struct {
	Signer     crypto.Signer
	Body       distribution.UpgradeGrantProjection
	Header     controlplane.ResponseHeader
	Assessment controlwire.ProtocolAssessment
}

type UpgradeResponseVerification struct {
	Document    controlplane.ResponseDocument[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]
	Expected    controlplane.ResponseExpectation
	TrustedKeys attest.TrustedKeys
}

func (i UpgradeResponseIssuance) Validate() error {
	return i.responseIssuance().ValidateForFamily(controlwire.RouteFamilyUpgrades)
}

func IssueUpgradeResponse(i UpgradeResponseIssuance) (controlplane.ResponseProjection[distribution.UpgradeGrantProjection], error) {
	return controlplane.IssueResponseForFamily(
		i.responseIssuance(), controlwire.RouteFamilyUpgrades,
	)
}

func (i UpgradeResponseIssuance) responseIssuance() controlplane.ResponseIssuance[distribution.UpgradeGrantProjection] {
	return controlplane.ResponseIssuance[distribution.UpgradeGrantProjection]{
		Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v UpgradeResponseVerification) Validate() error {
	return v.responseVerification().Validate()
}

func VerifyUpgradeResponse(v UpgradeResponseVerification) (controlplane.VerifiedResponse[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument], error) {
	return controlplane.VerifyResponse(v.responseVerification())
}

func (v UpgradeResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument] {
	return controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]{
		Document: v.Document, Expected: v.Expected, TrustedKeys: v.TrustedKeys,
	}
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
