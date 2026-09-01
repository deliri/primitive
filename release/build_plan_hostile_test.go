package release_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

func TestPrepareBuildPlanProjectsExactDocumentedGoArguments(t *testing.T) {
	t.Parallel()

	serverKey, err := release.NewLinkerAssignment(
		"github.com/example/product/internal/release.embeddedServerKey",
		"0123456789abcdef",
	)
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment(server key) error = %v, want nil", err)
	}
	releaseKey, err := release.NewLinkerAssignment(
		"github.com/example/product/internal/release.embeddedReleaseKey",
		"fedcba9876543210",
	)
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment(release key) error = %v, want nil", err)
	}
	assignments, err := release.NewLinkerAssignments([]release.LinkerAssignment{serverKey, releaseKey})
	if err != nil {
		t.Fatalf("release.NewLinkerAssignments() error = %v, want nil", err)
	}
	mainPackage, err := release.ParseMainPackage("github.com/example/product/cmd/product")
	if err != nil {
		t.Fatalf("release.ParseMainPackage() error = %v, want nil", err)
	}
	output, err := core.ParseRelativePath("dist/releases")
	if err != nil {
		t.Fatalf("core.ParseRelativePath() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	version := core.NewReleaseVersion(2026, 0, 11)
	offering := releaseExternalOffering(t, 1)

	plan, err := release.PrepareBuildPlan(release.BuildPlanRequest{
		Offering:          offering,
		Version:           version,
		Commit:            commit,
		GoToolchain:       release.CurrentGoToolchain(),
		MainPackage:       mainPackage,
		OutputDirectory:   output,
		ModuleMode:        release.BuildModuleVendor,
		LinkerAssignments: assignments,
	})
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("release.BuildPlan.Validate() error = %v, want nil", err)
	}

	wantTargets := release.Targets()
	for index := range release.TargetCount {
		command, ok := plan.At(index)
		if !ok {
			t.Fatalf("release.BuildPlan.At(%d) ok = false, want true", index)
		}
		wantTarget, ok := wantTargets.At(index)
		if !ok {
			t.Fatalf("release.Targets().At(%d) ok = false, want true", index)
		}
		if got := command.Build().Platform(); got != wantTarget {
			t.Fatalf("release.BuildCommand(%d).Build().Platform() = %v, want %v", index, got, wantTarget)
		}
		arguments, err := command.ArgumentValues()
		if err != nil {
			t.Fatalf("release.BuildCommand(%d).ArgumentValues() error = %v, want nil", index, err)
		}
		wantOutput := "dist/releases/" + offering.String() + "-2026.0.11-" + wantTarget.String()
		if wantTarget.OperatingSystem == core.OperatingSystemWindows {
			wantOutput += ".exe"
		}
		const linkerProjection = "compiler-owned-linker-projection"
		wantArguments := []string{
			"build",
			"-trimpath",
			"-buildvcs=false",
			"-pgo=off",
			"-mod=vendor",
			linkerProjection,
			"-o",
			wantOutput,
			"github.com/example/product/cmd/product",
		}
		linkerIndex := slices.Index(wantArguments, linkerProjection)
		if linkerIndex < 0 || len(arguments) != len(wantArguments) {
			t.Fatalf("release.BuildCommand(%d) argument extent/index = (%d, %d), want (%d, present)",
				index, len(arguments), linkerIndex, len(wantArguments))
		}
		gotLinker := arguments[linkerIndex]
		if !strings.HasPrefix(gotLinker, "-ldflags=-w -s") {
			t.Fatalf("release.BuildCommand(%d) linker projection = %q, want production stripping first", index, gotLinker)
		}
		for _, symbol := range []string{
			release.EmbeddedBuildOfferingLinkSymbol,
			release.EmbeddedBuildVersionLinkSymbol,
			release.EmbeddedBuildCommitLinkSymbol,
			release.EmbeddedBuildPlatformLinkSymbol,
			"github.com/example/product/internal/release.embeddedReleaseKey",
			"github.com/example/product/internal/release.embeddedServerKey",
		} {
			if strings.Count(gotLinker, symbol+"=") != 1 {
				t.Fatalf("release.BuildCommand(%d) linker projection contains %q %d times, want exactly once", index, symbol, strings.Count(gotLinker, symbol+"="))
			}
		}
		gotArguments := make([]string, len(arguments))
		copy(gotArguments, arguments)
		gotArguments[linkerIndex] = linkerProjection
		if !slices.Equal(gotArguments, wantArguments) {
			t.Fatalf("release.BuildCommand(%d).ArgumentValues() = %q, want %q", index, gotArguments, wantArguments)
		}
		if slices.Contains(arguments, "-tiny") {
			t.Fatalf("release.BuildCommand(%d) arguments contain -tiny, want preserved runtime diagnostics", index)
		}
		wantEnvironment := []string{
			"CGO_ENABLED=0",
			"GOARCH=" + wantTarget.Architecture.String(),
			"GOOS=" + wantTarget.OperatingSystem.String(),
			"GOTOOLCHAIN=local",
		}
		if wantTarget.Architecture == core.CPUArchitectureAMD64 {
			wantEnvironment = append(wantEnvironment, "GOAMD64=v1")
		} else {
			wantEnvironment = append(wantEnvironment, "GOARM64=v8.0")
		}
		wantEnvironment = append(wantEnvironment,
			"GOENV=off",
			"GOFLAGS=",
			"GOEXPERIMENT=",
			"GOFIPS140=off",
			"GOWORK=off",
		)
		environment, err := command.EnvironmentOverrides()
		if err != nil {
			t.Fatalf("release.BuildCommand(%d).EnvironmentOverrides() error = %v, want nil", index, err)
		}
		if !slices.Equal(environment, wantEnvironment) {
			t.Fatalf("release.BuildCommand(%d).EnvironmentOverrides() = %q, want %q", index, environment, wantEnvironment)
		}
	}
	if _, ok := plan.At(-1); ok {
		t.Fatalf("release.BuildPlan.At(-1) ok = true, want false")
	}
	if _, ok := plan.At(release.TargetCount); ok {
		t.Fatalf("release.BuildPlan.At(TargetCount) ok = true, want false")
	}
}

func TestBuildPlanRejectsIncompleteAndConflictingCompilerOwnedInputs(t *testing.T) {
	t.Parallel()

	valid := buildPlanRequestForHostileTest(t)
	cases := []struct {
		wantErr error
		mutate  func(*release.BuildPlanRequest)
		name    string
	}{
		{name: "zero offering is rejected", mutate: func(r *release.BuildPlanRequest) { r.Offering = core.Offering{} }, wantErr: core.ErrReleaseContract},
		{name: "noncanonical offering is rejected", mutate: func(r *release.BuildPlanRequest) { r.Offering = core.Offering{Token: "INVALID"} }, wantErr: core.ErrReleaseContract},
		{name: "zero version is rejected", mutate: func(r *release.BuildPlanRequest) { r.Version = core.ReleaseVersion{} }, wantErr: core.ErrReleaseContract},
		{name: "zero commit is rejected", mutate: func(r *release.BuildPlanRequest) { r.Commit = core.BuildCommit{} }, wantErr: core.ErrReleaseContract},
		{name: "zero main package is rejected", mutate: func(r *release.BuildPlanRequest) { r.MainPackage = release.MainPackage{} }, wantErr: core.ErrReleaseContract},
		{name: "zero output directory is rejected", mutate: func(r *release.BuildPlanRequest) { r.OutputDirectory = core.RelativePath{} }, wantErr: core.ErrReleaseContract},
		{name: "zero Go toolchain is rejected", mutate: func(r *release.BuildPlanRequest) { r.GoToolchain = release.GoToolchainUnknown }, wantErr: core.ErrReleaseContract},
		{name: "future Go toolchain is rejected", mutate: func(r *release.BuildPlanRequest) { r.GoToolchain = release.GoToolchainPrimitive2026 + 1 }, wantErr: core.ErrReleaseContract},
		{name: "zero module mode is rejected", mutate: func(r *release.BuildPlanRequest) { r.ModuleMode = release.BuildModuleUnknown }, wantErr: core.ErrReleaseContract},
		{name: "future module mode is rejected", mutate: func(r *release.BuildPlanRequest) { r.ModuleMode = release.BuildModuleVendor + 1 }, wantErr: core.ErrReleaseContract},
		{name: "zero linker assignment set is accepted", mutate: func(r *release.BuildPlanRequest) { r.LinkerAssignments = release.LinkerAssignments{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			got, gotErr := release.PrepareBuildPlan(request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.PrepareBuildPlan() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (release.BuildPlan{}) {
				t.Fatalf("release.PrepareBuildPlan() plan = %v, want zero on rejection", got)
			}
		})
	}
}

func TestLinkerAssignmentsRejectAmbiguousOrPrimitiveOwnedDefinitions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		symbol  string
		value   string
	}{
		{name: "ordinary product variable is accepted", symbol: "github.com/example/product/internal/release.embeddedServerKey", value: "0123"},
		{name: "empty symbol is rejected", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol without package separator is rejected", symbol: "embeddedServerKey", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol with whitespace is rejected", symbol: "github.com/x/y.bad symbol", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol with assignment delimiter is rejected", symbol: "github.com/x/y.bad=symbol", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol whose package begins with a flag prefix is rejected", symbol: "-ldflags/y.value", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol whose variable is not a Go identifier is rejected", symbol: "github.com/x/y.9value", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "symbol ending at the separator is rejected", symbol: "github.com/x/y.", value: "0123", wantErr: core.ErrReleaseContract},
		{name: "value at its exact bound is accepted", symbol: "github.com/x/y.value", value: strings.Repeat("v", 512)},
		{name: "value one byte above its bound is rejected", symbol: "github.com/x/y.value", value: strings.Repeat("v", 513), wantErr: core.ErrReleaseContract},
		{name: "value with a newline is rejected", symbol: "github.com/x/y.value", value: "one\ntwo", wantErr: core.ErrReleaseContract},
		{name: "value with a NUL byte is rejected", symbol: "github.com/x/y.value", value: "one\x00two", wantErr: core.ErrReleaseContract},
		{name: "empty value is rejected", symbol: "github.com/x/y.value", wantErr: core.ErrReleaseContract},
		{name: "value with whitespace is rejected", symbol: "github.com/x/y.value", value: "two values", wantErr: core.ErrReleaseContract},
		{name: "value with assignment delimiter is rejected", symbol: "github.com/x/y.value", value: "left=right", wantErr: core.ErrReleaseContract},
		{name: "Primitive offering stamp cannot be overridden", symbol: release.EmbeddedBuildOfferingLinkSymbol, value: "product", wantErr: core.ErrReleaseContract},
		{name: "Primitive version stamp cannot be overridden", symbol: release.EmbeddedBuildVersionLinkSymbol, value: "2026.0.11", wantErr: core.ErrReleaseContract},
		{name: "Primitive commit stamp cannot be overridden", symbol: release.EmbeddedBuildCommitLinkSymbol, value: "b5c32d95d212b0a1a8cef4126e4d11ff288079ef", wantErr: core.ErrReleaseContract},
		{name: "Primitive platform stamp cannot be overridden", symbol: release.EmbeddedBuildPlatformLinkSymbol, value: "darwin-arm64", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := release.NewLinkerAssignment(tc.symbol, tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.NewLinkerAssignment(%q, %q) error = %v, want %v", tc.symbol, tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (release.LinkerAssignment{}) {
				t.Fatalf("release.NewLinkerAssignment(%q, %q) = %v, want zero on rejection", tc.symbol, tc.value, got)
			}
		})
	}

	first, err := release.NewLinkerAssignment("github.com/x/y.value", "one")
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment(first) error = %v, want nil", err)
	}
	second, err := release.NewLinkerAssignment("github.com/x/y.value", "two")
	if err != nil {
		t.Fatalf("release.NewLinkerAssignment(second) error = %v, want nil", err)
	}
	if _, err := release.NewLinkerAssignments([]release.LinkerAssignment{first, second}); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("release.NewLinkerAssignments(duplicate symbol) error = %v, want %v", err, core.ErrReleaseContract)
	}
}

func TestMainPackageAndLinkerSetBoundEveryCommandProjection(t *testing.T) {
	t.Parallel()

	mainPackages := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "canonical product command is accepted", value: "github.com/example/product/cmd/product"},
		{name: "empty is rejected", wantErr: core.ErrReleaseContract},
		{name: "local path is rejected", value: "./cmd/product", wantErr: core.ErrReleaseContract},
		{name: "absolute path is rejected", value: "/cmd/product", wantErr: core.ErrReleaseContract},
		{name: "single component is rejected", value: "product", wantErr: core.ErrReleaseContract},
		{name: "unclean path is rejected", value: "github.com/example/../product", wantErr: core.ErrReleaseContract},
		{name: "shell delimiter is rejected", value: "github.com/example/product;run", wantErr: core.ErrReleaseContract},
		{name: "non ASCII path is rejected", value: "github.com/offGridSoft/witnéss", wantErr: core.ErrReleaseContract},
		{name: "leading hyphen would reach the go command as a flag", value: "-buildmode=exe/cmd", wantErr: core.ErrReleaseContract},
		{name: "leading hyphen without a delimiter is rejected", value: "-C/tmp", wantErr: core.ErrReleaseContract},
		{name: "single hyphen element is rejected", value: "github.com/-/product", wantErr: core.ErrReleaseContract},
		{name: "interior element beginning with a hyphen is rejected", value: "github.com/-example/product", wantErr: core.ErrReleaseContract},
		{name: "trailing element beginning with a hyphen is rejected", value: "github.com/example/-product", wantErr: core.ErrReleaseContract},
		{name: "interior dot element is rejected", value: "github.com/.hidden/product", wantErr: core.ErrReleaseContract},
		{name: "element ending with a dot is rejected", value: "github.com/example./product", wantErr: core.ErrReleaseContract},
		{name: "interior hyphen inside an element is accepted", value: "github.com/ex-ample/cmd-product"},
		{name: "interior dot inside an element is accepted", value: "gopkg.in/yaml.v3/cmd"},
		{name: "underscore and tilde elements are accepted", value: "example.com/a_b/c~d"},
		{name: "digits and version element are accepted", value: "github.com/deliri/primitive/v2026/release"},
		{name: "two component path is accepted", value: "example.com/tool"},
		{name: "trailing slash is rejected", value: "github.com/example/product/", wantErr: core.ErrReleaseContract},
		{name: "double slash is rejected", value: "github.com//product", wantErr: core.ErrReleaseContract},
		{name: "parent traversal element is rejected", value: "github.com/offGridSoft/../../etc", wantErr: core.ErrReleaseContract},
		{name: "space delimiter is rejected", value: "github.com/ex ample/product", wantErr: core.ErrReleaseContract},
		{name: "newline delimiter is rejected", value: "github.com/example/product\n-o=/tmp/pwn", wantErr: core.ErrReleaseContract},
		{name: "NUL byte is rejected", value: "github.com/example/product\x00", wantErr: core.ErrReleaseContract},
		{name: "exact maximum length is accepted", value: "github.com/" + strings.Repeat("a", 501)},
		{name: "over maximum length is rejected", value: "github.com/" + strings.Repeat("a", 502), wantErr: core.ErrReleaseContract},
	}
	for _, tc := range mainPackages {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := release.ParseMainPackage(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.ParseMainPackage(%q) error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr == nil && got.String() != tc.value {
				t.Fatalf("release.ParseMainPackage(%q).String() = %q", tc.value, got.String())
			}
		})
	}

	values := make([]release.LinkerAssignment, 17)
	for index := range values {
		assignment, err := release.NewLinkerAssignment(
			"github.com/example/product/internal/release.value"+strconv.Itoa(index),
			strconv.Itoa(index+1),
		)
		if err != nil {
			t.Fatalf("release.NewLinkerAssignment(%d) error = %v, want nil", index, err)
		}
		values[index] = assignment
	}
	if _, err := release.NewLinkerAssignments(values); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("release.NewLinkerAssignments(17 values) error = %v, want %v", err, core.ErrReleaseContract)
	}

	set, err := release.NewLinkerAssignments([]release.LinkerAssignment{values[10], values[2]})
	if err != nil {
		t.Fatalf("release.NewLinkerAssignments(two values) error = %v, want nil", err)
	}
	first, ok := set.At(0)
	if !ok || first.Symbol() != values[10].Symbol() || first.Value() != values[10].Value() {
		t.Fatalf("release.LinkerAssignments.At(0) = (%v, %t), want lexical value10", first, ok)
	}
	if _, ok := set.At(-1); ok {
		t.Fatal("release.LinkerAssignments.At(-1) ok = true, want false")
	}
	if _, ok := set.At(2); ok {
		t.Fatal("release.LinkerAssignments.At(2) ok = true, want false")
	}
}

func TestBuildModuleModeExhaustsItsEntireBackingDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		mode := release.BuildModuleMode(raw)
		wantValid := mode == release.BuildModuleReadonly || mode == release.BuildModuleVendor
		if got := mode.IsValid(); got != wantValid {
			t.Fatalf("release.BuildModuleMode(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		if gotErr := mode.Validate(); (gotErr == nil) != wantValid {
			t.Fatalf("release.BuildModuleMode(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
	}
	if gotReadonly, gotVendor := release.BuildModuleReadonly.String(), release.BuildModuleVendor.String(); gotReadonly != "readonly" || gotVendor != "vendor" {
		t.Fatalf("release build module labels = (%q, %q), want (%q, %q)",
			gotReadonly, gotVendor, "readonly", "vendor")
	}
	if got := release.BuildModuleUnknown.String(); got != core.UnknownEnumDiagnostic {
		t.Fatalf("release.BuildModuleUnknown.String() = %q, want %q", got, core.UnknownEnumDiagnostic)
	}
}

func TestGoToolchainIdentityPinsExactReleaseCompilerAcrossItsBackingDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		identity := release.GoToolchainIdentity(raw)
		wantValid := identity == release.GoToolchainPrimitive2026
		if got := identity.IsValid(); got != wantValid {
			t.Fatalf("release.GoToolchainIdentity(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		if gotErr := identity.Validate(); (gotErr == nil) != wantValid {
			t.Fatalf("release.GoToolchainIdentity(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
	}
	current := release.CurrentGoToolchain()
	version, err := current.Version()
	if err != nil {
		t.Fatalf("release.CurrentGoToolchain().Version() error = %v, want nil", err)
	}
	if version != "go1.27.0" {
		t.Fatalf("release.CurrentGoToolchain().Version() = %q, want go1.27.0", version)
	}
	if got := release.GoToolchainUnknown.String(); got != core.UnknownEnumDiagnostic {
		t.Fatalf("release.GoToolchainUnknown.String() = %q, want %q", got, core.UnknownEnumDiagnostic)
	}
}

func buildPlanRequestForHostileTest(t *testing.T) release.BuildPlanRequest {
	t.Helper()

	mainPackage, err := release.ParseMainPackage("github.com/example/product/cmd/product")
	if err != nil {
		t.Fatalf("release.ParseMainPackage() error = %v, want nil", err)
	}
	output, err := core.ParseRelativePath("dist/releases")
	if err != nil {
		t.Fatalf("core.ParseRelativePath() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	return release.BuildPlanRequest{
		Offering: releaseExternalOffering(t, 1), Version: core.NewReleaseVersion(2026, 0, 11), Commit: commit,
		MainPackage: mainPackage, OutputDirectory: output,
		GoToolchain: release.CurrentGoToolchain(), ModuleMode: release.BuildModuleReadonly,
	}
}
