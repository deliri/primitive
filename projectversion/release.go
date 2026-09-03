package projectversion

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Release is one immutable project release declaration.
//
// Projects construct it from their core-owned ReleaseMajor, ReleaseMinor, and
// ReleasePatch constants. Its unexported representation prevents callers from
// inventing alternate formatting or changing a release after publication.
type Release struct {
	version core.ReleaseVersion
}

// New constructs a project release from its three compiler-owned coordinates.
func New(major, minor, patch uint32) Release {
	return Release{version: core.NewReleaseVersion(major, minor, patch)}
}

// Validate proves that the release crossed the projectversion constructor.
func (r Release) Validate() error {
	if err := r.version.Validate(); err != nil {
		return errors.Join(core.ErrReleaseContract, err)
	}
	return nil
}

// Version returns the typed coordinate used by release manifests and update
// ordering. An invalid Release returns the zero coordinate rather than
// manufacturing evidence from an unconstructed value.
func (r Release) Version() core.ReleaseVersion {
	if r.Validate() != nil {
		return core.ReleaseVersion{}
	}
	return r.version
}

// String returns the canonical unprefixed release text used for display and
// protocol fields that carry a version rather than a Git reference.
func (r Release) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.version.String()
}

// Tag derives the exact typed Git tag from this release. Git-facing code must
// use this method instead of concatenating or copying the "v" convention.
func (r Release) Tag() Tag {
	if r.Validate() != nil {
		return Tag{}
	}
	return Tag{release: r}
}

// Compare orders two validated project releases by major, minor, then patch.
func (r Release) Compare(other Release) (core.Comparison, error) {
	if err := errors.Join(r.Validate(), other.Validate()); err != nil {
		return core.ComparisonUnknown, err
	}
	return r.version.Compare(other.version)
}

var _ core.Validatable = Release{}
