package distributionauth

import (
	"crypto"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
)

type MaterialResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       release.MaterialResponse
	Assessment controlwire.ProtocolAssessment
}

type MaterialResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[release.MaterialResponse, *release.MaterialResponse]
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v MaterialResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyReleaseMaterials)
}

func VerifyMaterialResponse(v MaterialResponseVerification) (controlplane.VerifiedResponse[release.MaterialResponse, *release.MaterialResponse], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyReleaseMaterials,
	)
}

func (v MaterialResponseVerification) responseVerification() controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse] {
	return controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

type PublicationResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       distribution.PublicationGrantProjection
	Assessment controlwire.ProtocolAssessment
}

type PublicationResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v PublicationResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyReleasePublications)
}

func VerifyPublicationResponse(v PublicationResponseVerification) (controlplane.VerifiedResponse[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyReleasePublications,
	)
}

func (v PublicationResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument] {
	return controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

type PublicationCompletionResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       release.LatestDocument
	Assessment controlwire.ProtocolAssessment
}

type PublicationCompletionResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[release.LatestDocument, *release.LatestDocument]
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v PublicationCompletionResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyReleasePublicationCompletions)
}

func VerifyPublicationCompletionResponse(
	v PublicationCompletionResponseVerification,
) (controlplane.VerifiedResponse[release.LatestDocument, *release.LatestDocument], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyReleasePublicationCompletions,
	)
}

func (v PublicationCompletionResponseVerification) responseVerification() controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument] {
	return controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

type UpdateResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       distribution.UpdateResponseDocument
	Assessment controlwire.ProtocolAssessment
}

type UpdateResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v UpdateResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyUpdateChecks)
}

func VerifyUpdateResponse(v UpdateResponseVerification) (controlplane.VerifiedResponse[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyUpdateChecks,
	)
}

func (v UpdateResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument] {
	return controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
	}
}

type UpgradeResponseIssuance struct {
	Server     controlplane.Server
	Signer     crypto.Signer
	Header     controlplane.ResponseHeader
	Body       distribution.UpgradeGrantProjection
	Assessment controlwire.ProtocolAssessment
}

type UpgradeResponseVerification struct {
	Client   controlplane.Client
	Expected controlplane.ResponseExpectation
	Document controlplane.ResponseDocument[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]
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
		Server: i.Server, Signer: i.Signer, Body: i.Body, Header: i.Header, Assessment: i.Assessment,
	}
}

func (v UpgradeResponseVerification) Validate() error {
	return v.responseVerification().ValidateForFamily(controlwire.RouteFamilyUpgrades)
}

func VerifyUpgradeResponse(v UpgradeResponseVerification) (controlplane.VerifiedResponse[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument], error) {
	return controlplane.VerifyResponseForFamily(
		v.responseVerification(), controlwire.RouteFamilyUpgrades,
	)
}

func (v UpgradeResponseVerification) responseVerification() controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument] {
	return controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]{
		Client: v.Client, Document: v.Document, Expected: v.Expected,
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
