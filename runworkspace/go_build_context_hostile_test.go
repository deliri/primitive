package runworkspace

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
)

func TestGoBuildContextDiscoveryHostileBoundaries(t *testing.T) {
	t.Parallel()

	type contextMutation func(testing.TB, *runnercontrol.GoBuildContext)
	cases := []struct {
		wantErr error
		mutate  contextMutation
		name    string
		file    string
		header  string
		want    bool
	}{
		{name: "valid unconstrained test file is visible", file: "subject_test.go", want: true},
		{name: "valid linux constraint matches admitted linux", file: "subject_test.go", header: "//go:build linux\n", want: true},
		{name: "valid amd64 constraint matches admitted amd64", file: "subject_test.go", header: "//go:build amd64\n", want: true},
		{name: "valid disabled cgo satisfies explicit negation", file: "subject_test.go", header: "//go:build !cgo\n", want: true},
		{name: "valid exact release tag matches compiler-owned release set", file: "subject_test.go", header: "//go:build go1.27\n", want: true},
		{name: "valid admitted product build tag is visible", file: "subject_test.go", header: "//go:build integration\n", mutate: addContextBuildTag("integration"), want: true},
		{name: "valid operating-system disjunction admits one true arm", file: "subject_test.go", header: "//go:build linux || darwin\n", want: true},
		{name: "valid linux filename suffix matches admitted operating system", file: "subject_linux_test.go", want: true},
		{name: "valid amd64 filename suffix matches admitted architecture", file: "subject_amd64_test.go", want: true},
		{name: "valid combined filename suffix matches exact platform", file: "subject_linux_amd64_test.go", want: true},

		{name: "reject darwin constraint on admitted linux", file: "subject_test.go", header: "//go:build darwin\n"},
		{name: "reject windows constraint on admitted linux", file: "subject_test.go", header: "//go:build windows\n"},
		{name: "reject arm64 constraint on admitted amd64", file: "subject_test.go", header: "//go:build arm64\n"},
		{name: "reject cgo constraint when cgo is disabled", file: "subject_test.go", header: "//go:build cgo\n"},
		{name: "reject unadmitted product build tag", file: "subject_test.go", header: "//go:build integration\n"},
		{name: "reject negated admitted operating system", file: "subject_test.go", header: "//go:build !linux\n"},
		{name: "reject future release absent from admitted release set", file: "subject_test.go", header: "//go:build go1.28\n"},
		{name: "reject race-only source under ordinary instrumentation", file: "subject_test.go", header: "//go:build race\n"},
		{name: "reject darwin filename suffix on admitted linux", file: "subject_darwin_test.go"},
		{name: "reject malformed build expression", file: "subject_test.go", header: "//go:build linux &&\n", wantErr: core.ErrPrimitiveContract},

		{name: "boundary race instrumentation supplies race tag", file: "subject_test.go", header: "//go:build race\n", mutate: setContextInstrumentation(runnercontrol.GoInstrumentationRace), want: true},
		{name: "boundary diagnostic instrumentation does not invent race tag", file: "subject_test.go", header: "//go:build race\n", mutate: setContextInstrumentation(runnercontrol.GoInstrumentationDiagnostic)},
		{name: "boundary enabled cgo supplies cgo tag", file: "subject_test.go", header: "//go:build cgo\n", mutate: enableContextCGO, want: true},
		{name: "boundary enabled cgo no longer satisfies cgo negation", file: "subject_test.go", header: "//go:build !cgo\n", mutate: enableContextCGO},
		{name: "boundary admitted experiment state supplies goexperiment tool tag", file: "subject_test.go", header: "//go:build goexperiment.boringcrypto\n", mutate: addContextExperiment("boringcrypto"), want: true},
		{name: "boundary absent experiment state refuses goexperiment tool tag", file: "subject_test.go", header: "//go:build goexperiment.boringcrypto\n"},
		{name: "boundary conjunction is visible when every admitted fact matches", file: "subject_test.go", header: "//go:build linux && amd64\n", want: true},
		{name: "boundary conjunction is hidden when one admitted fact differs", file: "subject_test.go", header: "//go:build linux && arm64\n"},
		{name: "boundary negated absent product tag is visible", file: "subject_test.go", header: "//go:build !enterprise\n", want: true},
		{name: "boundary parenthesized expression preserves precedence", file: "subject_test.go", header: "//go:build linux && (amd64 || arm64)\n", want: true},
		{name: "boundary legacy plus-build line remains compiler visible", file: "subject_test.go", header: "// +build linux\n\n", want: true},
		{name: "boundary equivalent modern and legacy constraints agree", file: "subject_test.go", header: "//go:build linux\n// +build linux\n\n", want: true},
		{name: "boundary modern constraint remains authoritative over stale legacy line", file: "subject_test.go", header: "//go:build linux\n// +build darwin\n\n", want: true},
		{name: "boundary hidden source filename is never build input", file: ".subject_test.go"},
		{name: "boundary underscore-prefixed source filename is never build input", file: "_subject_test.go"},
		{name: "boundary ordinary test suffix has no platform restriction", file: "subject_test.go", want: true},
		{name: "boundary linux suffix before test suffix remains visible", file: "subject_linux_test.go", want: true},
		{name: "boundary amd64 suffix before test suffix remains visible", file: "subject_amd64_test.go", want: true},
		{name: "boundary mismatched combined platform filename is hidden", file: "subject_linux_arm64_test.go"},
		{name: "boundary Unicode build tag is admitted and matched exactly", file: "subject_test.go", header: "//go:build café\n", mutate: addContextBuildTag("café"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			context := goBuildContextFixture(t)
			if tc.mutate != nil {
				tc.mutate(t, &context)
			}
			file, err := standard.ParseSourcePath(tc.file)
			if err != nil {
				t.Fatalf("standard.ParseSourcePath(%q) setup error = %v, want nil", tc.file, err)
			}
			source := []byte(tc.header + "package subject\n")
			got, gotErr := matchGoFileContext(source, file, context)
			if tc.wantErr != nil {
				if got || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("matchGoFileContext(%s) = (%t, %v), want false and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("matchGoFileContext(%s) = (%t, %v), want (%t, nil)", tc.name, got, gotErr, tc.want)
			}
		})
	}
}

func goBuildContextFixture(t testing.TB) runnercontrol.GoBuildContext {
	t.Helper()
	toolchain, toolchainErr := standard.NewIdentifier("go1-27-0")
	release, releaseErr := runnercontrol.NewGoBuildTag("go1.27")
	module, moduleErr := standard.ParseSourcePath("module")
	if err := errors.Join(toolchainErr, releaseErr, moduleErr); err != nil {
		t.Fatalf("GoBuildContext fixture identity error = %v, want nil", err)
	}
	context := runnercontrol.GoBuildContext{
		Toolchain: toolchain, ReleaseTags: []runnercontrol.GoBuildTag{release}, GOOS: core.OperatingSystemLinux,
		GOARCH: core.CPUArchitectureAMD64, BuildTags: []runnercontrol.GoBuildTag{}, Instrumentation: runnercontrol.GoInstrumentationOrdinary,
		GOExperiment: []runnercontrol.GoBuildTag{}, ModuleMode: runnercontrol.GoModuleModeModule, ModuleRoot: module,
		OtherInputs: core.SHA256Of([]byte("go-build-context-hostile-test")),
	}
	if err := context.Validate(); err != nil {
		t.Fatalf("GoBuildContext.Validate() setup error = %v, want nil", err)
	}
	return context
}

func addContextBuildTag(value string) func(testing.TB, *runnercontrol.GoBuildContext) {
	return func(t testing.TB, context *runnercontrol.GoBuildContext) {
		t.Helper()
		tag, err := runnercontrol.NewGoBuildTag(value)
		if err != nil {
			t.Fatalf("runnercontrol.NewGoBuildTag(%q) setup error = %v, want nil", value, err)
		}
		context.BuildTags = []runnercontrol.GoBuildTag{tag}
	}
}

func addContextExperiment(value string) func(testing.TB, *runnercontrol.GoBuildContext) {
	return func(t testing.TB, context *runnercontrol.GoBuildContext) {
		t.Helper()
		tag, err := runnercontrol.NewGoBuildTag(value)
		if err != nil {
			t.Fatalf("runnercontrol.NewGoBuildTag(%q) setup error = %v, want nil", value, err)
		}
		context.GOExperiment = []runnercontrol.GoBuildTag{tag}
	}
}

func setContextInstrumentation(value runnercontrol.GoInstrumentation) func(testing.TB, *runnercontrol.GoBuildContext) {
	return func(_ testing.TB, context *runnercontrol.GoBuildContext) { context.Instrumentation = value }
}

func enableContextCGO(_ testing.TB, context *runnercontrol.GoBuildContext) { context.CGOEnabled = true }
