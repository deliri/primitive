package release

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = AdvanceLatestRequest{}
	_ core.Validatable = ArtifactRequest{}
	_ core.Validatable = ArtifactSetRequest{}
	_ core.Validatable = AssessLatestRequest{}
	_ core.Validatable = EvaluateRequest{}
	_ core.Validatable = IssueLatestRequest{}
	_ core.Validatable = IssueManifestRequest{}
	_ core.Validatable = ManifestFactRequest{}
	_ core.Validatable = VerifyLatestRequest{}
	_ core.Validatable = VerifyManifestRequest{}

	_ core.ValidatedJSONMarshaler = ArtifactIdentity{}
	_ core.ValidatedJSONMarshaler = ArtifactIntegrity{}
	_ core.ValidatedJSONMarshaler = Artifact{}
	_ core.ValidatedJSONMarshaler = ArtifactSet{}
	_ core.ValidatedJSONMarshaler = Revision(0)
	_ core.ValidatedJSONMarshaler = Generation{}
	_ core.ValidatedJSONMarshaler = LatestIdentity{}
	_ core.ValidatedJSONMarshaler = LatestFact{}
	_ core.ValidatedJSONMarshaler = LatestDocument{}
	_ core.ValidatedJSONMarshaler = ManifestIdentity{}
	_ core.ValidatedJSONMarshaler = ManifestFact{}
	_ core.ValidatedJSONMarshaler = ManifestDocument{}
)
