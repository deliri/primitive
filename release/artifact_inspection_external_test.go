package release_test

import (
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
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
			wantSHA, wantCRC, length := digestInspectionFixture(t, path)
			wantExtent := mustInspectionExtent(t, length.Uint64())
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
	duration, err := temporal.DurationFromSeconds(120)
	if err != nil {
		t.Fatalf("DurationFromSeconds(build timeout) error = %v, want nil", err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
	if err != nil {
		t.Fatalf("WithTimeout(build fixture) error = %v, want nil", err)
	}
	defer cancel()
	name := strings.ReplaceAll(request.Build.Platform().String(), "-", "_") +
		"_" + strings.ReplaceAll(strings.TrimSpace(request.StripFlags), " ", "") + "_strip"
	if request.Build.Platform().OperatingSystem == core.OperatingSystemWindows {
		name += ".exe"
	}
	output := inspectionAbsolutePath(t, filepath.Join(request.Directory.String(), name))
	nameComponent, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatalf("ParsePathComponent(go) error = %v, want nil", err)
	}
	goExecutable, err := process.Resolve(ctx, nameComponent)
	if err != nil {
		t.Fatalf("process.Resolve(go) error = %v, want nil", err)
	}
	arguments, err := process.ParseArguments(inspectionGoBuildArguments(t, request, output))
	if err != nil {
		t.Fatalf("process.ParseArguments(build fixture) error = %v, want nil", err)
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	environment := inspectionBuildEnvironment(t, request.Build.Platform(), goExecutable)
	wait, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatalf("DurationFromSeconds(process wait) error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(4 << 20)
	if err != nil {
		t.Fatalf("NewByteCount(build output) error = %v, want nil", err)
	}
	runFixtureProcess(ctx, t, process.Request{
		Command: goExecutable, WorkingDirectory: directory, Arguments: arguments,
		Environment: environment, WaitDelay: wait, OutputLimit: maximum,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill},
	})
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

func inspectionBuildEnvironment(t testing.TB, platform core.Platform, goExecutable core.AbsolutePath) process.Environment {
	t.Helper()
	values := []string{
		"CGO_ENABLED=0", "GOARCH=" + platform.Architecture.String(),
		"GOOS=" + platform.OperatingSystem.String(), "GOTOOLCHAIN=local",
		"GOENV=off", "GOFLAGS=", "GOEXPERIMENT=", "GOFIPS140=off", "GOWORK=off",
		"PATH=" + filepath.Dir(goExecutable.String()),
	}
	if platform.Architecture == core.CPUArchitectureAMD64 {
		values = append(values, "GOAMD64=v1")
	} else {
		values = append(values, "GOARM64=v8.0")
	}
	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
	}
	for _, variable := range ambient.Variables {
		name, err := variable.Name.Value()
		if err != nil {
			t.Fatalf("ambient variable Name.Value() error = %v, want nil", err)
		}
		if slices.Contains([]string{"HOME", "GOCACHE", "GOMODCACHE", "GOPATH", "SYSTEMROOT"}, name) {
			value, err := variable.Value.Value()
			if err != nil {
				t.Fatalf("ambient variable Value.Value(%s) error = %v, want nil", name, err)
			}
			values = append(values, name+"="+value)
		}
	}
	environment, err := process.ParseExactEnvironment(values)
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment(build fixture) error = %v, want nil", err)
	}
	return environment
}

func digestInspectionFixture(t testing.TB, path core.AbsolutePath) (core.SHA256Digest, core.CRC32C, core.ByteLength) {
	t.Helper()
	location, err := filestore.OpenParent(t.Context(), path)
	if err != nil {
		t.Fatalf("filestore.OpenParent(digest fixture) error = %v, want nil", err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("digest parent Close() error = %v, want nil", err)
		}
	}()
	maximum, err := core.NewByteCount(release.BuiltArtifactMaximumBytes)
	if err != nil {
		t.Fatalf("NewByteCount(digest ceiling) error = %v, want nil", err)
	}
	sha := sha256.New()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	count, err := filestore.Read(t.Context(), filestore.ReadRequest{
		Location: location, Destination: io.MultiWriter(sha, crc), MaximumBytes: maximum,
	})
	if err != nil {
		t.Fatalf("filestore.Read(digest fixture) error = %v, want nil", err)
	}
	return core.NewSHA256Digest([core.SHA256DigestBytes]byte(sha.Sum(nil))), core.NewCRC32C(crc.Sum32()), count
}

func inspectionAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func mustInspectionExtent(t testing.TB, value uint64) core.ByteCount {
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
