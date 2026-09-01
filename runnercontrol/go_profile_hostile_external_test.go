package runnercontrol_test

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

type goPlanBoundaryCase struct {
	name     string
	class    string
	mutate   func(*runnercontrol.GoPlanRequest)
	wantFlag string
	wantErr  error
}

func TestCompileGoPlanCompletesHostileEvidenceFloor(t *testing.T) {
	t.Parallel()

	cases := goPlanSupplementalCases()
	gotClasses := make(map[string]int)
	for index := range cases {
		gotClasses[cases[index].class]++
	}
	wantClasses := map[string]int{"valid": 4, "rejection": 3, "boundary": 20}
	if len(cases) != 27 || gotClasses["valid"] != 4 || gotClasses["rejection"] != 3 || gotClasses["boundary"] != 20 {
		t.Fatalf("supplemental Go plan matrix classes = %v across %d cases, want %v across 27 cases completing the profile table's 10/10/20 floor", gotClasses, len(cases), wantClasses)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := baseAcceptanceGoPlanRequest(t)
			tc.mutate(&request)
			got, gotErr := runnercontrol.CompileGoPlan(request)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got.Process.SchemaVersion != 0 {
					t.Fatalf("CompileGoPlan(%s) = (process %+v, error %v), want zero and errors.Is(..., %v)", tc.name, got.Process, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("CompileGoPlan(%s) error = %v, want nil", tc.name, gotErr)
			}
			if tc.wantFlag != "" && !slices.Contains(planArguments(t, got.Process), tc.wantFlag) {
				t.Fatalf("CompileGoPlan(%s) arguments = %q, want load-bearing flag %q", tc.name, planArguments(t, got.Process), tc.wantFlag)
			}
		})
	}
}

func TestCompileGoPlanReportsRequestedAndEffectiveConcurrency(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		logicalCPUCount uint16
		wantEffective   uint16
		wantCPU         []uint16
	}{
		{name: "one core machine reports request one thousand acted one", logicalCPUCount: 1, wantEffective: 1, wantCPU: []uint16{1}},
		{name: "one hundred core machine reports request one thousand acted one hundred", logicalCPUCount: 100, wantEffective: 100, wantCPU: []uint16{1, 2, 4, 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := baseAcceptanceGoPlanRequest(t)
			request.Machine.LogicalCPUCount = tc.logicalCPUCount
			request.Environment.GOMAXPROCS = 1_000
			request.Experiment.Parallel = 1_000
			request.Experiment.PackageParallel = 1_000
			request.Experiment.CPU = []uint16{1, 2, 4, 1_000}

			got, gotErr := runnercontrol.CompileGoPlan(request)
			if gotErr != nil || got.Go == nil {
				t.Fatalf("CompileGoPlan(requested 1000 on %d CPUs) = (Go %v, error %v), want typed concurrency evidence and nil", tc.logicalCPUCount, got.Go, gotErr)
			}
			requested := got.Go.Requested
			effective := got.Go.Effective
			if requested.GOMAXPROCS != 1_000 || requested.Parallel != 1_000 || requested.PackageParallel != 1_000 || !slices.Equal(requested.CPU, []uint16{1, 2, 4, 1_000}) {
				t.Fatalf("CompileGoPlan() requested concurrency = %+v, want exact request 1000 and CPU [1 2 4 1000]", requested)
			}
			if effective.GOMAXPROCS != tc.wantEffective || effective.Parallel != tc.wantEffective || effective.PackageParallel != tc.wantEffective || !slices.Equal(effective.CPU, tc.wantCPU) {
				t.Fatalf("CompileGoPlan() effective concurrency = %+v, want CPU ceiling %d and CPU %v", effective, tc.wantEffective, tc.wantCPU)
			}
			arguments := planArguments(t, got.Process)
			wantParallel := "-parallel=" + strconv.FormatUint(uint64(tc.wantEffective), 10)
			wantPackage := "-p=" + strconv.FormatUint(uint64(tc.wantEffective), 10)
			if !slices.Contains(arguments, wantParallel) || !slices.Contains(arguments, wantPackage) {
				t.Fatalf("CompileGoPlan() arguments = %q, want acted-on %q and %q", arguments, wantParallel, wantPackage)
			}
			environment, environmentErr := got.Process.Environment.Strings()
			wantGOMAXPROCS := "GOMAXPROCS=" + strconv.FormatUint(uint64(tc.wantEffective), 10)
			if environmentErr != nil || !slices.Contains(environment, wantGOMAXPROCS) {
				t.Fatalf("CompileGoPlan() environment = (%q, %v), want acted-on %q", environment, environmentErr, wantGOMAXPROCS)
			}
		})
	}

	t.Run("tampered effective concurrency cannot validate as execution evidence", func(t *testing.T) {
		t.Parallel()
		request := baseAcceptanceGoPlanRequest(t)
		got, gotErr := runnercontrol.CompileGoPlan(request)
		if gotErr != nil || got.Go == nil {
			t.Fatalf("CompileGoPlan(tamper baseline) = (Go %v, error %v), want typed concurrency evidence and nil", got.Go, gotErr)
		}
		got.Go.Effective.Parallel++
		if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("ExperimentExecution.Validate(tampered effective parallelism) error = %v, want errors.Is(..., %v)", gotErr, core.ErrPrimitiveContract)
		}
	})
}

func baseAcceptanceGoPlanRequest(t testing.TB) runnercontrol.GoPlanRequest {
	t.Helper()
	zero := mustProfileDuration(t, 0)
	plan := runnercontrol.GoExperimentPlan{
		Profile: runnercontrol.GoProfileAcceptance, Kind: projectstandards.ProbeKindGoTest, Package: mustProfileSourcePath(t, "internal/subject"),
		Timeout: mustProfileDuration(t, 60_000_000_000), ShuffleSeed: 17, Parallel: 4, PackageParallel: 2, RepeatCount: runnercontrol.ExecutionRepeatCount,
		CPU: []uint16{1, 2, 4}, Tags: []projectstandards.Identifier{}, BenchmarkDuration: zero, FuzzDuration: zero, FuzzMinimizeDuration: zero,
	}
	return goPlanRequestFixture(t, plan)
}

func goPlanSupplementalCases() []goPlanBoundaryCase {
	return []goPlanBoundaryCase{
		{name: "valid anchored selector quotes regular expression metacharacters", class: "valid", mutate: func(r *runnercontrol.GoPlanRequest) {
			selector, _ := projectstandards.NewName("TestA+B")
			r.Experiment.Selector = &selector
		}, wantFlag: "-run=^TestA\\+B$"},
		{name: "valid set coverage retains its compiler-owned mode", class: "valid", mutate: addCoverage(runnercontrol.CoverageSet), wantFlag: "-covermode=set"},
		{name: "valid count coverage retains its compiler-owned mode", class: "valid", mutate: addCoverage(runnercontrol.CoverageCount), wantFlag: "-covermode=count"},
		{name: "valid diagnostic profile retains all five distinct artifact contracts", class: "valid", mutate: configureAllDiagnostics, wantFlag: "-trace=/workspace/artifacts/trace.out"},
		{name: "rejection repeat count cannot amplify a Go evidence phase", class: "rejection", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.RepeatCount = 2 }, wantErr: core.ErrPrimitiveContract},
		{name: "rejection coverage mode without a durable path is incomplete", class: "rejection", mutate: func(r *runnercontrol.GoPlanRequest) {
			mode := runnercontrol.CoverageAtomic
			r.Experiment.Coverage = &mode
		}, wantErr: core.ErrPrimitiveContract},
		{name: "rejection durable coverage path without a mode is incomplete", class: "rejection", mutate: func(r *runnercontrol.GoPlanRequest) {
			path, _ := core.ParseRelativePath("coverage.out")
			r.Experiment.CoveragePath = &path
		}, wantErr: core.ErrPrimitiveContract},
		{name: "boundary zero timeout is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.Timeout, _ = temporal.DurationFromNanoseconds(0) }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary minimum positive timeout is retained", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.Timeout, _ = temporal.DurationFromNanoseconds(1) }, wantFlag: "-timeout=1ns"},
		{name: "boundary zero expected units is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.ExpectedUnits = 0 }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary one expected unit closes one wave", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.ExpectedUnits = 1; r.Experiment.PackageParallel = 1 }, wantFlag: "-p=1"},
		{name: "boundary maximum requested package width is capped by the observed machine", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			r.ExpectedUnits = 400
			r.Experiment.PackageParallel = math.MaxUint16
		}, wantFlag: "-p=4"},
		{name: "boundary one above expected-unit ceiling is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			r.ExpectedUnits = runnercontrol.ExecutionAccountingUnitMaximum + 1
		}, wantErr: core.ErrPrimitiveContract},
		{name: "boundary package parallelism zero is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.PackageParallel = 0 }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary maximum package parallelism is capped by observed CPUs", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.PackageParallel = math.MaxUint16 }, wantFlag: "-p=4"},
		{name: "boundary test parallelism one is emitted exactly", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.Parallel = 1 }, wantFlag: "-parallel=1"},
		{name: "boundary maximum test parallelism is capped by observed CPUs", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.Parallel = math.MaxUint16 }, wantFlag: "-parallel=4"},
		{name: "boundary single CPU value remains non-vacuous", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.CPU = []uint16{1} }, wantFlag: "-cpu=1"},
		{name: "boundary maximum CPU value is capped by observed CPUs", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.CPU = []uint16{math.MaxUint16} }, wantFlag: "-cpu=4"},
		{name: "boundary zero CPU entry is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.CPU = []uint16{0} }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary duplicate CPU entry is refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.CPU = []uint16{1, 1} }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary descending CPU entries are refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.Experiment.CPU = []uint16{2, 1} }, wantErr: core.ErrPrimitiveContract},
		{name: "boundary one canonical build tag is emitted exactly", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			tag, _ := projectstandards.NewIdentifier("integration")
			r.Experiment.Tags = []projectstandards.Identifier{tag}
		}, wantFlag: "-tags=integration"},
		{name: "boundary two canonical build tags preserve declared order", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			first, _ := projectstandards.NewIdentifier("alpha")
			second, _ := projectstandards.NewIdentifier("omega")
			r.Experiment.Tags = []projectstandards.Identifier{first, second}
		}, wantFlag: "-tags=alpha,omega"},
		{name: "boundary duplicate build tags are refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			tag, _ := projectstandards.NewIdentifier("same")
			r.Experiment.Tags = []projectstandards.Identifier{tag, tag}
		}, wantErr: core.ErrPrimitiveContract},
		{name: "boundary descending build tags are refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) {
			first, _ := projectstandards.NewIdentifier("zeta")
			second, _ := projectstandards.NewIdentifier("alpha")
			r.Experiment.Tags = []projectstandards.Identifier{first, second}
		}, wantErr: core.ErrPrimitiveContract},
		{name: "boundary one hundred one effective waves are refused", class: "boundary", mutate: func(r *runnercontrol.GoPlanRequest) { r.ExpectedUnits = 202; r.Experiment.PackageParallel = 2 }, wantErr: core.ErrPrimitiveContract},
	}
}

func addCoverage(mode runnercontrol.CoverageMode) func(*runnercontrol.GoPlanRequest) {
	return func(request *runnercontrol.GoPlanRequest) {
		path, _ := core.ParseRelativePath("coverage.out")
		request.Experiment.Coverage = &mode
		request.Experiment.CoveragePath = &path
	}
}

func configureAllDiagnostics(request *runnercontrol.GoPlanRequest) {
	request.Experiment.Profile = runnercontrol.GoProfileDiagnostic
	request.Experiment.Kind = projectstandards.ProbeKindGoDiagnosticProfile
	paths := make([]core.RelativePath, 5)
	for index, value := range []string{"cpu.pprof", "memory.pprof", "block.pprof", "mutex.pprof", "trace.out"} {
		paths[index], _ = core.ParseRelativePath(value)
	}
	request.Experiment.Diagnostics = &runnercontrol.DiagnosticArtifacts{CPU: &paths[0], Memory: &paths[1], Block: &paths[2], Mutex: &paths[3], Trace: &paths[4]}
}

func TestCompileGoPlanExhaustsProfileVariantsAndRefusalEdges(t *testing.T) {
	t.Parallel()

	selector := mustProfileName(t, "TestBoundary")
	coverage := runnercontrol.CoverageAtomic
	coveragePath := mustProfileRelativePath(t, "coverage.out")
	cpuProfile := mustProfileRelativePath(t, "cpu.pprof")
	memoryProfile := mustProfileRelativePath(t, "memory.pprof")
	benchmarkDuration := mustProfileDuration(t, 30_000_000_000)
	fuzzDuration := mustProfileDuration(t, 30_000_000_000)
	exploratoryDuration := mustProfileDuration(t, 10_000_000_000)
	minimizeDuration := mustProfileDuration(t, 5_000_000_000)
	zeroDuration := mustProfileDuration(t, 0)
	benchmarkDiagnostics := &runnercontrol.DiagnosticArtifacts{CPU: &cpuProfile, Memory: &memoryProfile}

	cases := []struct {
		name            string
		profile         runnercontrol.GoProfileKind
		kind            projectstandards.ProbeKind
		selector        *projectstandards.Name
		parallel        uint16
		packageParallel uint16
		coverage        *runnercontrol.CoverageMode
		coveragePath    *core.RelativePath
		benchmark       temporal.Duration
		fuzz            temporal.Duration
		minimize        temporal.Duration
		diagnostics     *runnercontrol.DiagnosticArtifacts
		disableCGO      bool
		wantFlags       []string
		wantErr         error
	}{
		{name: "focused profile pins one exact test selector", profile: runnercontrol.GoProfileFocused, kind: projectstandards.ProbeKindGoTest, selector: &selector, parallel: 4, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, wantFlags: []string{"-run=^TestBoundary$"}},
		{name: "acceptance profile runs one uncached package sweep", profile: runnercontrol.GoProfileAcceptance, kind: projectstandards.ProbeKindGoTest, parallel: 4, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration},
		{name: "race profile preserves atomic coverage and race instrumentation", profile: runnercontrol.GoProfileRace, kind: projectstandards.ProbeKindGoRace, parallel: 2, coverage: &coverage, coveragePath: &coveragePath, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, wantFlags: []string{"-race", "-covermode=atomic", "-coverprofile=/workspace/artifacts/coverage.out"}},
		{name: "benchmark profile is serial for thirty seconds with CPU and memory proof", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 1, packageParallel: 1, benchmark: benchmarkDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: benchmarkDiagnostics, wantFlags: []string{"-run=^$", "-bench=^TestBoundary$", "-benchmem", "-benchtime=30s", "-cpuprofile=/workspace/artifacts/cpu.pprof", "-memprofile=/workspace/artifacts/memory.pprof"}},
		{name: "diagnostic profile projects the requested CPU evidence path", profile: runnercontrol.GoProfileDiagnostic, kind: projectstandards.ProbeKindGoDiagnosticProfile, parallel: 1, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: &runnercontrol.DiagnosticArtifacts{CPU: &cpuProfile}, wantFlags: []string{"-cpuprofile=/workspace/artifacts/cpu.pprof"}},
		{name: "fuzz profile is serial for the accepted thirty seconds", profile: runnercontrol.GoProfileFuzz, kind: projectstandards.ProbeKindGoFuzz, selector: &selector, parallel: 1, packageParallel: 1, benchmark: zeroDuration, fuzz: fuzzDuration, minimize: minimizeDuration, wantFlags: []string{"-run=^$", "-fuzz=^TestBoundary$", "-fuzztime=30s", "-fuzzminimizetime=5s"}},
		{name: "focused profile refuses a missing selector", profile: runnercontrol.GoProfileFocused, kind: projectstandards.ProbeKindGoTest, parallel: 4, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, wantErr: core.ErrPrimitiveContract},
		{name: "race profile refuses ordinary test kind", profile: runnercontrol.GoProfileRace, kind: projectstandards.ProbeKindGoTest, parallel: 2, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, wantErr: core.ErrPrimitiveContract},
		{name: "race profile refuses a machine context without CGO", profile: runnercontrol.GoProfileRace, kind: projectstandards.ProbeKindGoRace, parallel: 2, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, disableCGO: true, wantErr: core.ErrPrimitiveContract},
		{name: "benchmark profile refuses parallel execution", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 2, packageParallel: 1, benchmark: benchmarkDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: benchmarkDiagnostics, wantErr: core.ErrPrimitiveContract},
		{name: "benchmark profile refuses parallel package execution", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 1, packageParallel: 2, benchmark: benchmarkDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: benchmarkDiagnostics, wantErr: core.ErrPrimitiveContract},
		{name: "benchmark profile refuses exploratory duration as acceptance evidence", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 1, packageParallel: 1, benchmark: exploratoryDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: benchmarkDiagnostics, wantErr: core.ErrPrimitiveContract},
		{name: "benchmark profile refuses missing CPU and memory evidence", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 1, packageParallel: 1, benchmark: benchmarkDuration, fuzz: zeroDuration, minimize: zeroDuration, wantErr: core.ErrPrimitiveContract},
		{name: "benchmark profile refuses CPU-only evidence", profile: runnercontrol.GoProfileBenchmark, kind: projectstandards.ProbeKindGoBenchmark, selector: &selector, parallel: 1, packageParallel: 1, benchmark: benchmarkDuration, fuzz: zeroDuration, minimize: zeroDuration, diagnostics: &runnercontrol.DiagnosticArtifacts{CPU: &cpuProfile}, wantErr: core.ErrPrimitiveContract},
		{name: "fuzz profile refuses a zero search budget", profile: runnercontrol.GoProfileFuzz, kind: projectstandards.ProbeKindGoFuzz, selector: &selector, parallel: 1, packageParallel: 1, benchmark: zeroDuration, fuzz: zeroDuration, minimize: minimizeDuration, wantErr: core.ErrPrimitiveContract},
		{name: "fuzz profile refuses an exploratory ten second search budget", profile: runnercontrol.GoProfileFuzz, kind: projectstandards.ProbeKindGoFuzz, selector: &selector, parallel: 1, packageParallel: 1, benchmark: zeroDuration, fuzz: exploratoryDuration, minimize: minimizeDuration, wantErr: core.ErrPrimitiveContract},
		{name: "fuzz profile refuses parallel package execution", profile: runnercontrol.GoProfileFuzz, kind: projectstandards.ProbeKindGoFuzz, selector: &selector, parallel: 1, packageParallel: 2, benchmark: zeroDuration, fuzz: fuzzDuration, minimize: minimizeDuration, wantErr: core.ErrPrimitiveContract},
		{name: "diagnostic profile refuses an absent artifact contract", profile: runnercontrol.GoProfileDiagnostic, kind: projectstandards.ProbeKindGoDiagnosticProfile, parallel: 1, benchmark: zeroDuration, fuzz: zeroDuration, minimize: zeroDuration, wantErr: core.ErrPrimitiveContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			packageParallel := tc.packageParallel
			if packageParallel == 0 {
				packageParallel = 2
			}
			request := goPlanRequestFixture(t, runnercontrol.GoExperimentPlan{
				Profile: tc.profile, Kind: tc.kind, Package: mustProfileSourcePath(t, "internal/subject"), Selector: tc.selector,
				Timeout: mustProfileDuration(t, 60_000_000_000), ShuffleSeed: 8675309, Parallel: tc.parallel, PackageParallel: packageParallel, RepeatCount: runnercontrol.ExecutionRepeatCount, CPU: []uint16{1, 2, 4}, Tags: []projectstandards.Identifier{},
				Coverage: tc.coverage, CoveragePath: tc.coveragePath, BenchmarkDuration: tc.benchmark, FuzzDuration: tc.fuzz, FuzzMinimizeDuration: tc.minimize, Diagnostics: tc.diagnostics,
			})
			if tc.disableCGO {
				request.Environment.CGOEnabled = false
			}
			got, gotErr := runnercontrol.CompileGoPlan(request)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got.Process.SchemaVersion != 0 || len(got.Artifacts) != 0 {
					t.Fatalf("CompileGoPlan(%s) = (%+v, %v), want zero and errors.Is(..., %v)", tc.profile.String(), got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("CompileGoPlan(%s) error = %v, want nil", tc.profile.String(), gotErr)
			}
			arguments := planArguments(t, got.Process)
			for _, want := range tc.wantFlags {
				if !slices.Contains(arguments, want) {
					t.Fatalf("CompileGoPlan(%s) arguments = %q, want flag %q", tc.profile.String(), arguments, want)
				}
			}
			wantPackageFlag := "-p=2"
			if tc.profile == runnercontrol.GoProfileBenchmark || tc.profile == runnercontrol.GoProfileFuzz {
				wantPackageFlag = "-p=1"
			}
			if !slices.Contains(arguments, "-count=1") || !slices.Contains(arguments, wantPackageFlag) || !slices.Contains(arguments, "-json") || !slices.Contains(arguments, "-fullpath") || !slices.Contains(arguments, "-outputdir=/workspace/artifacts") {
				t.Fatalf("CompileGoPlan(%s) arguments = %q, want uncached JSON full-path output-directory evidence flags", tc.profile.String(), arguments)
			}
			environment, environmentErr := got.Process.Environment.Strings()
			wantCGO := "CGO_ENABLED=0"
			if tc.profile == runnercontrol.GoProfileRace {
				wantCGO = "CGO_ENABLED=1"
			}
			wantGOMAXPROCS := "GOMAXPROCS=4"
			if tc.profile == runnercontrol.GoProfileBenchmark || tc.profile == runnercontrol.GoProfileFuzz {
				wantGOMAXPROCS = "GOMAXPROCS=1"
			}
			wantEnvironment := []string{"HOME=/workspace/home", "GOCACHE=/workspace/cache", "TMPDIR=/workspace/tmp", "XDG_CACHE_HOME=/workspace/cache", wantGOMAXPROCS, wantCGO}
			if environmentErr != nil || !slices.Equal(environment, wantEnvironment) {
				t.Fatalf("CompileGoPlan(%s) environment = (%q, %v), want (%q, nil)", tc.profile.String(), environment, environmentErr, wantEnvironment)
			}
		})
	}
}

func TestCompileSubjectProcessLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive deny-all subject compiles cgroup namespace filesystem and identity enforcement", func(t *testing.T) {
		t.Parallel()
		capability := experimentObservationRequestFixture(t).Capability
		got, gotErr := runnercontrol.CompileSubjectProcess(capability)
		if gotErr != nil {
			t.Fatalf("CompileSubjectProcess(deny-all capability) error = %v, want nil", gotErr)
		}
		arguments := planArguments(t, got)
		want := []string{
			"--unit=primitive-run-" + capability.Experiment.String(),
			"--property=CPUQuota=100%", "--property=MemoryMax=1024", "--property=TasksMax=2", "--property=LimitNOFILE=2",
			"--property=MemoryAccounting=yes", "--property=CPUAccounting=yes", "--property=IOAccounting=yes", "--property=IPAccounting=yes",
			"--property=RuntimeMaxSec=1ns", "--property=TimeoutStopSec=1ns", "--property=KillMode=control-group", "--property=SendSIGKILL=yes",
			"--property=PrivateNetwork=yes", "--property=ProtectSystem=strict", "--property=ReadOnlyPaths=/tmp",
			"--property=ReadWritePaths=/workspace", "--property=InaccessiblePaths=/run/primitive-control.sock",
			"--property=InaccessiblePaths=/var/lib/primitive/credentials", "--property=InaccessiblePaths=/var/lib/primitive/signing",
			"--property=InaccessiblePaths=/var/lib/primitive/runtime", "--", "/bin/sh",
		}
		for _, required := range want {
			if !slices.Contains(arguments, required) {
				t.Fatalf("CompileSubjectProcess() arguments = %q, want load-bearing isolation argument %q", arguments, required)
			}
		}
	})

	t.Run("negative pinned egress is refused without a Primitive-prepared network namespace", func(t *testing.T) {
		t.Parallel()
		capability := experimentObservationRequestFixture(t).Capability
		service, serviceErr := projectstandards.NewIdentifier("subject-api")
		endpoint, endpointErr := core.ParseHTTPEndpoint("https://127.0.0.1:8443")
		if err := errors.Join(serviceErr, endpointErr); err != nil {
			t.Fatalf("pinned egress fixture error = %v, want nil", err)
		}
		capability.Resources.Egress = runnercontrol.EgressPolicy{
			Mode: runnercontrol.EgressPinned, DNSPolicy: core.SHA256Of([]byte("pinned-dns")),
			Rules: []runnercontrol.EgressRule{{Service: service, Endpoint: endpoint, Protocol: runnercontrol.NetworkTCP, Port: 8443, Certificate: core.SHA256Of([]byte("subject-api-certificate"))}},
		}
		egressDigest, digestErr := capability.Resources.Egress.Digest()
		if digestErr != nil {
			t.Fatalf("EgressPolicy.Digest(pinned fixture) error = %v, want nil", digestErr)
		}
		capability.Execution.Subject.EgressPolicyIdentity = egressDigest
		got, gotErr := runnercontrol.CompileSubjectProcess(capability)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.SchemaVersion != 0 {
			t.Fatalf("CompileSubjectProcess(pinned egress without namespace) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("positive pinned egress enters only the Primitive-prepared namespace bound by policy digest", func(t *testing.T) {
		t.Parallel()
		capability := experimentObservationRequestFixture(t).Capability
		service, serviceErr := projectstandards.NewIdentifier("subject-api")
		endpoint, endpointErr := core.ParseHTTPEndpoint("https://127.0.0.1:8443")
		if err := errors.Join(serviceErr, endpointErr); err != nil {
			t.Fatalf("pinned namespace fixture error = %v, want nil", err)
		}
		capability.Resources.Egress = runnercontrol.EgressPolicy{
			Mode: runnercontrol.EgressPinned, DNSPolicy: core.SHA256Of([]byte("pinned-dns")),
			Rules: []runnercontrol.EgressRule{{Service: service, Endpoint: endpoint, Protocol: runnercontrol.NetworkTCP, Port: 8443, Certificate: core.SHA256Of([]byte("subject-api-certificate"))}},
		}
		egressDigest, digestErr := capability.Resources.Egress.Digest()
		if digestErr != nil {
			t.Fatalf("EgressPolicy.Digest(pinned namespace) error = %v, want nil", digestErr)
		}
		namespace := mustProfileAbsolutePath(t, "/run/netns/primitive-pinned")
		controller := mustProfileAbsolutePath(t, "/usr/libexec/primitive-network")
		capability.Execution.Subject.EgressPolicyIdentity = egressDigest
		capability.Execution.Subject.NetworkNamespace = &namespace
		capability.Execution.Subject.NetworkController = &controller
		got, gotErr := runnercontrol.CompileSubjectProcess(capability)
		arguments := planArguments(t, got)
		if gotErr != nil || !slices.Contains(arguments, "--property=PrivateNetwork=no") || !slices.Contains(arguments, "--property=NetworkNamespacePath=/run/netns/primitive-pinned") || slices.Contains(arguments, "--property=PrivateNetwork=yes") {
			t.Fatalf("CompileSubjectProcess(pinned namespace) = (arguments %q, error %v), want exact prepared namespace and no deny-all namespace", arguments, gotErr)
		}
	})

	t.Run("neutral empty target environment remains empty outside the isolated unit", func(t *testing.T) {
		t.Parallel()
		capability := experimentObservationRequestFixture(t).Capability
		got, gotErr := runnercontrol.CompileSubjectProcess(capability)
		environment, environmentErr := got.Environment.Strings()
		if gotErr != nil || environmentErr != nil || len(environment) != 0 {
			t.Fatalf("CompileSubjectProcess() outer environment = (%q, %v, compile %v), want empty exact environment and nil errors", environment, environmentErr, gotErr)
		}
		for _, argument := range planArguments(t, got) {
			if argument == "--property=IPAddressAllow=" {
				t.Fatalf("CompileSubjectProcess(deny-all) argument = %q, want no fabricated allow rule", argument)
			}
		}
	})
}

func goPlanRequestFixture(t testing.TB, experiment runnercontrol.GoExperimentPlan) runnercontrol.GoPlanRequest {
	t.Helper()
	sourceRoot := mustProfileAbsolutePath(t, "/source")
	uuid := capabilityUUID(t, 41)
	observation, observationErr := projectstandards.NewMachineObservationID(uuid)
	generation, generationErr := projectstandards.NewMachineGenerationID(uuid)
	if err := errors.Join(observationErr, generationErr); err != nil {
		t.Fatalf("machine execution settings fixture error = %v, want nil", err)
	}
	return runnercontrol.GoPlanRequest{
		Command: mustProfileAbsolutePath(t, "/usr/local/go/bin/go"), WorkingDirectory: sourceRoot, WorkspaceRoot: mustProfileAbsolutePath(t, "/workspace"), ArtifactDirectory: mustProfileRelativePath(t, "artifacts"),
		Environment: runnercontrol.GoExecutionEnvironment{Home: mustProfileAbsolutePath(t, "/workspace/home"), Cache: mustProfileAbsolutePath(t, "/workspace/cache"), Temporary: mustProfileAbsolutePath(t, "/workspace/tmp"), GOMAXPROCS: 4, CGOEnabled: experiment.Profile == runnercontrol.GoProfileRace},
		Machine:     projectstandards.MachineExecutionSettings{Observation: observation, Generation: generation, LogicalCPUCount: 4},
		Experiment:  experiment, OutputLimit: mustProfileByteCount(t, 8<<20), ArtifactLimit: mustProfileByteCount(t, 64<<20), ExpectedUnits: 1, WaitDelay: mustProfileDuration(t, 5_000_000_000),
		Subject: subjectExecutionFixture(t, sourceRoot),
	}
}

func subjectExecutionFixture(t testing.TB, sourceRoot core.AbsolutePath) runnercontrol.SubjectExecution {
	t.Helper()
	user, userErr := projectstandards.NewIdentifier("isolated-subject")
	if userErr != nil {
		t.Fatalf("projectstandards.NewIdentifier(subject user) error = %v, want nil", userErr)
	}
	egress := runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, DNSPolicy: core.SHA256Of([]byte("deny-all-dns"))}
	egressIdentity, egressErr := egress.Digest()
	if egressErr != nil {
		t.Fatalf("EgressPolicy.Digest(subject fixture) error = %v, want nil", egressErr)
	}
	return runnercontrol.SubjectExecution{
		Engine: runnercontrol.SubjectIsolationSystemd, Supervisor: mustProfileAbsolutePath(t, "/usr/bin/systemd-run"), Controller: mustProfileAbsolutePath(t, "/usr/bin/systemctl"),
		PolicyIdentity: core.SHA256Of([]byte("subject-isolation-policy")), ProcessUser: user, SourceRoot: sourceRoot,
		EgressPolicyIdentity: egressIdentity,
		ControlSocket:        mustProfileAbsolutePath(t, "/run/primitive-control.sock"), HostCredentials: mustProfileAbsolutePath(t, "/var/lib/primitive/credentials"),
		SigningState: mustProfileAbsolutePath(t, "/var/lib/primitive/signing"), ExecutableState: mustProfileAbsolutePath(t, "/var/lib/primitive/runtime"),
	}
}

func planArguments(t testing.TB, plan process.Plan) []string {
	t.Helper()
	got := make([]string, len(plan.Arguments))
	for index := range plan.Arguments {
		value, err := plan.Arguments[index].Value()
		if err != nil {
			t.Fatalf("process.Argument.Value(%d) error = %v, want nil", index, err)
		}
		got[index] = value
	}
	return got
}

func mustProfileName(t testing.TB, value string) projectstandards.Name {
	t.Helper()
	got, err := projectstandards.NewName(value)
	if err != nil {
		t.Fatalf("projectstandards.NewName(%q) profile fixture error = %v, want nil", value, err)
	}
	return got
}

func mustProfileSourcePath(t testing.TB, value string) projectstandards.SourcePath {
	t.Helper()
	got, err := projectstandards.ParseSourcePath(value)
	if err != nil {
		t.Fatalf("projectstandards.ParseSourcePath(%q) profile fixture error = %v, want nil", value, err)
	}
	return got
}

func mustProfileAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	got, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) profile fixture error = %v, want nil", value, err)
	}
	return got
}

func mustProfileRelativePath(t testing.TB, value string) core.RelativePath {
	t.Helper()
	got, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) profile fixture error = %v, want nil", value, err)
	}
	return got
}

func mustProfileByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) profile fixture error = %v, want nil", value, err)
	}
	return got
}

func mustProfileDuration(t testing.TB, value int64) temporal.Duration {
	t.Helper()
	got, err := temporal.DurationFromNanoseconds(value)
	if err != nil {
		t.Fatalf("temporal.DurationFromNanoseconds(%d) profile fixture error = %v, want nil", value, err)
	}
	return got
}
