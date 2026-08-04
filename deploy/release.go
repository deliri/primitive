package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

const (
	// ReleaseObjectCount is the exact immutable object count in one deployed
	// release: four executables, one signed manifest, and three metadata assets.
	ReleaseObjectCount         = 8
	releaseManifestObjectIndex = 4
	releaseMetadataObjectIndex = 5
)

var (
	_ [ReleaseObjectCount - (release.TargetCount + 1 + release.MetadataAssetCount)]struct{}
	_ [(release.TargetCount + 1 + release.MetadataAssetCount) - ReleaseObjectCount]struct{}
	_ [releaseManifestObjectIndex - release.TargetCount]struct{}
	_ [release.TargetCount - releaseManifestObjectIndex]struct{}
	_ [releaseMetadataObjectIndex - (releaseManifestObjectIndex + 1)]struct{}
	_ [(releaseManifestObjectIndex + 1) - releaseMetadataObjectIndex]struct{}
)

// ObjectRole is the closed ordered publication object domain.
type ObjectRole uint8

const (
	ObjectRoleUnknown ObjectRole = iota
	ObjectRoleWindowsAMD64
	ObjectRoleDarwinARM64
	ObjectRoleLinuxAMD64
	ObjectRoleLinuxARM64
	ObjectRoleManifest
	ObjectRoleDependencies
	ObjectRoleDocumentation
	ObjectRoleReleaseNotes
	objectRoleLimit
)

func objectRoleLabels() [objectRoleLimit]string {
	return [...]string{
		"", "windows_amd64", "darwin_arm64", "linux_amd64", "linux_arm64",
		"manifest", release.MetadataKindDependencies.String(),
		release.MetadataKindDocumentation.String(), release.MetadataKindReleaseNotes.String(),
	}
}

func (r ObjectRole) Validate() error {
	if r <= ObjectRoleUnknown || r >= objectRoleLimit || objectRoleLabels()[r] == "" {
		return contractError(errors.New("deploy object role is outside the closed domain"))
	}
	return nil
}

func (r ObjectRole) IsValid() bool { return r.Validate() == nil }
func (ObjectRole) OffWireEnum()    {}
func (r ObjectRole) String() string {
	if !r.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return objectRoleLabels()[r]
}

// UploadItemRequest supplies one already-issued capability and exact source.
// Commitment must be the separately authenticated commitment bound by the
// caller's grant protocol.
type UploadItemRequest struct {
	Source     io.Reader
	Capability objectstore.UploadCapability
	Integrity  objectstore.Integrity
	Commitment objectstore.UploadCapabilityCommitment
	Role       ObjectRole
}

func (r UploadItemRequest) Validate() error {
	if r.Source == nil {
		return contractError(errors.New("deploy upload source is nil"))
	}
	for _, err := range []error{
		r.Capability.Validate(), r.Commitment.Validate(), r.Integrity.Validate(), r.Role.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("deploy upload item is invalid"), err)
		}
	}
	provider, err := r.Capability.Provider()
	if err != nil || provider != objectstore.ProviderGoogleCloudStorage {
		return contractError(errors.New("deploy capability is not google cloud storage"), err)
	}
	commitment, err := r.Capability.Commitment()
	if err != nil || commitment != r.Commitment {
		return contractError(errors.New("deploy capability commitment differs from its grant"), err)
	}
	return nil
}

// UploadItem is a sealed source/capability pair. Its bearer remains opaque.
type UploadItem struct {
	source     io.Reader
	capability objectstore.UploadCapability
	integrity  objectstore.Integrity
	commitment objectstore.UploadCapabilityCommitment
	role       ObjectRole
	valid      bool
}

func NewUploadItem(request UploadItemRequest) (UploadItem, error) {
	if err := request.Validate(); err != nil {
		return UploadItem{}, err
	}
	return UploadItem{
		source: request.Source, capability: request.Capability,
		commitment: request.Commitment, integrity: request.Integrity,
		role: request.Role, valid: true,
	}, nil
}

func (i UploadItem) Validate() error {
	if !i.valid {
		return contractError(errors.New("deploy upload item is unset"))
	}
	return (UploadItemRequest{
		Source: i.source, Capability: i.capability, Commitment: i.commitment,
		Integrity: i.integrity, Role: i.role,
	}).Validate()
}

// ReleasePlanRequest binds all upload items to one authenticated manifest.
type ReleasePlanRequest struct {
	Items    [ReleaseObjectCount]UploadItem
	Manifest release.VerifiedManifest
	Policy   objectstore.Policy
}

func (r ReleasePlanRequest) Validate() error {
	if err := r.Manifest.Validate(); err != nil {
		return contractError(errors.New("deploy manifest is not verified"), err)
	}
	if err := r.Policy.Validate(); err != nil {
		return contractError(errors.New("deploy objectstore policy is invalid"), err)
	}
	return validateReleaseItems(r.Manifest, r.Items)
}

// ReleasePlan is an exact fixed-order GCS deployment prepared before any
// source is read or external request begins.
type ReleasePlan struct {
	items    [ReleaseObjectCount]UploadItem
	manifest release.VerifiedManifest
	policy   objectstore.Policy
	valid    bool
}

func PrepareRelease(request ReleasePlanRequest) (ReleasePlan, error) {
	if err := request.Validate(); err != nil {
		return ReleasePlan{}, err
	}
	return ReleasePlan{
		manifest: request.Manifest, items: request.Items,
		policy: request.Policy, valid: true,
	}, nil
}

func (p ReleasePlan) Validate() error {
	if !p.valid {
		return contractError(errors.New("deploy release plan is unset"))
	}
	return (ReleasePlanRequest{
		Manifest: p.manifest, Items: p.items, Policy: p.policy,
	}).Validate()
}

func validateReleaseItems(
	manifest release.VerifiedManifest,
	items [ReleaseObjectCount]UploadItem,
) error {
	if err := validateUniqueCapabilities(items); err != nil {
		return err
	}
	artifacts := manifest.Artifacts()
	for index := range release.TargetCount {
		artifact, ok := artifacts.At(index)
		if !ok {
			return contractError(errors.New("deploy manifest artifact slot is invalid"))
		}
		if err := validateReleaseItem(items[index], ObjectRole(index+1), artifact.Integrity()); err != nil {
			return err
		}
	}
	if err := validateManifestItem(items[releaseManifestObjectIndex], manifest); err != nil {
		return err
	}
	metadata := manifest.Metadata()
	for index := range release.MetadataAssetCount {
		asset, ok := metadata.At(index)
		if !ok {
			return contractError(errors.New("deploy metadata slot is invalid"))
		}
		role := ObjectRole(int(ObjectRoleDependencies) + index)
		if err := validateReleaseItem(items[index+releaseMetadataObjectIndex], role, asset.Integrity()); err != nil {
			return err
		}
	}
	return nil
}

func validateUniqueCapabilities(items [ReleaseObjectCount]UploadItem) error {
	for index, item := range items {
		for prior := range index {
			if item.commitment == items[prior].commitment {
				return contractError(errors.New("deploy capability commitment is duplicated"))
			}
		}
	}
	return nil
}

func validateReleaseItem(
	item UploadItem,
	role ObjectRole,
	integrity release.ArtifactIntegrity,
) error {
	if err := item.Validate(); err != nil {
		return err
	}
	if item.role != role {
		return contractError(errors.New("deploy upload item occupies the wrong role slot"))
	}
	want, err := objectstoreIntegrity(integrity)
	if err != nil {
		return err
	}
	if item.integrity != want {
		return contractError(errors.New("deploy upload integrity differs from the manifest"))
	}
	return nil
}

func validateManifestItem(item UploadItem, manifest release.VerifiedManifest) error {
	if err := item.Validate(); err != nil {
		return err
	}
	if item.role != ObjectRoleManifest {
		return contractError(errors.New("deploy manifest occupies the wrong role slot"))
	}
	want, err := objectstoreIntegrity(manifest.DocumentIntegrity())
	if err != nil {
		return err
	}
	if item.integrity != want {
		return contractError(errors.New("deploy manifest bytes differ from the authenticated document"))
	}
	return nil
}

func objectstoreIntegrity(value release.ArtifactIntegrity) (objectstore.Integrity, error) {
	extent, err := value.Extent().Uint64()
	if err != nil {
		return objectstore.Integrity{}, contractError(err)
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return objectstore.Integrity{}, contractError(err)
	}
	return objectstore.Integrity{
		Length: length, SHA256: value.SHA256(), CRC32C: value.CRC32C(),
	}, nil
}

// Receipt is confirmed provider evidence for one exact deployed object.
type Receipt struct {
	transfer   objectstore.Transfer
	commitment objectstore.UploadCapabilityCommitment
	role       ObjectRole
	valid      bool
}

func (r Receipt) Validate() error {
	if !r.valid {
		return contractError(errors.New("deploy receipt is unset"))
	}
	for _, err := range []error{r.transfer.Validate(), r.commitment.Validate(), r.role.Validate()} {
		if err != nil {
			return contractError(errors.New("deploy receipt is invalid"), err)
		}
	}
	if r.transfer.Provider() != objectstore.ProviderGoogleCloudStorage ||
		r.transfer.Direction() != objectstore.DirectionUpload {
		return contractError(errors.New("deploy receipt names the wrong transfer"))
	}
	return nil
}

func (r Receipt) Role() ObjectRole               { return r.role }
func (r Receipt) Transfer() objectstore.Transfer { return r.transfer }

// Receipts contains the confirmed prefix of the plan. A failed upload returns
// prior confirmations here and the failed transfer in UploadError.
type Receipts struct {
	values [ReleaseObjectCount]Receipt
	count  uint8
}

func (r Receipts) Validate() error {
	if int(r.count) > len(r.values) {
		return contractError(errors.New("deploy receipt count exceeds storage"))
	}
	for index, receipt := range r.values {
		if index < int(r.count) {
			if err := receipt.Validate(); err != nil {
				return err
			}
			if receipt.role != ObjectRole(index+1) {
				return contractError(errors.New("deploy receipt occupies the wrong role slot"))
			}
			continue
		}
		if receipt != (Receipt{}) {
			return contractError(errors.New("deploy receipt padding is nonzero"))
		}
	}
	return nil
}

func (r Receipts) Count() int { return int(r.count) }
func (r Receipts) At(index int) (Receipt, bool) {
	if r.Validate() != nil || index < 0 || index >= int(r.count) {
		return Receipt{}, false
	}
	return r.values[index], true
}

// UploadError retains the failed role and Objectstore's commitment evidence
// without exposing the signed capability.
type UploadError struct {
	Cause    error
	Transfer objectstore.Transfer
	Role     ObjectRole
}

func (e *UploadError) Error() string {
	if e == nil {
		return core.ErrDeployContract.Error()
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", core.ErrDeployContract, e.Role)
	}
	return fmt.Sprintf("%s: %s: %s", core.ErrDeployContract, e.Role, e.Cause)
}

func (e *UploadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return errors.Join(core.ErrDeployContract, e.Cause)
}

// ReleaseGCS performs each exact create-only upload once, in manifest
// order. It has no retry, reconciliation, activation, or Latest side effect.
func ReleaseGCS(
	ctx context.Context,
	client objectstore.Client,
	plan ReleasePlan,
) (Receipts, error) {
	if ctx == nil {
		return Receipts{}, contractError(core.ErrNilContext)
	}
	if err := ctx.Err(); err != nil {
		return Receipts{}, contractError(err)
	}
	if err := client.Validate(); err != nil {
		return Receipts{}, contractError(errors.New("deploy objectstore client is invalid"), err)
	}
	if err := plan.Validate(); err != nil {
		return Receipts{}, err
	}
	var receipts Receipts
	for index, item := range plan.items {
		target, err := item.capability.Target()
		if err != nil {
			return receipts, contractError(err)
		}
		contentType, err := contentTypeForRole(item.role, plan.manifest)
		if err != nil {
			return receipts, err
		}
		transfer, uploadErr := objectstore.UploadGCS(ctx, client, objectstore.UploadRequest{
			Source: item.source, ContentType: contentType, Target: target,
			Integrity: item.integrity, Policy: plan.policy,
		})
		if uploadErr != nil {
			return receipts, &UploadError{Transfer: transfer, Role: item.role, Cause: uploadErr}
		}
		receipt := Receipt{
			transfer: transfer, commitment: item.commitment,
			role: item.role, valid: true,
		}
		if err := receipt.Validate(); err != nil {
			return receipts, &UploadError{Transfer: transfer, Role: item.role, Cause: err}
		}
		receipts.values[index] = receipt
		receipts.count++
	}
	return receipts, nil
}

func contentTypeForRole(
	role ObjectRole,
	manifest release.VerifiedManifest,
) (core.HTTPMediaType, error) {
	switch {
	case role >= ObjectRoleWindowsAMD64 && role <= ObjectRoleLinuxARM64:
		return core.HTTPMediaTypeOctetStream(), nil
	case role == ObjectRoleManifest:
		return core.ParseHTTPMediaType("application/json")
	case role >= ObjectRoleDependencies && role <= ObjectRoleReleaseNotes:
		asset, ok := manifest.Metadata().At(int(role - ObjectRoleDependencies))
		if !ok {
			return core.HTTPMediaType{}, contractError(errors.New("deploy metadata role is absent"))
		}
		return asset.ContentType()
	default:
		return core.HTTPMediaType{}, contractError(errors.New("deploy role has no media type"))
	}
}

func contractError(causes ...error) error {
	values := make([]error, 0, len(causes)+2)
	values = append(values, core.ErrDeployContract, core.ErrPrimitiveContract)
	values = append(values, causes...)
	return errors.Join(values...)
}

var _ core.OffWireEnum = ObjectRoleUnknown
