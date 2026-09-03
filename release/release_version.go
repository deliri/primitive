package release

import "github.com/deliri/primitive/v2026/projectversion"

// These three constants are Primitive's sole release-version source of truth.
// Advancing a release means changing the intended coordinate here; manifests,
// update comparisons, display text, and Git tags must derive from Release.
const (
	ReleaseMajor uint32 = 2026
	ReleaseMinor uint32 = 1
	ReleasePatch uint32 = 2
)

// Release returns Primitive's immutable compiler-owned release declaration.
// Do not add another version string or parse Primitive's own tag back from
// text; use Version, String, and Tag on this value for the required projection.
func Release() projectversion.Release {
	return projectversion.New(ReleaseMajor, ReleaseMinor, ReleasePatch)
}

// ReleaseTag returns the exact Git tag derived from Primitive's release
// coordinates. Release automation must use this value rather than spelling a
// tag independently.
func ReleaseTag() projectversion.Tag { return Release().Tag() }
