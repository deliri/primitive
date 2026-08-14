//go:build unix

package release_test

import (
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

func TestBuiltArtifactFileInspectionLayerTriad(t *testing.T) {
	t.Parallel()
	// This local lifecycle triad complements, and does not replace, the package
	// pressure matrices: twelve accepted real build/strip combinations, more
	// than thirty typed refusals, and exact path/extent/format/stamp boundaries.

	t.Run("positive executable file closes into exact artifact authority", func(t *testing.T) {
		t.Parallel()

		build := inspectionFileTriadBuild(t)
		assignments := mustInspectionAssignments(t, inspectionProductValue)
		path := buildInspectionFixture(t, buildInspectionFixtureRequest{
			Directory: inspectionAbsolutePath(t, t.TempDir()), Build: build,
			ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
		})
		got, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{
			Path: path, Build: build, LinkerAssignments: assignments,
		})
		if gotErr != nil || got.Validate() != nil || got.Build() != build {
			t.Fatalf("release.InspectBuiltArtifact(executable file) = (%v, %v), want exact valid artifact and nil", got, gotErr)
		}
	})

	t.Run("negative native bytes without executable standing acquire no artifact authority", func(t *testing.T) {
		t.Parallel()

		build := inspectionFileTriadBuild(t)
		assignments := mustInspectionAssignments(t, inspectionProductValue)
		path := buildInspectionFixture(t, buildInspectionFixtureRequest{
			Directory: inspectionAbsolutePath(t, t.TempDir()), Build: build,
			ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
		})
		if err := os.Chmod(path.String(), 0o600); err != nil {
			t.Fatalf("os.Chmod(nonexecutable artifact) error = %v, want nil", err)
		}
		got, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{
			Path: path, Build: build, LinkerAssignments: assignments,
		})
		if !errors.Is(gotErr, core.ErrReleaseContract) || !errors.Is(gotErr, core.ErrProcessContract) ||
			got != (release.Artifact{}) {
			t.Fatalf("release.InspectBuiltArtifact(nonexecutable file) = (%v, %v), want exact zero, %v, and %v", got, gotErr, core.ErrReleaseContract, core.ErrProcessContract)
		}
	})

	t.Run("neutral zero request acquires no artifact authority", func(t *testing.T) {
		t.Parallel()

		got, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{})
		if !errors.Is(gotErr, core.ErrReleaseContract) || got != (release.Artifact{}) {
			t.Fatalf("release.InspectBuiltArtifact(zero) = (%v, %v), want exact zero and %v", got, gotErr, core.ErrReleaseContract)
		}
	})
}

func inspectionFileTriadBuild(t testing.TB) core.BuildIdentity {
	t.Helper()
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.OfferingWitness, Version: core.NewReleaseVersion(2026, 0, 11), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	return build
}
