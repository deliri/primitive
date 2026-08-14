//go:build unix

package release_test

import (
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

const (
	inspectionModeExecutable uint8 = iota
	inspectionModeReadable
	inspectionModeAbsent
	inspectionModeReadOnly
	inspectionModeCount
)

const (
	inspectionBindingExact uint8 = iota
	inspectionBindingArchitecture
	inspectionBindingOffering
	inspectionBindingAssignment
	inspectionBindingCount
)

// FuzzInspectBuiltArtifactFileSemanticClosure ratchets the previously
// unpinned filesystem-content ingress. Every admitted mutation must close to
// the exact independently observed extent, digests, and build; every refusal
// must return no Artifact authority and leave the caller-owned file unchanged.
func FuzzInspectBuiltArtifactFileSemanticClosure(f *testing.F) {
	build := inspectionFileTriadBuild(f)
	directory := inspectionAbsolutePath(f, f.TempDir())
	seedPath := buildInspectionFixture(f, buildInspectionFixtureRequest{
		Directory: directory, Build: build,
		ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
	})
	canonical, err := os.ReadFile(seedPath.String())
	if err != nil {
		f.Fatalf("os.ReadFile(canonical built artifact) error = %v, want nil", err)
	}
	seedArtifactInspectionFuzzCorpus(f, canonical)

	f.Fuzz(func(t *testing.T, data []byte, modeSelector uint8, bindingSelector uint8) {
		directory := t.TempDir()
		mode := inspectionFuzzMode(modeSelector)
		path := writeInspectionFuzzFile(t, inspectionFuzzFileWrite{
			Directory: directory, Data: data, Mode: mode,
		})
		request := inspectionFuzzRequest(t, inspectionFuzzRequestInput{
			Path: path, Build: build, BindingSelector: bindingSelector,
		})

		got, gotErr := release.InspectBuiltArtifact(t.Context(), request)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrReleaseContract) || got != (release.Artifact{}) {
				t.Fatalf("release.InspectBuiltArtifact(fuzz refusal) = (%v, %v), want exact zero and %v", got, gotErr, core.ErrReleaseContract)
			}
			if modeSelector%inspectionModeCount != inspectionModeExecutable && len(data) > 0 &&
				uint64(len(data)) <= release.BuiltArtifactMaximumBytes &&
				!errors.Is(gotErr, core.ErrProcessContract) {
				t.Fatalf("release.InspectBuiltArtifact(nonexecutable fuzz file) error = %v, want %v", gotErr, core.ErrProcessContract)
			}
			proveInspectionFuzzFileUnchanged(t, inspectionFuzzFileProof{
				Path: path, Data: data, WantMode: mode,
			})
			return
		}

		if err := got.Validate(); err != nil {
			t.Fatalf("release.InspectBuiltArtifact(fuzz accepted).Validate() error = %v, want nil", err)
		}
		if modeSelector%inspectionModeCount != inspectionModeExecutable {
			t.Fatalf("release.InspectBuiltArtifact(fuzz) authenticated nonexecutable mode selector %d", modeSelector)
		}
		if bindingSelector%inspectionBindingCount != inspectionBindingExact {
			t.Fatalf("release.InspectBuiltArtifact(fuzz) authenticated foreign binding selector %d", bindingSelector)
		}
		if got.Build() != request.Build {
			t.Fatalf("release.InspectBuiltArtifact(fuzz accepted).Build() = %v, want %v", got.Build(), request.Build)
		}
		wantExtent, err := core.NewByteCount(uint64(len(data)))
		if err != nil {
			t.Fatalf("core.NewByteCount(fuzz accepted extent) error = %v, want nil", err)
		}
		wantSHA := core.NewSHA256Digest(sha256.Sum256(data))
		wantCRC := core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
		if got.Integrity().Extent() != wantExtent || got.Integrity().SHA256() != wantSHA ||
			got.Integrity().CRC32C() != wantCRC {
			t.Fatalf("release.InspectBuiltArtifact(fuzz accepted) integrity does not match independent file bytes")
		}
		second, secondErr := release.InspectBuiltArtifact(t.Context(), request)
		if secondErr != nil || second != got {
			t.Fatalf("release.InspectBuiltArtifact(fuzz second closure) = (%v, %v), want (%v, nil)", second, secondErr, got)
		}
		proveInspectionFuzzFileUnchanged(t, inspectionFuzzFileProof{
			Path: path, Data: data, WantMode: mode,
		})
	})
}

func seedArtifactInspectionFuzzCorpus(f *testing.F, canonical []byte) {
	f.Helper()
	f.Add(canonical, inspectionModeExecutable, inspectionBindingExact)
	f.Add(canonical, inspectionModeReadable, inspectionBindingExact)
	f.Add(canonical, inspectionModeExecutable, inspectionBindingArchitecture)
	f.Add(canonical, inspectionModeExecutable, inspectionBindingOffering)
	f.Add(canonical, inspectionModeExecutable, inspectionBindingAssignment)
	f.Add([]byte{}, inspectionModeExecutable, inspectionBindingExact)
	f.Add(canonical[:1], inspectionModeExecutable, inspectionBindingExact)
	f.Add(canonical[:len(canonical)-1], inspectionModeExecutable, inspectionBindingExact)
}

func inspectionFuzzMode(selector uint8) os.FileMode {
	switch selector % inspectionModeCount {
	case inspectionModeExecutable:
		return 0o700
	case inspectionModeReadable:
		return 0o600
	case inspectionModeAbsent:
		return 0
	case inspectionModeReadOnly:
		return 0o400
	default:
		panic("unreachable inspection mode")
	}
}

type inspectionFuzzRequestInput struct {
	Path            core.AbsolutePath
	Build           core.BuildIdentity
	BindingSelector uint8
}

func inspectionFuzzRequest(t *testing.T, input inspectionFuzzRequestInput) release.ArtifactInspectionRequest {
	t.Helper()
	build := input.Build
	assignments := mustInspectionAssignments(t, inspectionProductValue)
	switch input.BindingSelector % inspectionBindingCount {
	case inspectionBindingExact:
	case inspectionBindingArchitecture:
		build = mustInspectionBuild(t, input.Build, core.CPUArchitectureAMD64)
	case inspectionBindingOffering:
		build = inspectionFuzzOfferingBuild(t, input.Build)
	case inspectionBindingAssignment:
		assignments = mustInspectionAssignments(t, "foreign-product-stamp-73")
	default:
		panic("unreachable inspection binding")
	}
	return release.ArtifactInspectionRequest{Path: input.Path, Build: build, LinkerAssignments: assignments}
}

func inspectionFuzzOfferingBuild(t *testing.T, base core.BuildIdentity) core.BuildIdentity {
	t.Helper()
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.OfferingBug, Version: base.Version(), Commit: base.Commit(), Platform: base.Platform(),
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity(foreign offering) error = %v, want nil", err)
	}
	return build
}

type inspectionFuzzFileWrite struct {
	Directory string
	Data      []byte
	Mode      os.FileMode
}

func writeInspectionFuzzFile(t *testing.T, request inspectionFuzzFileWrite) core.AbsolutePath {
	t.Helper()
	path := inspectionAbsolutePath(t, filepath.Join(request.Directory, "artifact"))
	if err := os.WriteFile(path.String(), request.Data, request.Mode); err != nil {
		t.Fatalf("os.WriteFile(fuzz artifact) error = %v, want nil", err)
	}
	return path
}

type inspectionFuzzFileProof struct {
	Path     core.AbsolutePath
	Data     []byte
	WantMode os.FileMode
}

func proveInspectionFuzzFileUnchanged(t *testing.T, proof inspectionFuzzFileProof) {
	t.Helper()
	info, err := os.Stat(proof.Path.String())
	if err != nil {
		t.Fatalf("os.Stat(fuzz artifact after inspection) error = %v, want nil", err)
	}
	if info.Mode().Perm() != proof.WantMode.Perm() {
		t.Fatalf("fuzz artifact mode after inspection = %v, want %v", info.Mode().Perm(), proof.WantMode.Perm())
	}
	if proof.WantMode.Perm()&0o400 == 0 {
		if err := os.Chmod(proof.Path.String(), 0o600); err != nil {
			t.Fatalf("os.Chmod(fuzz artifact oracle read) error = %v, want nil", err)
		}
	}
	got, err := os.ReadFile(proof.Path.String())
	if err != nil {
		t.Fatalf("os.ReadFile(fuzz artifact after inspection) error = %v, want nil", err)
	}
	if info.Size() != int64(len(proof.Data)) || sha256.Sum256(got) != sha256.Sum256(proof.Data) {
		t.Fatalf("fuzz artifact after inspection = (extent %d, digest %x), want (%d, %x)",
			info.Size(), sha256.Sum256(got), len(proof.Data), sha256.Sum256(proof.Data))
	}
}
