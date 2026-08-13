package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// ProjectVersion is the exact current Primitive Git tag. Its closed domain has
// one member so no caller can invent a second current-version authority.
type ProjectVersion string

// PrimitiveVersion is the single compiler-owned Primitive project and Git
// release version. Publication derives the Git tag from this value directly.
const PrimitiveVersion ProjectVersion = "v2026.0.95"

// Validate accepts only the one current compiler-owned tag and proves its
// suffix through Core's release-version parser.
func (v ProjectVersion) Validate() error {
	if v != PrimitiveVersion {
		return contractError(errors.New("primitive project version differs from its compiler authority"))
	}
	text := string(v)
	if len(text) < 2 || text[0] != 'v' {
		return contractError(errors.New("primitive project version is not a go release tag"))
	}
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte(text[1:])); err != nil {
		return contractError(errors.New("primitive project version is invalid"), err)
	}
	return nil
}

// ParseProjectVersion admits only the exact current compiler-owned tag.
func ParseProjectVersion(value string) (ProjectVersion, error) {
	version := ProjectVersion(value)
	if err := version.Validate(); err != nil {
		return ProjectVersion(""), err
	}
	return version, nil
}

// String returns the exact Git tag or empty text for an invalid value.
func (v ProjectVersion) String() string {
	if v.Validate() != nil {
		return ""
	}
	return string(v)
}

// ReleaseVersion projects the current tag into the typed version embedded in
// release manifests and build identities.
func (v ProjectVersion) ReleaseVersion() (core.ReleaseVersion, error) {
	if err := v.Validate(); err != nil {
		return core.ReleaseVersion{}, err
	}
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte(string(v)[1:])); err != nil {
		return core.ReleaseVersion{}, contractError(err)
	}
	return version, nil
}

var _ core.Validatable = PrimitiveVersion
