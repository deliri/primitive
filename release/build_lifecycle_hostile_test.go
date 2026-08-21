package release_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestDeterministicFourTargetBuildLifecycleLayerTriad closes the product-
// neutral build mechanics from a real clean Git observation through all four
// exact, ambient-free process projections. Existing package tables own the
// broader dirty-worktree, argument, environment, tag, and linker boundaries.
func TestDeterministicFourTargetBuildLifecycleLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive clean exact commit projects four deterministic build processes", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		home := t.TempDir()
		fixture := newRepositoryFixtureAt(t, root, home)
		repository, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
		if err != nil || repository.Validate() != nil {
			t.Fatalf("release.VerifyRepository(clean exact commit) = (%v, %v), want valid proof and nil", repository, err)
		}

		request := buildPlanRequestForHostileTest(t)
		request.Commit = repository.Commit()
		plan, err := release.PrepareBuildPlan(request)
		if err != nil || plan.Validate() != nil {
			t.Fatalf("release.PrepareBuildPlan(clean exact commit) = (%v, %v), want valid plan and nil", plan, err)
		}
		repeated, err := release.PrepareBuildPlan(request)
		if err != nil || repeated != plan {
			t.Fatalf("release.PrepareBuildPlan(repeated) = (%v, %v), want byte-for-byte-equivalent plan %v", repeated, err, plan)
		}

		tools := verifiedBuildToolsForLiveTest(t)
		hostEnvironment, err := process.ParseExactEnvironment([]string{
			"PATH=/ambient/not-reviewed", "HOME=/customer/home", "CGO_ENABLED=1",
			"GOOS=ambient", "GOARCH=ambient", "GOTOOLCHAIN=auto", "GOFLAGS=-race",
		})
		if err != nil {
			t.Fatalf("process.ParseExactEnvironment(hostile ambient controls) error = %v, want nil", err)
		}
		outputLimit, err := core.NewByteCount(64 << 10)
		if err != nil {
			t.Fatalf("core.NewByteCount(output limit) error = %v, want nil", err)
		}
		waitDelay, err := temporal.NewDuration(30 * time.Second)
		if err != nil {
			t.Fatalf("temporal.NewDuration(wait delay) error = %v, want nil", err)
		}
		wantPlatforms := [release.TargetCount]core.Platform{
			{OperatingSystem: core.OperatingSystemWindows, Architecture: core.CPUArchitectureAMD64},
			{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
			{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64},
			{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureARM64},
		}

		for index, wantPlatform := range wantPlatforms {
			command, ok := plan.At(index)
			if !ok || command.Build().Platform() != wantPlatform ||
				command.Build().Commit() != repository.Commit() ||
				command.Build().Offering() != request.Offering ||
				command.Build().Version() != request.Version {
				t.Fatalf("release.BuildPlan.At(%d) = (%v, %t), want exact build for %v", index, command, ok, wantPlatform)
			}
			prepared, err := release.PrepareBuildProcess(release.BuildProcessRequest{
				Streams: process.Streams{
					Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard,
				},
				WorkingDirectory: fixture.root, HostEnvironment: hostEnvironment,
				Repository: repository, Tools: tools, Command: command,
				OutputLimit: outputLimit, WaitDelay: waitDelay,
			})
			if err != nil || prepared.Validate() != nil {
				t.Fatalf("release.PrepareBuildProcess(%v) = (%v, %v), want valid request and nil", wantPlatform, prepared, err)
			}
			if prepared.Command != tools.GoExecutable() || prepared.WorkingDirectory != repository.Root() ||
				prepared.Environment.Mode != process.EnvironmentModeExact {
				t.Fatalf("prepared %v execution facts = (%v, %v, %v), want verified Go, repository root, and exact environment", wantPlatform, prepared.Command, prepared.WorkingDirectory, prepared.Environment.Mode)
			}
			wantArguments, err := command.ArgumentValues()
			if err != nil {
				t.Fatalf("release.BuildCommand(%v).ArgumentValues() error = %v, want nil", wantPlatform, err)
			}
			gotArguments := make([]string, len(prepared.Arguments))
			for argumentIndex, argument := range prepared.Arguments {
				gotArguments[argumentIndex], err = argument.Value()
				if err != nil {
					t.Fatalf("process.Argument(%v, %d).Value() error = %v, want nil", wantPlatform, argumentIndex, err)
				}
			}
			if !slices.Equal(gotArguments, wantArguments) {
				t.Fatalf("prepared %v arguments = %q, want exact BuildCommand arguments %q", wantPlatform, gotArguments, wantArguments)
			}
			gotEnvironment, err := prepared.Environment.Strings()
			if err != nil || slices.Contains(gotEnvironment, "PATH=/ambient/not-reviewed") ||
				slices.Contains(gotEnvironment, "CGO_ENABLED=1") || slices.Contains(gotEnvironment, "GOOS=ambient") ||
				slices.Contains(gotEnvironment, "GOARCH=ambient") || slices.Contains(gotEnvironment, "GOTOOLCHAIN=auto") ||
				slices.Contains(gotEnvironment, "GOFLAGS=-race") {
				t.Fatalf("prepared %v environment = (%q, %v), want exact projection with every ambient build control removed", wantPlatform, gotEnvironment, err)
			}
		}
	})

	t.Run("negative independently valid foreign commit cannot enter the verified worktree", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		home := t.TempDir()
		fixture := newRepositoryFixtureAt(t, root, home)
		repository, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
		if err != nil {
			t.Fatalf("release.VerifyRepository(clean exact commit) error = %v, want nil", err)
		}
		foreignCommit, err := core.ParseBuildCommit(strings.Repeat("f", 40))
		if err != nil || foreignCommit == repository.Commit() {
			t.Fatalf("core.ParseBuildCommit(foreign) = (%v, %v), want valid commit distinct from %v", foreignCommit, err, repository.Commit())
		}
		request := buildPlanRequestForHostileTest(t)
		request.Commit = foreignCommit
		plan, err := release.PrepareBuildPlan(request)
		if err != nil {
			t.Fatalf("release.PrepareBuildPlan(foreign valid commit) error = %v, want nil", err)
		}
		command, ok := plan.At(0)
		if !ok {
			t.Fatal("release.BuildPlan.At(0) ok = false, want independently valid foreign command")
		}
		hostEnvironment, err := process.ParseExactEnvironment([]string{})
		if err != nil {
			t.Fatalf("process.ParseExactEnvironment(empty exact host) error = %v, want nil", err)
		}
		outputLimit, err := core.NewByteCount(64 << 10)
		if err != nil {
			t.Fatalf("core.NewByteCount(output limit) error = %v, want nil", err)
		}
		waitDelay, err := temporal.NewDuration(30 * time.Second)
		if err != nil {
			t.Fatalf("temporal.NewDuration(wait delay) error = %v, want nil", err)
		}
		got, gotErr := release.PrepareBuildProcess(release.BuildProcessRequest{
			Streams: process.Streams{
				Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard,
			},
			WorkingDirectory: fixture.root, HostEnvironment: hostEnvironment,
			Repository: repository, Tools: verifiedBuildToolsForLiveTest(t), Command: command,
			OutputLimit: outputLimit, WaitDelay: waitDelay,
		})
		if !errors.Is(gotErr, core.ErrReleaseContract) || !zeroProcessRequest(got) {
			t.Fatalf("release.PrepareBuildProcess(foreign commit) = (%v, %v), want exact zero and %v", got, gotErr, core.ErrReleaseContract)
		}
	})

	t.Run("neutral zero planning and execution inputs acquire no build authority", func(t *testing.T) {
		t.Parallel()

		plan, planErr := release.PrepareBuildPlan(release.BuildPlanRequest{})
		if !errors.Is(planErr, core.ErrReleaseContract) || plan != (release.BuildPlan{}) {
			t.Fatalf("release.PrepareBuildPlan(zero) = (%v, %v), want exact zero and %v", plan, planErr, core.ErrReleaseContract)
		}
		prepared, processErr := release.PrepareBuildProcess(release.BuildProcessRequest{})
		if !errors.Is(processErr, core.ErrReleaseContract) || !zeroProcessRequest(prepared) {
			t.Fatalf("release.PrepareBuildProcess(zero) = (%v, %v), want exact zero and %v", prepared, processErr, core.ErrReleaseContract)
		}
	})
}
