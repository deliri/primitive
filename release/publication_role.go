package release

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// PublicationObjectCount is the exact immutable object count in one complete
// release publication: four executables, the signed manifest, and three
// metadata assets.
const PublicationObjectCount = TargetCount + 1 + MetadataAssetCount

// PublicationRole is the closed canonical order of objects in one release
// publication. Release owns the order because both the distribution agreement
// and the deployment effect must bind the same manifest slots.
type PublicationRole uint8

const (
	PublicationRoleUnknown PublicationRole = iota
	PublicationRoleWindowsAMD64
	PublicationRoleDarwinARM64
	PublicationRoleLinuxAMD64
	PublicationRoleLinuxARM64
	PublicationRoleManifest
	PublicationRoleDependencies
	PublicationRoleDocumentation
	PublicationRoleReleaseNotes
	publicationRoleLimit
)

func publicationRoleLabels() [publicationRoleLimit]string {
	return [...]string{
		PublicationRoleUnknown:       "",
		PublicationRoleWindowsAMD64:  "windows_amd64",
		PublicationRoleDarwinARM64:   "darwin_arm64",
		PublicationRoleLinuxAMD64:    "linux_amd64",
		PublicationRoleLinuxARM64:    "linux_arm64",
		PublicationRoleManifest:      "manifest",
		PublicationRoleDependencies:  MetadataKindDependencies.String(),
		PublicationRoleDocumentation: MetadataKindDocumentation.String(),
		PublicationRoleReleaseNotes:  MetadataKindReleaseNotes.String(),
	}
}

// Validate rejects the unset role and every value outside the complete
// publication order.
func (r PublicationRole) Validate() error {
	if r <= PublicationRoleUnknown || r >= publicationRoleLimit || publicationRoleLabels()[r] == "" {
		return contractError(errors.New("publication role is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed publication-role domain.
func (r PublicationRole) IsValid() bool { return r.Validate() == nil }

// String returns the canonical role label or Core's unknown diagnostic.
func (r PublicationRole) String() string {
	if !r.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return publicationRoleLabels()[r]
}

// Index returns the fixed zero-based publication slot owned by Release.
func (r PublicationRole) Index() (int, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	return int(r - 1), nil
}

// PublicationRoleAt returns the role occupying one fixed publication slot.
func PublicationRoleAt(index int) (PublicationRole, bool) {
	if index < 0 || index >= PublicationObjectCount {
		return PublicationRoleUnknown, false
	}
	role := PublicationRole(index + 1)
	return role, role.Validate() == nil
}

// MarshalJSON emits the canonical role token.
func (r PublicationRole) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(r.String())
}

// UnmarshalJSON accepts only one exact canonical role token without mutating
// the receiver on rejection.
func (r *PublicationRole) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("publication role receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	for candidate := PublicationRoleUnknown + 1; candidate < publicationRoleLimit; candidate++ {
		if candidate.String() == value {
			canonical, marshalErr := json.Marshal(value)
			if marshalErr != nil || string(canonical) != string(data) {
				return jsonError(errors.New("publication role is not canonical"), marshalErr)
			}
			*r = candidate
			return nil
		}
	}
	return jsonError(errors.New("publication role is unsupported"))
}

var (
	_ core.Validatable            = PublicationRoleUnknown
	_ core.ValidatedJSONMarshaler = PublicationRole(0)
	_ json.Unmarshaler            = (*PublicationRole)(nil)
)
