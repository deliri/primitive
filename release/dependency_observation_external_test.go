package release_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// observedMainModule is Primitive's own module, which every observed target
	// closure must agree on.
	observedMainModule = "github.com/deliri/primitive/v2026"
	// observedDependencyModule is the one external module Primitive requires. It
	// is reached through hostfacts' platform files, so observing it proves the
	// module parser and union are not vacuous.
	observedDependencyModule = "golang.org/x/sys"
	// observedDependencyPackage is a Primitive package whose closure actually
	// contains an external module. Observing a package with no external modules
	// would leave every module-collection path unexecuted.
	observedDependencyPackage = "github.com/deliri/primitive/v2026/hostfacts"
)

// TestObserveBuildDependenciesReturnsTheRealCrossTargetModuleUnion is the
// production-path proof. It drives the real verified Go executable across all
// four canonical target environments and asserts the observed module facts, not
// just that the call returned without error. The observed package is chosen
// because its closure contains a real external module; observing a closure with
// no external modules would leave module parsing, insertion, ordering, and the
// cross-target union completely unexecuted while still reporting success.
func TestObserveBuildDependenciesReturnsTheRealCrossTargetModuleUnion(t *testing.T) {
	t.Parallel()

	dependencies, err := release.ObserveBuildDependencies(
		t.Context(), dependencyObservationRequestForLiveTest(t, observedDependencyPackage))
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
		if dependencies.Count() == 0 {
			t.Fatalf("observed module count = 0, want the closure of %s to carry %s; "+
				"a zero-module observation proves no module fact was ever parsed",
				observedDependencyPackage, observedDependencyModule)
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
			if !strings.HasPrefix(module.Version().String(), "v") {
				t.Fatalf("%s version = %q, want a cmd/go module version", observedDependencyModule, module.Version().String())
			}
			if !strings.HasPrefix(module.Sum().String(), "h1:") {
				t.Fatalf("%s sum = %q, want an h1 module checksum", observedDependencyModule, module.Sum().String())
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
		if !strings.Contains(string(document), observedDependencyModule) {
			t.Fatalf("dependency document = %s, want it to name %s", document, observedDependencyModule)
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

	valid := dependencyObservationRequestForLiveTest(t, observedDependencyPackage)
	vendorPlan := buildPlanRequestForHostileTest(t)
	vendorPlan.ModuleMode = release.BuildModuleVendor
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

	valid := dependencyObservationRequestForLiveTest(t, observedDependencyPackage)
	cases := []struct {
		wantErr error
		ctx     func() context.Context
		name    string
	}{
		{
			name: "nil context is refused", ctx: func() context.Context { return nil },
			wantErr: core.ErrNilContext,
		},
		{
			name: "pre-cancelled context starts no Go process", ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "expired deadline starts no Go process", ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 0)
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
			dependencies, err := release.ObserveBuildDependencies(tc.ctx(), valid)
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

	request := dependencyObservationRequestForLiveTest(t,
		observedMainModule+"/this-package-does-not-exist")
	dependencies, err := release.ObserveBuildDependencies(t.Context(), request)
	if !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("release.ObserveBuildDependencies() error = %v, want errors.Is(..., %v)",
			err, core.ErrReleaseContract)
	}
	if dependencies != (release.BuildDependencies{}) {
		t.Fatalf("failed observation = %v, want zero facts", dependencies)
	}
}

func dependencyObservationRequestForLiveTest(
	t *testing.T,
	mainPackage string,
) release.BuildDependencyObservationRequest {
	t.Helper()

	parsed, err := release.ParseMainPackage(mainPackage)
	if err != nil {
		t.Fatalf("release.ParseMainPackage(%q) error = %v, want nil", mainPackage, err)
	}
	planRequest := buildPlanRequestForHostileTest(t)
	planRequest.MainPackage = parsed
	plan, err := release.PrepareBuildPlan(planRequest)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	environment, err := process.ParseExactEnvironment(os.Environ())
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment() error = %v, want nil", err)
	}
	verification := buildToolVerificationRequestForLiveTest(t)
	return release.BuildDependencyObservationRequest{
		Stderr:           io.Discard,
		WorkingDirectory: verification.WorkingDirectory,
		HostEnvironment:  environment,
		Tools:            verifiedBuildToolsForLiveTest(t),
		Plan:             plan,
		WaitDelay:        verification.WaitDelay,
	}
}
