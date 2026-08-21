package release

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// TestArtifactPatternFinderFindsStampsAcrossEveryChunkBoundary proves the
// bounded carry-over window. The streaming finder never sees the whole
// executable, so a stamp split across two writes is the exact failure class
// that would silently certify an artifact missing its embedded build values.
func TestArtifactPatternFinderFindsStampsAcrossEveryChunkBoundary(t *testing.T) {
	t.Parallel()

	const stamp = "2026.0.11-b5c32d95"
	filler := strings.Repeat("\x00\xff", 1024)

	for split := range len(stamp) + 1 {
		t.Run("stamp split after "+strconv.Itoa(split)+" bytes", func(t *testing.T) {
			t.Parallel()

			var patterns [artifactInspectionPatternCount]string
			patterns[0] = stamp
			finder := newArtifactPatternFinder(patterns)
			for _, chunk := range [][]byte{
				[]byte(filler + stamp[:split]),
				[]byte(stamp[split:] + filler),
			} {
				written, err := finder.Write(chunk)
				if err != nil || written != len(chunk) {
					t.Fatalf("artifactPatternFinder.Write() = (%d, %v), want (%d, nil)", written, err, len(chunk))
				}
			}
			if err := finder.Validate(); err != nil {
				t.Fatalf("artifactPatternFinder.Validate() error = %v, want nil for a split stamp", err)
			}
		})
	}
}

// TestArtifactPatternFinderHoldsTheLongestAdmittedValueAcrossWrites drives the
// carry-over window at its exact declared bound: a maximum-length linker value
// split one byte from its start.
func TestArtifactPatternFinderHoldsTheLongestAdmittedValueAcrossWrites(t *testing.T) {
	t.Parallel()

	longest := strings.Repeat("v", linkerValueMaximumBytes)
	if len(longest) != linkerValueMaximumBytes {
		t.Fatalf("longest admitted value = %d bytes, want %d", len(longest), linkerValueMaximumBytes)
	}
	var patterns [artifactInspectionPatternCount]string
	patterns[4] = longest
	finder := newArtifactPatternFinder(patterns)
	if _, err := finder.Write([]byte("prefix" + longest[:1])); err != nil {
		t.Fatalf("artifactPatternFinder.Write(head) error = %v, want nil", err)
	}
	if _, err := finder.Write([]byte(longest[1:] + "suffix")); err != nil {
		t.Fatalf("artifactPatternFinder.Write(tail) error = %v, want nil", err)
	}
	if err := finder.Validate(); err != nil {
		t.Fatalf("artifactPatternFinder.Validate() error = %v, want nil for a split maximum value", err)
	}
	if finder.tailSize >= linkerValueMaximumBytes {
		t.Fatalf("artifactPatternFinder tail size = %d, want below %d", finder.tailSize, linkerValueMaximumBytes)
	}
}

// TestArtifactPatternFinderAllocationsStayFlatAcrossRepresentativeScale
// ratchets the streaming ownership contract. The same bounded finder storage
// must serve one transfer buffer and a representative ten-MiB artifact; a
// per-write window allocation makes the larger observation fail by hundreds.
func TestArtifactPatternFinderAllocationsStayFlatAcrossRepresentativeScale(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	const representativeChunks = (10 << 20) / artifactInspectionBufferBytes

	var patterns [artifactInspectionPatternCount]string
	patterns[0] = "embedded-release-stamp"
	chunk := bytes.Repeat([]byte("embedded-release-stamp"),
		artifactInspectionBufferBytes/len(patterns[0]))
	chunk = append(chunk, bytes.Repeat([]byte{0x5a}, artifactInspectionBufferBytes-len(chunk))...)
	allocations := func(writes int) (float64, error) {
		var observedErr error
		count := testing.AllocsPerRun(5, func() {
			finder := newArtifactPatternFinder(patterns)
			for range writes {
				written, err := finder.Write(chunk)
				if err != nil || written != len(chunk) {
					observedErr = errors.Join(observedErr, err, core.ErrReleaseContract)
					return
				}
			}
			observedErr = errors.Join(observedErr, finder.Validate())
		})
		return count, observedErr
	}

	oneChunk, err := allocations(1)
	if err != nil {
		t.Fatalf("one-buffer pattern scan error = %v, want nil", err)
	}
	representative, err := allocations(representativeChunks)
	if err != nil {
		t.Fatalf("ten-MiB pattern scan error = %v, want nil", err)
	}
	if representative > oneChunk+1 {
		t.Fatalf("ten-MiB pattern scan allocations = %.0f, want at most one above one-buffer %.0f", representative, oneChunk)
	}
}

// TestArtifactPatternFinderRejectsEveryAbsentOrNearMissStamp proves the finder
// never accepts a value it did not actually observe.
func TestArtifactPatternFinderRejectsEveryAbsentOrNearMissStamp(t *testing.T) {
	t.Parallel()

	const stamp = "product-stamp-41"
	cases := []struct {
		wantErr error
		name    string
		content string
	}{
		{name: "exact stamp is found", content: "aaa" + stamp + "bbb"},
		{name: "stamp at the first byte is found", content: stamp + strings.Repeat("z", 64)},
		{name: "stamp at the last byte is found", content: strings.Repeat("z", 64) + stamp},
		{name: "empty stream omits the stamp", content: "", wantErr: core.ErrReleaseContract},
		{name: "one byte short prefix omits the stamp", content: stamp[:len(stamp)-1], wantErr: core.ErrReleaseContract},
		{name: "one byte short suffix omits the stamp", content: stamp[1:], wantErr: core.ErrReleaseContract},
		{name: "single character substitution omits the stamp", content: "product-stamp-42", wantErr: core.ErrReleaseContract},
		{name: "case change omits the stamp", content: "Product-Stamp-41", wantErr: core.ErrReleaseContract},
		{name: "interior NUL omits the stamp", content: "product-stamp\x00-41", wantErr: core.ErrReleaseContract},
		{name: "reordered halves omit the stamp", content: "stamp-41product-", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var patterns [artifactInspectionPatternCount]string
			patterns[0] = stamp
			finder := newArtifactPatternFinder(patterns)
			for chunk := range bytes.SplitSeq([]byte(tc.content), []byte("-")) {
				if _, err := finder.Write(chunk); err != nil {
					t.Fatalf("artifactPatternFinder.Write() error = %v, want nil", err)
				}
				if _, err := finder.Write([]byte("-")); err != nil {
					t.Fatalf("artifactPatternFinder.Write(separator) error = %v, want nil", err)
				}
			}
			gotErr := finder.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("artifactPatternFinder.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

// TestInspectBuiltArtifactRejectsEveryObservedExtentOutsideItsBound proves the
// file's real Filestore observation, not a caller claim, owns the ceiling.
func TestInspectBuiltArtifactRejectsEveryObservedExtentOutsideItsBound(t *testing.T) {
	t.Parallel()

	build := inspectionBuildForTest(t)
	cases := []struct {
		name     string
		extent   uint64
		zeroPath bool
	}{
		{name: "unset path is rejected", zeroPath: true},
		{name: "empty file is rejected", extent: 0},
		{name: "one byte file reaches native format refusal", extent: 1},
		{name: "one byte below the artifact ceiling reaches native format refusal", extent: BuiltArtifactMaximumBytes - 1},
		{name: "the artifact ceiling reaches native format refusal", extent: BuiltArtifactMaximumBytes},
		{name: "one byte above the artifact ceiling is rejected", extent: BuiltArtifactMaximumBytes + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := ArtifactInspectionRequest{Build: build, LinkerAssignments: emptyLinkerAssignmentsForTest()}
			if !tc.zeroPath {
				request.Path = writeInspectionContent(t, inspectionContentRequest{
					Directory: t.TempDir(), Extent: tc.extent, Mode: 0o700,
				})
			}
			_, gotErr := InspectBuiltArtifact(t.Context(), request)
			if !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("InspectBuiltArtifact() error = %v, want %v", gotErr, core.ErrReleaseContract)
			}
		})
	}
}

// TestInspectBuiltArtifactRejectsContentThatIsNotANativeExecutable proves the
// format parsers reject arbitrary bytes for every admitted target.
func TestInspectBuiltArtifactRejectsContentThatIsNotANativeExecutable(t *testing.T) {
	t.Parallel()

	payloads := []struct {
		name    string
		content []byte
	}{
		{name: "zero bytes", content: make([]byte, 4096)},
		{name: "ascii text", content: bytes.Repeat([]byte("not an executable\n"), 256)},
		{name: "elf magic without a body", content: append([]byte("\x7fELF"), make([]byte, 512)...)},
		{name: "mach-o magic without a body", content: append([]byte{0xcf, 0xfa, 0xed, 0xfe}, make([]byte, 512)...)},
		{name: "pe stub without a header", content: append([]byte("MZ"), make([]byte, 512)...)},
	}
	targets := Targets()
	for index := range TargetCount {
		platform, ok := targets.At(index)
		if !ok {
			t.Fatalf("Targets().At(%d) ok = false, want true", index)
		}
		for _, payload := range payloads {
			t.Run(platform.String()+"/"+payload.name, func(t *testing.T) {
				t.Parallel()

				build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
					Offering: releaseOffering(t, 1), Version: core.NewReleaseVersion(2026, 0, 11),
					Commit: inspectionCommitForTest(t), Platform: platform,
				})
				if err != nil {
					t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
				}
				path := writeInspectionContent(t, inspectionContentRequest{
					Directory: t.TempDir(), Content: payload.content,
					Extent: uint64(len(payload.content)), Mode: 0o700,
				})
				_, gotErr := InspectBuiltArtifact(t.Context(), ArtifactInspectionRequest{
					Path: path, Build: build, LinkerAssignments: emptyLinkerAssignmentsForTest(),
				})
				if !errors.Is(gotErr, core.ErrReleaseContract) {
					t.Fatalf("InspectBuiltArtifact() error = %v, want %v", gotErr, core.ErrReleaseContract)
				}
			})
		}
	}
}

type inspectionContentRequest struct {
	Directory string
	Content   []byte
	Extent    uint64
	Mode      os.FileMode
}

func writeInspectionContent(t *testing.T, request inspectionContentRequest) core.AbsolutePath {
	t.Helper()
	path := filepath.Join(request.Directory, "artifact")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, request.Mode)
	if err != nil {
		t.Fatalf("os.OpenFile(inspection content) error = %v, want nil", err)
	}
	if len(request.Content) != 0 {
		written, writeErr := file.Write(request.Content)
		if writeErr != nil || written != len(request.Content) {
			_ = file.Close()
			t.Fatalf("inspection content Write() = (%d, %v), want (%d, nil)", written, writeErr, len(request.Content))
		}
	}
	truncateErr := file.Truncate(int64(request.Extent))
	closeErr := file.Close()
	if err := errors.Join(truncateErr, closeErr); err != nil {
		t.Fatalf("inspection content finalize error = %v, want nil", err)
	}
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(inspection content) error = %v, want nil", err)
	}
	return absolute
}

// TestLinkerAssignmentsRejectEveryCorruptedStorageShape proves the fixed
// storage invariants that no exported constructor can produce.
func TestLinkerAssignmentsRejectEveryCorruptedStorageShape(t *testing.T) {
	t.Parallel()

	first, err := NewLinkerAssignment("github.com/x/y.alpha", "1")
	if err != nil {
		t.Fatalf("NewLinkerAssignment(alpha) error = %v, want nil", err)
	}
	second, err := NewLinkerAssignment("github.com/x/y.beta", "2")
	if err != nil {
		t.Fatalf("NewLinkerAssignment(beta) error = %v, want nil", err)
	}

	cases := []struct {
		corrupt func() LinkerAssignments
		name    string
	}{
		{
			name: "negative count is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{count: -1}
			},
		},
		{
			name: "count beyond storage is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{count: linkerAssignmentMaximumCount + 1}
			},
		},
		{
			name: "count claiming unset slots is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{values: [linkerAssignmentMaximumCount]LinkerAssignment{first}, count: 2}
			},
		},
		{
			name: "descending symbol order is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{
					values: [linkerAssignmentMaximumCount]LinkerAssignment{second, first}, count: 2,
				}
			},
		},
		{
			name: "duplicated symbol is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{
					values: [linkerAssignmentMaximumCount]LinkerAssignment{first, first}, count: 2,
				}
			},
		},
		{
			name: "nonzero padding beyond the count is rejected",
			corrupt: func() LinkerAssignments {
				value := LinkerAssignments{values: [linkerAssignmentMaximumCount]LinkerAssignment{first}, count: 1}
				value.values[7] = second
				return value
			},
		},
		{
			name: "invalid stored assignment is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{
					values: [linkerAssignmentMaximumCount]LinkerAssignment{{symbol: "nopackage", value: "1"}},
					count:  1,
				}
			},
		},
		{
			name: "stored Primitive-owned symbol is rejected",
			corrupt: func() LinkerAssignments {
				return LinkerAssignments{
					values: [linkerAssignmentMaximumCount]LinkerAssignment{
						{symbol: EmbeddedBuildVersionLinkSymbol, value: "2026.0.11"},
					},
					count: 1,
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			corrupted := tc.corrupt()
			if gotErr := corrupted.Validate(); !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("LinkerAssignments.Validate() error = %v, want %v", gotErr, core.ErrReleaseContract)
			}
			if _, ok := corrupted.At(0); ok {
				t.Fatal("LinkerAssignments.At(0) ok = true, want false for corrupted storage")
			}
		})
	}
}

func inspectionCommitForTest(t *testing.T) core.BuildCommit {
	t.Helper()
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	return commit
}

func inspectionBuildForTest(t *testing.T) core.BuildIdentity {
	t.Helper()
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: releaseOffering(t, 1), Version: core.NewReleaseVersion(2026, 0, 11),
		Commit: inspectionCommitForTest(t),
		Platform: core.Platform{
			OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64,
		},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	return build
}
