package gotoolchain_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
)

func TestCompilerEnvironmentLayerTriad(t *testing.T) {
	t.Parallel()
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	directory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	request := gotoolchain.ObservationRequest{WorkingDirectory: directory}
	environment := compilerEnvironmentFixture(t)
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &environment})
	if err != nil {
		t.Fatalf("Open(exact environment) error = %v, want nil", err)
	}
	// Mutate the caller's original backing slice after admission. The captured
	// environment must remain owned, including its configuration validation.
	for index := range environment.Variables {
		environment.Variables[index] = process.EnvironmentVariable{}
	}
	observed, err := capability.ObserveBuildContext(t.Context(), request)
	wantPlatform := core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64}
	if err != nil || observed.Platform != wantPlatform || observed.CGOEnabled {
		t.Fatalf("ObserveBuildContext(captured exact environment) = (%+v,%v), want platform %+v and cgo disabled", observed, err, wantPlatform)
	}
	poisoned := compilerEnvironmentFixture(t)
	poisoned = compilerEnvironmentVariable(t, poisoned, "GOFLAGS", "-primitive-invalid-build-flag")
	poisonedCapability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &poisoned})
	if err != nil {
		t.Fatalf("Open(admitted process data) error = %v, want nil", err)
	}
	refused, err := poisonedCapability.ObserveModule(t.Context(), request)
	if !errors.Is(err, core.ErrGoToolchainExecution) || refused.String() != "" {
		t.Fatalf("ObserveModule(invalid Go flag) = (%v,%v), want zero module and execution refusal", refused, err)
	}
	ambient, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatalf("Open(ambient capture) error = %v, want nil", err)
	}
	ambientModule, err := ambient.ObserveModule(t.Context(), request)
	if err != nil || ambientModule.String() != core.PrimitiveModulePath {
		t.Fatalf("ObserveModule(ambient capture) = (%v,%v), want %s and nil", ambientModule, err, core.PrimitiveModulePath)
	}
}

func TestCompilerEnvironmentRejectsAmbiguousSuppliedModes(t *testing.T) {
	t.Parallel()
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []process.EnvironmentMode{process.EnvironmentModeUnknown, process.EnvironmentModeInherit} {
		environment := process.Environment{Mode: mode}
		_, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &environment})
		if !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("Open(environment mode %v) error = %v, want Go toolchain contract refusal", mode, err)
		}
	}
}

func compilerEnvironmentFixture(t testing.TB) process.Environment {
	t.Helper()
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []struct{ name, value string }{
		{"GOOS", core.OperatingSystemLinux.String()},
		{"GOARCH", core.CPUArchitectureAMD64.String()},
		{"CGO_ENABLED", "0"}, {"GOENV", "off"}, {"GOFLAGS", ""}, {"GOTOOLCHAIN", "local"},
	} {
		environment = compilerEnvironmentVariable(t, environment, variable.name, variable.value)
	}
	return environment
}

func compilerEnvironmentVariable(t testing.TB, environment process.Environment, nameText, valueText string) process.Environment {
	t.Helper()
	name, err := process.NewEnvironmentName(nameText)
	if err != nil {
		t.Fatal(err)
	}
	value, err := process.NewEnvironmentValue(valueText)
	if err != nil {
		t.Fatal(err)
	}
	result, err := environment.With(process.EnvironmentVariable{Name: name, Value: value})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
