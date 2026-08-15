package release_test

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestPrepareBuildProcessReplacesEveryTargetControlledEnvironmentFact(t *testing.T) {
	t.Parallel()

	fixture, repository := verifiedRepositoryForBuildProcessTest(t)
	planRequest := buildPlanRequestForHostileTest(t)
	planRequest.Commit = fixture.commit
	plan, err := release.PrepareBuildPlan(planRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	command, ok := plan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) ok = false, want Windows/AMD64 command")
	}
	hostEnvironment, err := process.ParseExactEnvironment([]string{
		"PATH=/poison/ambient/bin",
		"HOME=/tmp/release-home",
		"GOCACHE=/executor/cache",
		"GOMODCACHE=/executor/modcache",
		"GOPATH=/executor/gopath",
		"GOOS=poison",
		"GOARCH=poison",
		"CGO_ENABLED=1",
		"GOTOOLCHAIN=auto",
		"GOAMD64=v4",
		"GOFLAGS=-race",
		"GOWORK=/tmp/ambient.work",
	})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment() error = %v, want nil", err)
	}
	tools := verifiedBuildToolsForLiveTest(t)
	goToolDirectory, err := tools.GoExecutable().Parent()
	if err != nil {
		t.Fatalf("verified Go executable parent error = %v, want nil", err)
	}
	workingDirectory := fixture.root
	outputLimit, err := core.NewByteCount(64 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(output limit) error = %v, want nil", err)
	}
	waitDelay, err := temporal.NewDuration(30 * time.Second)
	if err != nil {
		t.Fatalf("temporal.NewDuration(wait delay) error = %v, want nil", err)
	}

	got, err := release.PrepareBuildProcess(release.BuildProcessRequest{
		Command: command, Repository: repository, Tools: tools, WorkingDirectory: workingDirectory,
		HostEnvironment: hostEnvironment,
		Streams:         process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard},
		OutputLimit:     outputLimit, WaitDelay: waitDelay,
	})
	if err != nil {
		t.Fatalf("release.PrepareBuildProcess() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("prepared process.Request.Validate() error = %v, want nil", err)
	}
	if got.Command != tools.GarbleExecutable() || got.WorkingDirectory != workingDirectory {
		t.Fatalf("prepared process paths = (%v, %v), want (%v, %v)", got.Command, got.WorkingDirectory, tools.GarbleExecutable(), workingDirectory)
	}
	wantArguments, err := command.ArgumentValues()
	if err != nil {
		t.Fatalf("release.BuildCommand.ArgumentValues() error = %v, want nil", err)
	}
	gotArguments := make([]string, len(got.Arguments))
	for index, argument := range got.Arguments {
		gotArguments[index], err = argument.Value()
		if err != nil {
			t.Fatalf("process.Argument(%d).Value() error = %v, want nil", index, err)
		}
	}
	if !slices.Equal(gotArguments, wantArguments) {
		t.Fatalf("prepared arguments = %q, want %q", gotArguments, wantArguments)
	}
	environment, err := got.Environment.Strings()
	if err != nil {
		t.Fatalf("prepared Environment.Strings() error = %v, want nil", err)
	}
	gitToolDirectory, err := repository.GitExecutable().Parent()
	if err != nil {
		t.Fatalf("verified Git executable parent error = %v, want nil", err)
	}
	wantPath := goToolDirectory.String()
	if gitToolDirectory != goToolDirectory {
		wantPath = goToolDirectory.String() + string(os.PathListSeparator) + gitToolDirectory.String()
	}
	wantEnvironment := []string{
		"HOME=/tmp/release-home",
		"GOCACHE=/executor/cache",
		"GOMODCACHE=/executor/modcache",
		"GOPATH=/executor/gopath",
		"PATH=" + wantPath,
		"CGO_ENABLED=0",
		"GOARCH=amd64",
		"GOOS=windows",
		"GOTOOLCHAIN=local",
		"GOAMD64=v1",
		"GOENV=off",
		"GOFLAGS=",
		"GOEXPERIMENT=",
		"GOFIPS140=off",
		"GOWORK=off",
	}
	if !slices.Equal(environment, wantEnvironment) {
		t.Fatalf("prepared environment = %q, want %q", environment, wantEnvironment)
	}
}

func TestPrepareBuildProcessRejectsAmbientOrUnusableHostExecutionInputs(t *testing.T) {
	t.Parallel()

	valid := buildProcessRequestForHostileTest(t)
	foreignPlanRequest := buildPlanRequestForHostileTest(t)
	foreignCommit, err := core.ParseBuildCommit(strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(foreign) error = %v, want nil", err)
	}
	foreignPlanRequest.Commit = foreignCommit
	foreignPlan, err := release.PrepareBuildPlan(foreignPlanRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan(foreign) error = %v, want nil", err)
	}
	foreignCommand, ok := foreignPlan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) for foreign commit ok = false, want true")
	}
	foreignWorkingDirectory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(foreign worktree) error = %v, want nil", err)
	}
	cases := []struct {
		wantErr error
		mutate  func(*release.BuildProcessRequest)
		name    string
	}{
		{name: "zero command", mutate: func(r *release.BuildProcessRequest) { r.Command = release.BuildCommand{} }, wantErr: core.ErrReleaseContract},
		{name: "zero verified repository", mutate: func(r *release.BuildProcessRequest) { r.Repository = release.VerifiedRepository{} }, wantErr: core.ErrReleaseContract},
		{name: "command commit differs from verified repository", mutate: func(r *release.BuildProcessRequest) { r.Command = foreignCommand }, wantErr: core.ErrReleaseContract},
		{name: "zero verified tools", mutate: func(r *release.BuildProcessRequest) { r.Tools = release.VerifiedBuildTools{} }, wantErr: core.ErrReleaseContract},
		{name: "zero working directory", mutate: func(r *release.BuildProcessRequest) { r.WorkingDirectory = core.AbsolutePath{} }, wantErr: core.ErrReleaseContract},
		{name: "working directory differs from verified repository", mutate: func(r *release.BuildProcessRequest) {
			r.WorkingDirectory = foreignWorkingDirectory
		}, wantErr: core.ErrReleaseContract},
		{name: "ambient inheritance", mutate: func(r *release.BuildProcessRequest) {
			r.HostEnvironment = process.Environment{Mode: process.EnvironmentModeInherit}
		}, wantErr: core.ErrReleaseContract},
		{name: "nil stdin", mutate: func(r *release.BuildProcessRequest) { r.Streams.Stdin = nil }, wantErr: core.ErrReleaseContract},
		{name: "zero output limit", mutate: func(r *release.BuildProcessRequest) { r.OutputLimit = core.ByteCount{} }, wantErr: core.ErrReleaseContract},
		{name: "zero wait delay", mutate: func(r *release.BuildProcessRequest) { r.WaitDelay = temporal.Duration{} }, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			got, gotErr := release.PrepareBuildProcess(request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.PrepareBuildProcess() error = %v, want %v", gotErr, tc.wantErr)
			}
			if gotErr != nil && !zeroProcessRequest(got) {
				t.Fatalf("release.PrepareBuildProcess() request = %v, want zero on rejection", got)
			}
		})
	}
}

func zeroProcessRequest(request process.Request) bool {
	return request.Streams.Stdin == nil && request.Streams.Stdout == nil && request.Streams.Stderr == nil &&
		request.Command == (core.AbsolutePath{}) && request.WorkingDirectory == (core.AbsolutePath{}) &&
		request.Arguments == nil && request.Environment.Mode == process.EnvironmentModeUnknown &&
		request.Environment.Variables == nil && request.OutputLimit == (core.ByteCount{}) &&
		request.WaitDelay == (temporal.Duration{})
}

func buildProcessRequestForHostileTest(t *testing.T) release.BuildProcessRequest {
	t.Helper()

	fixture, repository := verifiedRepositoryForBuildProcessTest(t)
	planRequest := buildPlanRequestForHostileTest(t)
	planRequest.Commit = fixture.commit
	plan, err := release.PrepareBuildPlan(planRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	command, ok := plan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) ok = false, want true")
	}
	tools := verifiedBuildToolsForLiveTest(t)
	workingDirectory := fixture.root
	environment, err := process.ParseExactEnvironment([]string{"PATH=/opt/pinned-go/bin"})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment() error = %v, want nil", err)
	}
	outputLimit, err := core.NewByteCount(64 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	waitDelay, err := temporal.NewDuration(30 * time.Second)
	if err != nil {
		t.Fatalf("temporal.NewDuration() error = %v, want nil", err)
	}
	return release.BuildProcessRequest{
		Command: command, Repository: repository, Tools: tools, WorkingDirectory: workingDirectory,
		HostEnvironment: environment,
		Streams:         process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard},
		OutputLimit:     outputLimit, WaitDelay: waitDelay,
	}
}

func verifiedRepositoryForBuildProcessTest(t *testing.T) (repositoryFixture, release.VerifiedRepository) {
	t.Helper()
	fixture := newRepositoryFixture(t)
	verified, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
	if err != nil {
		t.Fatalf("release.VerifyRepository(build worktree) error = %v, want nil", err)
	}
	return fixture, verified
}
