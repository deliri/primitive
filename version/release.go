package version

import (
	"errors"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
)

// Release is one immutable project release derived from Compass.
type Release struct {
	version core.ReleaseVersion
}

// FromProject derives the sole release identity from a validated Compass
// project declaration.
func FromProject(project compass.Project) (Release, error) {
	if err := project.Validate(); err != nil {
		return Release{}, errors.Join(core.ErrReleaseContract, err)
	}
	coordinates := project.Release
	release := Release{version: core.NewReleaseVersion(coordinates.Major, coordinates.Minor, coordinates.Patch)}
	if err := release.Validate(); err != nil {
		return Release{}, err
	}
	return release, nil
}

// Validate proves that the release contains valid typed coordinates.
func (r Release) Validate() error {
	if err := r.version.Validate(); err != nil {
		return errors.Join(core.ErrReleaseContract, err)
	}
	return nil
}

// Version returns the typed coordinate used by release manifests and ordering.
func (r Release) Version() core.ReleaseVersion {
	if r.Validate() != nil {
		return core.ReleaseVersion{}
	}
	return r.version
}

// String returns the canonical unprefixed release text.
func (r Release) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.version.String()
}

// Tag derives the exact typed Git tag from this release.
func (r Release) Tag() Tag {
	if r.Validate() != nil {
		return Tag{}
	}
	return Tag{release: r}
}

// Compare orders two validated releases by major, minor, then patch.
func (r Release) Compare(other Release) (core.Comparison, error) {
	if err := errors.Join(r.Validate(), other.Validate()); err != nil {
		return core.ComparisonUnknown, err
	}
	return r.version.Compare(other.version)
}

var _ core.Validatable = Release{}
