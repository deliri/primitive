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
	_ core.Validatable = MetadataAssetRequest{}
	_ core.Validatable = MetadataInspectionRequest{}
	_ core.Validatable = MetadataSetRequest{}
	_ core.Validatable = BuildProvenanceRequest{}
	_ core.Validatable = ArtifactInspectionRequest{}
	_ core.Validatable = VerifyLatestRequest{}
	_ core.Validatable = VerifyManifestRequest{}
	_ core.Validatable = BuildDependencyObservationRequest{}
	_ core.Validatable = GoModulePath{}
	_ core.Validatable = GoModuleVersion{}
	_ core.Validatable = GoModuleSum{}
	_ core.Validatable = BuildDependency{}
	_ core.Validatable = BuildDependencies{}
	_ core.Validatable = RepositoryVerificationRequest{}
	_ core.Validatable = VerifiedRepository{}
	_ core.Validatable = RepositoryCommitMismatchError{}
	_ core.Validatable = RepositoryDirtyError{}

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
	_ core.ValidatedJSONMarshaler = MetadataKind(0)
	_ core.ValidatedJSONMarshaler = MetadataAsset{}
	_ core.ValidatedJSONMarshaler = MetadataSet{}
	_ core.ValidatedJSONMarshaler = BuildProvenance{}
	_ core.ValidatedJSONMarshaler = BuildDependencies{}
)
