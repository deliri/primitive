package release

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEmbeddedBuildIdentityTextBoundary(t *testing.T) {
	t.Parallel()

	valid := embeddedBuildIdentityText{
		offering: "witness",
		version:  "2026.8.2",
		commit:   "0123456789abcdef0123456789abcdef01234567",
		platform: "darwin-arm64",
	}
	got, gotErr := parseEmbeddedBuildIdentity(valid)
	if gotErr != nil || got.Offering() != core.OfferingWitness ||
		got.Version() != core.NewReleaseVersion(2026, 8, 2) ||
		got.Platform() != (core.Platform{
			OperatingSystem: core.OperatingSystemDarwin,
			Architecture:    core.CPUArchitectureARM64,
		}) {
		t.Fatalf("parseEmbeddedBuildIdentity(valid) = (%v, %v), want exact embedded facts", got, gotErr)
	}

	for _, tc := range []struct {
		name  string
		value embeddedBuildIdentityText
	}{
		{name: "offering", value: embeddedBuildIdentityText{offering: "future", version: valid.version, commit: valid.commit, platform: valid.platform}},
		{name: "version", value: embeddedBuildIdentityText{offering: valid.offering, version: "2026.08.2", commit: valid.commit, platform: valid.platform}},
		{name: "commit", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: "invalid", platform: valid.platform}},
		{name: "platform", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: valid.commit, platform: "darwin-386"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseEmbeddedBuildIdentity(tc.value)
			if got != (core.BuildIdentity{}) || !errors.Is(gotErr, core.ErrReleaseContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("parseEmbeddedBuildIdentity(%s invalid) = (%v, %v), want zero, %v, and %v", tc.name, got, gotErr, core.ErrReleaseContract, core.ErrPrimitiveContract)
			}
		})
	}
}
