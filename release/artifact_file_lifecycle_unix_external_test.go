//go:build unix

package release_test

import (
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

func TestBuiltArtifactFileInspectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mode    os.FileMode
		zero    bool
		wantErr error
	}{
		{name: "executable closes into exact artifact authority", mode: 0o700},
		{name: "readable bytes without executable standing cannot become authority", mode: 0o600, wantErr: core.ErrProcessContract},
		{name: "inaccessible bytes cannot become authority", wantErr: core.ErrFilestoreSource},
		{name: "zero intent cannot acquire artifact authority", zero: true, wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var request release.ArtifactInspectionRequest
			if !tc.zero {
				build := inspectionFileTriadBuild(t)
				path := buildInspectionFixture(t, buildInspectionFixtureRequest{
					Directory: inspectionAbsolutePath(t, t.TempDir()), Build: build,
					ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
				})
				location := releaseFixtureLocation(t, path)
				held, err := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: location})
				if err != nil {
					t.Fatalf("filestore.OpenUpdate(permission fixture) error = %v, want nil", err)
				}
				// The native handle permits hostile mode zero, which Filestore's
				// durable permission request intentionally refuses to create.
				if err := errors.Join(held.Chmod(tc.mode), held.Close()); err != nil {
					t.Fatalf("permission fixture Chmod/Close() error = %v, want nil", err)
				}
				request = release.ArtifactInspectionRequest{Path: path, Build: build, LinkerAssignments: mustInspectionAssignments(t, inspectionProductValue)}
			}
			got, err := release.InspectBuiltArtifact(t.Context(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("InspectBuiltArtifact() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, core.ErrReleaseContract) || got != (release.Artifact{}) {
					t.Fatalf("refused artifact = (%v, %v), want zero and Release refusal", got, err)
				}
				return
			}
			sha, crc, length := digestInspectionFixture(t, request.Path)
			wantExtent := mustInspectionExtent(t, length.Uint64())
			if got.Validate() != nil || got.Build() != request.Build || got.Integrity().SHA256() != sha || got.Integrity().CRC32C() != crc || got.Integrity().Extent() != wantExtent {
				t.Fatalf("admitted artifact = %v, want build %v and exact independently read hashes/extent (%v, %v, %v)", got, request.Build, sha, crc, length)
			}
		})
	}
}

func BenchmarkInspectBuiltArtifactRealExecutable(b *testing.B) {
	b.ReportAllocs()
	benchmarkInspectBuiltArtifactRealExecutable(b, 0)
}

func BenchmarkInspectBuiltArtifactRealExecutablePlusTenMiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkInspectBuiltArtifactRealExecutable(b, 10<<20)
}

func benchmarkInspectBuiltArtifactRealExecutable(b *testing.B, padding int64) {
	b.Helper()

	build := inspectionFileTriadBuild(b)
	assignments := mustInspectionAssignments(b, inspectionProductValue)
	path := buildInspectionFixture(b, buildInspectionFixtureRequest{
		Directory: inspectionAbsolutePath(b, b.TempDir()), Build: build,
		ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
	})
	location := releaseFixtureLocation(b, path)
	held, err := filestore.OpenUpdate(b.Context(), filestore.UpdateHandleRequest{Location: location})
	if err != nil {
		b.Fatalf("filestore.OpenUpdate(benchmark fixture) error = %v, want nil", err)
	}
	b.Cleanup(func() {
		if err := held.Close(); err != nil {
			b.Errorf("benchmark fixture Close() error = %v, want nil", err)
		}
	})
	info, err := held.Stat()
	if err != nil {
		b.Fatalf("benchmark fixture Stat() error = %v, want nil", err)
	}
	if err := held.Truncate(info.Size() + padding); err != nil {
		b.Fatalf("benchmark fixture Truncate() error = %v, want nil", err)
	}
	info, err = held.Stat()
	if err != nil {
		b.Fatalf("padded benchmark fixture Stat() error = %v, want nil", err)
	}
	request := release.ArtifactInspectionRequest{
		Path: path, Build: build, LinkerAssignments: assignments,
	}
	sha, crc, length := digestInspectionFixture(b, path)
	want, err := release.NewArtifact(release.ArtifactRequest{
		Build: build, Extent: mustInspectionExtent(b, length.Uint64()), SHA256: sha, CRC32C: crc,
	})
	if err != nil || length.Uint64() != uint64(info.Size()) {
		b.Fatalf("benchmark oracle = (%v, %v), want the exact %d-byte fixture", want, err, info.Size())
	}
	shaText, err := sha.Hex()
	if err != nil {
		b.Fatalf("benchmark fixture SHA256.Hex error = %v, want nil", err)
	}
	b.Logf("artifact fixture bytes=%d sha256=%s", length.Uint64(), shaText)
	b.ReportAllocs()
	b.SetBytes(info.Size())
	b.ResetTimer()
	for b.Loop() {
		artifact, err := release.InspectBuiltArtifact(b.Context(), request)
		if err != nil || artifact != want {
			b.Fatalf("release.InspectBuiltArtifact() = (%v, %v), want exact independently hashed artifact %v", artifact, err, want)
		}
	}
}

func inspectionFileTriadBuild(t testing.TB) core.BuildIdentity {
	t.Helper()
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: releaseExternalOffering(t, 2), Version: core.NewReleaseVersion(2026, 0, 11), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	return build
}
