package release

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
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
	if finder.tailSize > len(finder.tail) {
		t.Fatalf("artifactPatternFinder tail size = %d, want at most %d", finder.tailSize, len(finder.tail))
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

// TestArtifactInspectionRequestRejectsEveryExtentOutsideItsBound closes the
// pure boundary before any source byte is read.
func TestArtifactInspectionRequestRejectsEveryExtentOutsideItsBound(t *testing.T) {
	t.Parallel()

	build := inspectionBuildForTest(t)
	cases := []struct {
		extent  func(*testing.T) core.ByteCount
		name    string
		wantNil bool
	}{
		{
			name:    "nil source is rejected",
			extent:  func(t *testing.T) core.ByteCount { return mustByteCount(t, 16) },
			wantNil: true,
		},
		{
			name:   "unset extent is rejected",
			extent: func(*testing.T) core.ByteCount { return core.ByteCount{} },
		},
		{
			name:   "one byte above the artifact ceiling is rejected",
			extent: func(t *testing.T) core.ByteCount { return mustByteCount(t, BuiltArtifactMaximumBytes+1) },
		},
		{
			name:   "the artifact ceiling itself passes validation and fails on short bytes",
			extent: func(t *testing.T) core.ByteCount { return mustByteCount(t, BuiltArtifactMaximumBytes) },
		},
		{
			name:   "one byte extent passes validation and fails on format",
			extent: func(t *testing.T) core.ByteCount { return mustByteCount(t, 1) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := ArtifactInspectionRequest{
				Extent: tc.extent(t), Build: build,
				LinkerAssignments: emptyLinkerAssignmentsForTest(),
			}
			if !tc.wantNil {
				request.Source = bytes.NewReader(make([]byte, 16))
			}
			_, gotErr := InspectBuiltArtifact(request)
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
					Offering: core.OfferingBug, Version: core.NewReleaseVersion(2026, 0, 11),
					Commit: inspectionCommitForTest(t), Platform: platform,
				})
				if err != nil {
					t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
				}
				_, gotErr := InspectBuiltArtifact(ArtifactInspectionRequest{
					Source: bytes.NewReader(payload.content),
					Extent: mustByteCount(t, uint64(len(payload.content))),
					Build:  build, LinkerAssignments: emptyLinkerAssignmentsForTest(),
				})
				if !errors.Is(gotErr, core.ErrReleaseContract) {
					t.Fatalf("InspectBuiltArtifact() error = %v, want %v", gotErr, core.ErrReleaseContract)
				}
			})
		}
	}
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
		Offering: core.OfferingBug, Version: core.NewReleaseVersion(2026, 0, 11),
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
