package release_test

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// The isolated module imports x/sys/cpu on every target. Its pinned facts
	// come from cmd/go's cached module metadata, never a copied version literal.
	observedMainModule        = "github.com/deliri/primitive/v2026"
	observedDependencyModule  = "golang.org/x/sys"
	observedDependencyPackage = observedMainModule
)

// This projection consumes cmd/go's public module output; it carries no
// independently invented release protocol or live resource ownership.
type dependencyFixtureModule struct {
	Path     string
	Version  string
	Sum      string
	GoModSum string
}

type dependencyLiveFixture struct {
	request release.BuildDependencyObservationRequest
	module  dependencyFixtureModule
}

// TestObserveBuildDependenciesReturnsTheRealCrossTargetModuleUnion is the
// production-path proof. It drives the real verified Go executable across all
// four canonical target environments and asserts the observed module facts, not
// just that the call returned without error. The observed package is chosen
// because its closure contains a real external module; observing a closure with
// no external modules would leave module parsing, insertion, ordering, and the
// cross-target union completely unexecuted while still reporting success.
func TestObserveBuildDependenciesReturnsTheRealCrossTargetModuleUnion(t *testing.T) {
	t.Parallel()

	fixture := dependencyObservationFixture(t, t.TempDir(), t.TempDir(), observedDependencyPackage)
	dependencies, err := release.ObserveBuildDependencies(t.Context(), fixture.request)
	if err != nil {
		t.Fatalf("release.ObserveBuildDependencies() error = %v, want nil", err)
	}
	t.Run("observed closure retains exact module facts", func(t *testing.T) {
		t.Parallel()

		if err := dependencies.Validate(); err != nil {
			t.Fatalf("release.BuildDependencies.Validate() error = %v, want nil", err)
		}
		if got := dependencies.MainModule().String(); got != observedMainModule {
			t.Fatalf("main module = %q, want %q", got, observedMainModule)
		}
		if got := dependencies.GoToolchain(); got != release.CurrentGoToolchain() {
			t.Fatalf("Go toolchain = %v, want %v", got, release.CurrentGoToolchain())
		}
		if dependencies.Count() != 1 {
			t.Fatalf("observed module count = %d, want exactly one %s module", dependencies.Count(), observedDependencyModule)
		}
		found := false
		for index := range dependencies.Count() {
			module, ok := dependencies.At(index)
			if !ok {
				t.Fatalf("release.BuildDependencies.At(%d) = (_, false), want a module below Count()", index)
			}
			if err := module.Validate(); err != nil {
				t.Fatalf("observed module %d Validate() error = %v, want nil", index, err)
			}
			if index > 0 {
				previous, _ := dependencies.At(index - 1)
				if previous.Path().String() >= module.Path().String() {
					t.Fatalf("module paths at (%d, %d) = (%q, %q), want ascending order",
						index-1, index, previous.Path().String(), module.Path().String())
				}
			}
			if module.Path().String() != observedDependencyModule {
				continue
			}
			found = true
			if module.Version().String() != fixture.module.Version {
				t.Fatalf("%s version = %q, want %q", observedDependencyModule, module.Version().String(), fixture.module.Version)
			}
			if module.Sum().String() != fixture.module.Sum {
				t.Fatalf("%s sum = %q, want %q", observedDependencyModule, module.Sum().String(), fixture.module.Sum)
			}
		}
		if !found {
			t.Fatalf("observed modules omit %s, want the real closure of %s",
				observedDependencyModule, observedDependencyPackage)
		}
		if _, ok := dependencies.At(dependencies.Count()); ok {
			t.Fatalf("release.BuildDependencies.At(%d) = (_, true), want (_, false)", dependencies.Count())
		}
	})

	t.Run("observed closure projects into one metadata document", func(t *testing.T) {
		t.Parallel()

		document, err := dependencies.MarshalJSON()
		if err != nil {
			t.Fatalf("release.BuildDependencies.MarshalJSON() error = %v, want nil", err)
		}
		var decoded release.BuildDependencies
		if err := decoded.UnmarshalJSON(document); err != nil {
			t.Fatalf("release.BuildDependencies.UnmarshalJSON() error = %v, want nil", err)
		}
		second, err := decoded.MarshalJSON()
		if err != nil {
			t.Fatalf("re-encoded dependency document error = %v, want nil", err)
		}
		if string(second) != string(document) {
			t.Fatalf("re-encoded dependency document = %s, want %s", second, document)
		}
		if decoded.Count() != dependencies.Count() ||
			decoded.MainModule() != dependencies.MainModule() ||
			decoded.GoToolchain() != dependencies.GoToolchain() {
			t.Fatalf("decoded document = (%q, %v, %d modules), want (%q, %v, %d)",
				decoded.MainModule().String(), decoded.GoToolchain(), decoded.Count(),
				dependencies.MainModule().String(), dependencies.GoToolchain(), dependencies.Count())
		}
		extent, err := core.NewByteCount(uint64(len(document)))
		if err != nil {
			t.Fatalf("core.NewByteCount() error = %v, want nil", err)
		}
		asset, err := release.InspectMetadataAsset(release.MetadataInspectionRequest{
			Source: strings.NewReader(string(document)),
			Extent: extent,
			Kind:   release.MetadataKindDependencies,
		})
		if err != nil {
			t.Fatalf("release.InspectMetadataAsset() error = %v, want nil", err)
		}
		if err := asset.Validate(); err != nil {
			t.Fatalf("release.MetadataAsset.Validate() error = %v, want nil", err)
		}
		if asset.Kind() != release.MetadataKindDependencies {
			t.Fatalf("metadata asset kind = %v, want %v", asset.Kind(), release.MetadataKindDependencies)
		}
	})
}

// TestObserveBuildDependenciesRejectsEveryIncompleteRequest pressures each
// boundary input independently. The zero-request case alone cannot tell a
// missing stderr from a missing tool set, so a regression that drops one field
// from Validate would still pass a single zero-value case.
func TestObserveBuildDependenciesRejectsEveryIncompleteRequest(t *testing.T) {
	t.Parallel()

	fixture := dependencyObservationFixture(t, t.TempDir(), t.TempDir(), observedDependencyPackage)
	valid := fixture.request
	vendorPlan := buildPlanRequestForHostileTest(t)
	vendorPlan.ModuleMode = release.BuildModuleVendor
	vendorPlan.Commit = valid.Repository.Commit()
	vendored, err := release.PrepareBuildPlan(vendorPlan)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan(vendor) error = %v, want nil", err)
	}
	cases := []struct {
		mutate func(*release.BuildDependencyObservationRequest)
		name   string
	}{
		{name: "zero request is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			*r = release.BuildDependencyObservationRequest{}
		}},
		{name: "nil stderr is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.Stderr = nil
		}},
		{name: "zero working directory is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.WorkingDirectory = core.AbsolutePath{}
		}},
		{name: "zero host environment is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.HostEnvironment = process.Environment{}
		}},
		{name: "unverified build tools are rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.Tools = release.VerifiedBuildTools{}
		}},
		{name: "unverified repository is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.Repository = release.VerifiedRepository{}
		}},
		{name: "unset build plan is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.Plan = release.BuildPlan{}
		}},
		{name: "zero wait delay is rejected", mutate: func(r *release.BuildDependencyObservationRequest) {
			r.WaitDelay = temporal.Duration{}
		}},
		{
			// cmd/go reports no module checksum for a vendored closure, so a
			// vendored observation could only publish version-only facts as if
			// they were checksum pinned. It must fail before any Go process runs.
			name: "vendored module mode is rejected before any Go process starts",
			mutate: func(r *release.BuildDependencyObservationRequest) {
				r.Plan = vendored
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			dependencies, err := release.ObserveBuildDependencies(t.Context(), request)
			if !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("release.ObserveBuildDependencies() error = %v, want errors.Is(..., %v)",
					err, core.ErrReleaseContract)
			}
			if dependencies != (release.BuildDependencies{}) {
				t.Fatalf("rejected observation = %v, want zero facts", dependencies)
			}
			if err := request.Validate(); !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("release.BuildDependencyObservationRequest.Validate() error = %v, want errors.Is(..., %v)",
					err, core.ErrReleaseContract)
			}
		})
	}
}

// TestObserveBuildDependenciesRefusesUnusableContexts proves the neutral half of
// the context contract: a nil or already-cancelled context must be refused
// before four Go processes are spawned.
func TestObserveBuildDependenciesRefusesUnusableContexts(t *testing.T) {
	t.Parallel()

	fixture := dependencyObservationFixture(t, t.TempDir(), t.TempDir(), observedDependencyPackage)
	valid := fixture.request
	cases := []struct {
		wantErr error
		ctx     func(*testing.T) context.Context
		name    string
	}{
		{
			name: "nil context is refused", ctx: func(*testing.T) context.Context { return nil },
			wantErr: core.ErrNilContext,
		},
		{
			name: "pre-cancelled context starts no Go process", ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "expired deadline starts no Go process", ctx: func(t *testing.T) context.Context {
				ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: temporal.Duration{}})
				if err != nil {
					t.Fatalf("temporal.WithTimeout(expired fixture) error = %v, want nil", err)
				}
				cancel()
				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			//nolint:staticcheck // The nil-context contract is the behavior under test.
			dependencies, err := release.ObserveBuildDependencies(tc.ctx(t), valid)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("release.ObserveBuildDependencies() error = %v, want errors.Is(..., %v)", err, tc.wantErr)
			}
			if dependencies != (release.BuildDependencies{}) {
				t.Fatalf("refused observation = %v, want zero facts", dependencies)
			}
		})
	}
}

// TestObserveBuildDependenciesFailsLoudlyOnAnAbsentPackage proves the process
// layer of the triad: when cmd/go exits nonzero the observation must fail rather
// than publish an empty or partial closure.
func TestObserveBuildDependenciesFailsLoudlyOnAnAbsentPackage(t *testing.T) {
	t.Parallel()

	fixture := dependencyObservationFixture(t, t.TempDir(), t.TempDir(),
		observedMainModule+"/this-package-does-not-exist")
	request := fixture.request
	dependencies, err := release.ObserveBuildDependencies(t.Context(), request)
	if !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("release.ObserveBuildDependencies() error = %v, want errors.Is(..., %v)",
			err, core.ErrReleaseContract)
	}
	if dependencies != (release.BuildDependencies{}) {
		t.Fatalf("failed observation = %v, want zero facts", dependencies)
	}
}

func dependencyObservationFixture(t *testing.T, root, home, mainPackage string) dependencyLiveFixture {
	t.Helper()
	tools := verifiedBuildToolsForLiveTest(t)
	working, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
	}
	values, err := ambient.Strings()
	if err != nil {
		t.Fatalf("Environment.Strings() error = %v, want nil", err)
	}
	environment, err := process.ParseEffectiveEnvironment(append(values,
		"GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOENV=off", "GOFLAGS=",
	))
	if err != nil {
		t.Fatalf("process.ParseEffectiveEnvironment() error = %v, want nil", err)
	}
	arguments, err := process.ParseArguments([]string{"list", "-m", "-json", observedDependencyModule})
	if err != nil {
		t.Fatalf("process.ParseArguments(module) error = %v, want nil", err)
	}
	wait, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(1 << 20)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	output := runFixtureProcess(t.Context(), t, process.Request{
		Command: tools.GoExecutable(), WorkingDirectory: working, Arguments: arguments,
		Environment: environment, WaitDelay: wait, OutputLimit: limit,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill},
	})
	var module dependencyFixtureModule
	if err := json.Unmarshal([]byte(output), &module); err != nil {
		t.Fatalf("decode Go module fixture error = %v, want nil", err)
	}
	if module.Path != observedDependencyModule || module.Version == "" || module.Sum == "" || module.GoModSum == "" {
		t.Fatalf("cached Go module facts = %+v, want exact dependency with version and both checksums", module)
	}
	repository := newRepositoryFixtureAt(t, root, home)
	version, err := tools.GoToolchain().Version()
	if err != nil {
		t.Fatalf("GoToolchain.Version() error = %v, want nil", err)
	}
	for _, file := range []repositoryFileWrite{
		{root: repository.root, name: "go.mod", body: "module " + observedMainModule + "\n\ngo " + strings.TrimPrefix(version, "go") + "\n\nrequire " + module.Path + " " + module.Version + "\n"},
		{root: repository.root, name: "go.sum", body: module.Path + " " + module.Version + " " + module.Sum + "\n" + module.Path + " " + module.Version + "/go.mod " + module.GoModSum + "\n"},
		{root: repository.root, name: "facts.go", body: "package facts\nimport \"golang.org/x/sys/cpu\"\nvar _ cpu.CacheLinePad\n"},
	} {
		writeRepositoryFileForTest(t, file)
	}
	runRepositoryGitForTest(t, repository, "add", "--", "go.mod", "go.sum", "facts.go")
	runRepositoryGitForTest(t, repository, "commit", "--quiet", "-m", "dependency fixture")
	repository.commit = repositoryHeadForTest(t, repository)
	verified, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, repository))
	if err != nil {
		t.Fatalf("release.VerifyRepository(module) error = %v, want nil", err)
	}
	parsed, err := release.ParseMainPackage(mainPackage)
	if err != nil {
		t.Fatalf("release.ParseMainPackage() error = %v, want nil", err)
	}
	planRequest := buildPlanRequestForHostileTest(t)
	planRequest.MainPackage, planRequest.Commit = parsed, verified.Commit()
	plan, err := release.PrepareBuildPlan(planRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	return dependencyLiveFixture{module: module, request: release.BuildDependencyObservationRequest{
		Stderr: io.Discard, WorkingDirectory: verified.Root(), HostEnvironment: environment,
		Repository: verified, Tools: tools, Plan: plan, WaitDelay: wait,
	}}
}

// Repository and commit equality are independent, binary ownership relations.
// All four combinations are exhausted; malformed fields have a separate table.
func TestDependencyObservationBindsVerifiedRootAndCommitBeforeEffects(t *testing.T) {
	t.Parallel()
	fixture := dependencyObservationFixture(t, t.TempDir(), t.TempDir(), observedDependencyPackage)
	foreignRoot := absolutePathForTest(t, t.TempDir())
	planRequest := buildPlanRequestForHostileTest(t)
	main, err := release.ParseMainPackage(observedDependencyPackage)
	if err != nil {
		t.Fatalf("release.ParseMainPackage(fixture) error = %v, want nil", err)
	}
	planRequest.MainPackage = main
	if planRequest.Commit == fixture.request.Repository.Commit() {
		t.Fatalf("foreign commit = %v, want distinct from fixture commit %v", planRequest.Commit, fixture.request.Repository.Commit())
	}
	foreignPlan, err := release.PrepareBuildPlan(planRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan(foreign) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name          string
		foreignRoot   bool
		foreignCommit bool
		wantErr       error
	}{
		{name: "same verified root and commit admit observation"},
		{name: "foreign root with verified commit refuses substitution", foreignRoot: true, wantErr: core.ErrReleaseContract},
		{name: "verified root with foreign commit refuses false attribution", foreignCommit: true, wantErr: core.ErrReleaseContract},
		{name: "foreign root and commit cannot borrow verification", foreignRoot: true, foreignCommit: true, wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := fixture.request
			if tc.foreignRoot {
				request.WorkingDirectory = foreignRoot
			}
			if tc.foreignCommit {
				request.Plan = foreignPlan
			}
			if got := request.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("request.Validate() error = %v, want %v", got, tc.wantErr)
			}
			if tc.wantErr == nil {
				return
			}
			got, err := release.ObserveBuildDependencies(t.Context(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ObserveBuildDependencies() error = %v, want %v", err, tc.wantErr)
			}
			if got != (release.BuildDependencies{}) {
				t.Fatalf("refused dependencies = %v, want zero", got)
			}
		})
	}
}
