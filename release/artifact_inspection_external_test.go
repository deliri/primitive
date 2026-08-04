package release_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

const (
	inspectionProductSymbol = "github.com/deliri/primitive/v2026/testdata/releasestamp.Value"
	// releaseStripFlags is the exact linker stripping that BuildCommand emits.
	releaseStripFlags = "-w -s"
)

func TestInspectBuiltArtifactProvesEveryShippedExecutable(t *testing.T) {
	t.Parallel()

	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v", err)
	}
	version := core.NewReleaseVersion(2026, 0, 11)
	assignment, err := release.NewLinkerAssignment(inspectionProductSymbol, "product-stamp-41")
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment() error = %v", err)
	}
	assignments, err := release.NewLinkerAssignments([]release.LinkerAssignment{assignment})
	if err != nil {
		t.Fatalf("release.NewLinkerAssignments() error = %v", err)
	}
	targets := release.Targets()
	for index := range release.TargetCount {
		platform, ok := targets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false", index)
		}
		t.Run(platform.String(), func(t *testing.T) {
			t.Parallel()

			build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
				Offering: core.OfferingWitness, Version: version, Commit: commit, Platform: platform,
			})
			if err != nil {
				t.Fatalf("core.NewBuildIdentity() error = %v", err)
			}
			path := buildInspectionFixture(t, build, "product-stamp-41", releaseStripFlags)
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("os.Open() error = %v", err)
			}
			closeInspectionFile(t, file)
			info, err := file.Stat()
			if err != nil {
				t.Fatalf("file.Stat() error = %v", err)
			}
			extent, err := core.NewByteCount(uint64(info.Size()))
			if err != nil {
				t.Fatalf("core.NewByteCount() error = %v", err)
			}
			artifact, err := release.InspectBuiltArtifact(release.ArtifactInspectionRequest{
				Source: file, Extent: extent, Build: build, LinkerAssignments: assignments,
			})
			if err != nil {
				t.Fatalf("release.InspectBuiltArtifact() error = %v", err)
			}
			wantSHA, wantCRC := digestInspectionFixture(t, path)
			if artifact.Build() != build || artifact.Integrity().Extent() != extent ||
				artifact.Integrity().SHA256() != wantSHA || artifact.Integrity().CRC32C() != wantCRC {
				t.Fatalf("release.InspectBuiltArtifact() artifact does not describe exact fixture bytes")
			}
		})
	}
}

func TestInspectBuiltArtifactBreaksOnEachGuardRemoval(t *testing.T) {
	t.Parallel()

	platform := core.Platform{
		OperatingSystem: core.OperatingSystemDarwin,
		Architecture:    core.CPUArchitectureARM64,
	}
	commit, _ := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.OfferingBug, Version: core.NewReleaseVersion(2026, 0, 11),
		Commit: commit, Platform: platform,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v", err)
	}
	path := buildInspectionFixture(t, build, "product-stamp-41", releaseStripFlags)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	extent := mustInspectionExtent(t, uint64(info.Size()))
	validAssignments := mustInspectionAssignments(t, "product-stamp-41")

	cases := []struct {
		name        string
		assignments release.LinkerAssignments
		extent      core.ByteCount
		build       core.BuildIdentity
	}{
		{name: "short declared extent", extent: mustInspectionExtent(t, uint64(info.Size()-1)), build: build, assignments: validAssignments},
		{name: "missing product stamp", extent: extent, build: build, assignments: mustInspectionAssignments(t, "absent-product-stamp-99")},
		{name: "wrong architecture", extent: extent, build: mustInspectionBuild(t, build, core.CPUArchitectureAMD64), assignments: validAssignments},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("os.Open() error = %v", err)
			}
			closeInspectionFile(t, file)
			_, gotErr := release.InspectBuiltArtifact(release.ArtifactInspectionRequest{
				Source: file, Extent: tc.extent, Build: tc.build, LinkerAssignments: tc.assignments,
			})
			if !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("release.InspectBuiltArtifact() error = %v, want %v", gotErr, core.ErrReleaseContract)
			}
		})
	}
}

// TestInspectBuiltArtifactRejectsEveryUnderStrippedExecutableFormat pins the
// exact contract that -w alone is not stripping. Removing DWARF while the
// linker symbol table survives is the failure mode that a section-name-only
// scan cannot see, and it differs per executable format.
func TestInspectBuiltArtifactRejectsEveryUnderStrippedExecutableFormat(t *testing.T) {
	t.Parallel()

	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v", err)
	}
	version := core.NewReleaseVersion(2026, 0, 11)
	assignments := mustInspectionAssignments(t, "product-stamp-41")
	targets := release.Targets()

	for index := range release.TargetCount {
		platform, ok := targets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false", index)
		}
		for _, mode := range []struct {
			wantErr    error
			name       string
			stripFlags string
		}{
			{name: "no linker stripping retains dwarf and symbols", stripFlags: "", wantErr: core.ErrReleaseContract},
			{name: "dwarf only stripping retains the symbol table", stripFlags: "-w", wantErr: core.ErrReleaseContract},
			{name: "symbol only stripping removes both", stripFlags: "-s"},
			{name: "release stripping removes both", stripFlags: releaseStripFlags},
		} {
			t.Run(platform.String()+"/"+mode.name, func(t *testing.T) {
				t.Parallel()

				build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
					Offering: core.OfferingWitness, Version: version, Commit: commit, Platform: platform,
				})
				if err != nil {
					t.Fatalf("core.NewBuildIdentity() error = %v", err)
				}
				path := buildInspectionFixture(t, build, "product-stamp-41", mode.stripFlags)
				file, err := os.Open(path)
				if err != nil {
					t.Fatalf("os.Open() error = %v", err)
				}
				closeInspectionFile(t, file)
				info, err := file.Stat()
				if err != nil {
					t.Fatalf("file.Stat() error = %v", err)
				}
				_, gotErr := release.InspectBuiltArtifact(release.ArtifactInspectionRequest{
					Source: file, Extent: mustInspectionExtent(t, uint64(info.Size())),
					Build: build, LinkerAssignments: assignments,
				})
				if !errors.Is(gotErr, mode.wantErr) {
					t.Fatalf("release.InspectBuiltArtifact(ldflags %q) error = %v, want %v",
						mode.stripFlags, gotErr, mode.wantErr)
				}
			})
		}
	}
}

func buildInspectionFixture(
	t *testing.T,
	build core.BuildIdentity,
	productValue string,
	stripFlags string,
) string {
	t.Helper()
	context, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := strings.ReplaceAll(build.Platform().String(), "-", "_") +
		"_" + strings.ReplaceAll(strings.TrimSpace(stripFlags), " ", "") + "_strip"
	if build.Platform().OperatingSystem == core.OperatingSystemWindows {
		name += ".exe"
	}
	output := filepath.Join(t.TempDir(), name)
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("exec.LookPath(go) error = %v", err)
	}
	linker := stripFlags
	if linker != "" {
		linker += " "
	}
	linker += "-X " + release.EmbeddedBuildOfferingLinkSymbol + "=" + build.Offering().String() +
		" -X " + release.EmbeddedBuildVersionLinkSymbol + "=" + build.Version().String() +
		" -X " + release.EmbeddedBuildCommitLinkSymbol + "=" + build.Commit().String() +
		" -X " + release.EmbeddedBuildPlatformLinkSymbol + "=" + build.Platform().String() +
		" -X " + inspectionProductSymbol + "=" + productValue
	command := exec.CommandContext(context, goExecutable,
		"build", "-trimpath", "-buildvcs=false", "-pgo=off", "-ldflags="+linker,
		"-o", output, "../testdata/releaseartifact")
	command.Dir = "."
	command.Env = inspectionBuildEnvironment(build.Platform(), goExecutable)
	combined, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go build fixture error = %v, output = %s", err, combined)
	}
	return output
}

func inspectionBuildEnvironment(platform core.Platform, goExecutable string) []string {
	values := []string{
		"CGO_ENABLED=0", "GOARCH=" + platform.Architecture.String(),
		"GOOS=" + platform.OperatingSystem.String(), "GOTOOLCHAIN=local",
		"GOENV=off", "GOFLAGS=", "GOEXPERIMENT=", "GOFIPS140=off", "GOWORK=off",
		"PATH=" + filepath.Dir(goExecutable),
	}
	if platform.Architecture == core.CPUArchitectureAMD64 {
		values = append(values, "GOAMD64=v1")
	} else {
		values = append(values, "GOARM64=v8.0")
	}
	for _, name := range []string{"HOME", "GOCACHE", "GOMODCACHE", "GOPATH"} {
		if value := os.Getenv(name); value != "" {
			values = append(values, name+"="+value)
		}
	}
	return values
}

func digestInspectionFixture(t *testing.T, path string) (core.SHA256Digest, core.CRC32C) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open() error = %v", err)
	}
	closeInspectionFile(t, file)
	sha := sha256.New()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	if _, err := io.CopyBuffer(io.MultiWriter(sha, crc), file, make([]byte, 64<<10)); err != nil {
		t.Fatalf("io.CopyBuffer() error = %v", err)
	}
	var digest [sha256.Size]byte
	copy(digest[:], sha.Sum(nil))
	return core.NewSHA256Digest(digest), core.NewCRC32C(crc.Sum32())
}

func closeInspectionFile(t *testing.T, file *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("(*os.File).Close() error = %v, want nil", err)
		}
	})
}

func mustInspectionExtent(t *testing.T, value uint64) core.ByteCount {
	t.Helper()
	extent, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%s) error = %v", strconv.FormatUint(value, 10), err)
	}
	return extent
}

func mustInspectionAssignments(t *testing.T, value string) release.LinkerAssignments {
	t.Helper()
	assignment, err := release.NewLinkerAssignment(inspectionProductSymbol, value)
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment() error = %v", err)
	}
	assignments, err := release.NewLinkerAssignments([]release.LinkerAssignment{assignment})
	if err != nil {
		t.Fatalf("release.NewLinkerAssignments() error = %v", err)
	}
	return assignments
}

func mustInspectionBuild(
	t *testing.T,
	base core.BuildIdentity,
	architecture core.CPUArchitecture,
) core.BuildIdentity {
	t.Helper()
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: base.Offering(), Version: base.Version(), Commit: base.Commit(),
		Platform: core.Platform{OperatingSystem: base.Platform().OperatingSystem, Architecture: architecture},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v", err)
	}
	return build
}
