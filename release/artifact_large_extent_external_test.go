package release_test

import (
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/release"
)

func TestBuiltArtifactDigestAuthenticatesNativeExecutableBeyondFormerCeiling(t *testing.T) {
	t.Parallel()
	directory := inspectionAbsolutePath(t, t.TempDir())
	build := inspectionFileTriadBuild(t)
	path := buildInspectionFixture(t, buildInspectionFixtureRequest{Directory: directory, Build: build, ProductValue: inspectionProductValue, StripFlags: releaseStripFlags})
	const size = 256<<20 + 1
	// Native truncation creates a sparse zero tail without materializing it.
	// Go's executable parser must still admit the original stripped format.
	if err := os.Truncate(path.String(), size); err != nil {
		t.Fatal(err)
	}
	got, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{Path: path, Build: build, LinkerAssignments: mustInspectionAssignments(t, inspectionProductValue)})
	wantSHA, wantCRC, wantLength := digestInspectionFixture(t, path)
	gotExtent, extentErr := got.Integrity().Extent().Uint64()
	if gotErr != nil || extentErr != nil || got.Build() != build || gotExtent != size || wantLength.Uint64() != size || got.Integrity().SHA256() != wantSHA || got.Integrity().CRC32C() != wantCRC {
		t.Fatalf("native executable inspection = (%v,%v), want exact %d-byte extent and native integrity", got, gotErr, size)
	}
}
