package release_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectversion"
	"github.com/deliri/primitive/v2026/release"
)

func TestPrimitiveReleaseIsTheSingleProjectAndGitAuthority(t *testing.T) {
	t.Parallel()

	current := release.Release()
	if err := current.Validate(); err != nil {
		t.Fatalf("release.Release().Validate() error = %v, want nil", err)
	}
	if got, want := current.Version(), core.NewReleaseVersion(release.ReleaseMajor, release.ReleaseMinor, release.ReleasePatch); got != want {
		t.Fatalf("release.Release().Version() = %v, want %v", got, want)
	}
	if got, want := release.ReleaseTag(), current.Tag(); got != want {
		t.Fatalf("release.ReleaseTag() = %v, want %v", got, want)
	}
	if got, want := current.String(), "2026.1.2"; got != want {
		t.Fatalf("release.Release().String() = %q, want %q", got, want)
	}
	if got, want := current.Tag().String(), "v2026.1.2"; got != want {
		t.Fatalf("release.Release().Tag().String() = %q, want %q", got, want)
	}
	parsed, err := projectversion.ParseTag(release.ReleaseTag().String())
	if err != nil || parsed != release.ReleaseTag() {
		t.Fatalf("projectversion.ParseTag(current) = (%v, %v), want (%v, nil)", parsed, err, release.ReleaseTag())
	}
}
