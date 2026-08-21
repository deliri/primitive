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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

const (
	inspectionProductSymbol = "github.com/deliri/primitive/v2026/testdata/releasestamp.Value"
	inspectionProductValue  = "product-stamp-41"
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
	assignment, err := release.NewLinkerAssignment(inspectionProductSymbol, inspectionProductValue)
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
				Offering: releaseExternalOffering(t, 2), Version: version, Commit: commit, Platform: platform,
			})
			if err != nil {
				t.Fatalf("core.NewBuildIdentity() error = %v", err)
			}
			directory := inspectionAbsolutePath(t, t.TempDir())
			path := buildInspectionFixture(t, buildInspectionFixtureRequest{
				Directory: directory, Build: build, ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
			})
			artifact, err := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{
				Path: path, Build: build, LinkerAssignments: assignments,
			})
			if err != nil {
				t.Fatalf("release.InspectBuiltArtifact() error = %v", err)
			}
			wantSHA, wantCRC := digestInspectionFixture(t, path)
			info, err := os.Stat(path.String())
			if err != nil {
				t.Fatalf("os.Stat() error = %v, want nil", err)
			}
			wantExtent := mustInspectionExtent(t, uint64(info.Size()))
			if artifact.Build() != build || artifact.Integrity().Extent() != wantExtent ||
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
		Offering: releaseExternalOffering(t, 1), Version: core.NewReleaseVersion(2026, 0, 11),
		Commit: commit, Platform: platform,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v", err)
	}
	directory := inspectionAbsolutePath(t, t.TempDir())
	path := buildInspectionFixture(t, buildInspectionFixtureRequest{
		Directory: directory, Build: build, ProductValue: inspectionProductValue, StripFlags: releaseStripFlags,
	})
	validAssignments := mustInspectionAssignments(t, inspectionProductValue)

	cases := []struct {
		name        string
		assignments release.LinkerAssignments
		build       core.BuildIdentity
	}{
		{name: "missing product stamp", build: build, assignments: mustInspectionAssignments(t, "absent-product-stamp-99")},
		{name: "wrong architecture", build: mustInspectionBuild(t, build, core.CPUArchitectureAMD64), assignments: validAssignments},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{
				Path: path, Build: tc.build, LinkerAssignments: tc.assignments,
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
	assignments := mustInspectionAssignments(t, inspectionProductValue)
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
					Offering: releaseExternalOffering(t, 2), Version: version, Commit: commit, Platform: platform,
				})
				if err != nil {
					t.Fatalf("core.NewBuildIdentity() error = %v", err)
				}
				directory := inspectionAbsolutePath(t, t.TempDir())
				path := buildInspectionFixture(t, buildInspectionFixtureRequest{
					Directory: directory, Build: build, ProductValue: inspectionProductValue, StripFlags: mode.stripFlags,
				})
				_, gotErr := release.InspectBuiltArtifact(t.Context(), release.ArtifactInspectionRequest{
					Path: path, Build: build, LinkerAssignments: assignments,
				})
				if !errors.Is(gotErr, mode.wantErr) {
					t.Fatalf("release.InspectBuiltArtifact(ldflags %q) error = %v, want %v",
						mode.stripFlags, gotErr, mode.wantErr)
				}
			})
		}
	}
}

type buildInspectionFixtureRequest struct {
	Directory    core.AbsolutePath
	ProductValue string
	StripFlags   string
	Build        core.BuildIdentity
}

func buildInspectionFixture(t testing.TB, request buildInspectionFixtureRequest) core.AbsolutePath {
	t.Helper()
	context, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := strings.ReplaceAll(request.Build.Platform().String(), "-", "_") +
		"_" + strings.ReplaceAll(strings.TrimSpace(request.StripFlags), " ", "") + "_strip"
	if request.Build.Platform().OperatingSystem == core.OperatingSystemWindows {
		name += ".exe"
	}
	output := inspectionAbsolutePath(t, filepath.Join(request.Directory.String(), name))
	goExecutable, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("exec.LookPath(go) error = %v", err)
	}
	arguments := inspectionGoBuildArguments(t, request, output)
	command := exec.CommandContext(context, goExecutable, arguments...)
	command.Dir = "."
	command.Env = inspectionBuildEnvironment(request.Build.Platform(), goExecutable)
	combined, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go build fixture error = %v, output = %s", err, combined)
	}
	return output
}

func inspectionGoBuildArguments(
	t testing.TB,
	request buildInspectionFixtureRequest,
	output core.AbsolutePath,
) []string {
	t.Helper()
	mainPackage, err := release.ParseMainPackage("github.com/deliri/primitive/v2026/testdata/releaseartifact")
	if err != nil {
		t.Fatalf("release.ParseMainPackage(inspection fixture) error = %v, want nil", err)
	}
	outputDirectory, err := core.ParseRelativePath("inspection-output")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(inspection output) error = %v, want nil", err)
	}
	plan, err := release.PrepareBuildPlan(release.BuildPlanRequest{
		Offering: request.Build.Offering(), Version: request.Build.Version(), Commit: request.Build.Commit(),
		GoToolchain: release.CurrentGoToolchain(), MainPackage: mainPackage, OutputDirectory: outputDirectory,
		ModuleMode:        release.BuildModuleReadonly,
		LinkerAssignments: mustInspectionAssignments(t, request.ProductValue),
	})
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan(inspection fixture) error = %v, want nil", err)
	}
	command := inspectionBuildCommand(t, plan, request.Build)
	arguments, err := command.ArgumentValues()
	if err != nil {
		t.Fatalf("release.BuildCommand.ArgumentValues(inspection fixture) error = %v, want nil", err)
	}
	return lowerInspectionGoArguments(t, lowerInspectionGoArgumentsRequest{
		Arguments: arguments, Output: output, StripFlags: request.StripFlags,
	})
}

func inspectionBuildCommand(
	t testing.TB,
	plan release.BuildPlan,
	wantBuild core.BuildIdentity,
) release.BuildCommand {
	t.Helper()
	for index := range release.TargetCount {
		command, ok := plan.At(index)
		if ok && command.Build() == wantBuild {
			return command
		}
	}
	t.Fatalf("release.BuildPlan has no command for %v", wantBuild)
	return release.BuildCommand{}
}

type lowerInspectionGoArgumentsRequest struct {
	Output     core.AbsolutePath
	StripFlags string
	Arguments  []string
}

func lowerInspectionGoArguments(t testing.TB, request lowerInspectionGoArgumentsRequest) []string {
	t.Helper()
	buildIndex := slices.Index(request.Arguments, "build")
	if buildIndex < 0 {
		t.Fatalf("release.BuildCommand arguments contain no Go build boundary: %q", request.Arguments)
	}
	goArguments := append([]string{"build"}, request.Arguments[buildIndex+1:]...)
	outputIndex := slices.Index(goArguments, "-o")
	if outputIndex < 0 || outputIndex+1 >= len(goArguments) {
		t.Fatalf("release.BuildCommand arguments contain no complete output projection: %q", goArguments)
	}
	goArguments[outputIndex+1] = request.Output.String()
	linkerIndex := -1
	for index, argument := range goArguments {
		if strings.HasPrefix(argument, "-ldflags=") {
			linkerIndex = index
			break
		}
	}
	if linkerIndex < 0 || !strings.Contains(goArguments[linkerIndex], releaseStripFlags) {
		t.Fatalf("release.BuildCommand arguments contain no production linker stripping: %q", goArguments)
	}
	goArguments[linkerIndex] = strings.Replace(goArguments[linkerIndex], releaseStripFlags, request.StripFlags, 1)
	return goArguments
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

func digestInspectionFixture(t *testing.T, path core.AbsolutePath) (core.SHA256Digest, core.CRC32C) {
	t.Helper()
	file, err := os.Open(path.String())
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

func inspectionAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
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

func mustInspectionAssignments(t testing.TB, value string) release.LinkerAssignments {
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
