package process_test

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

// TestExecutableNamesTheRunningBinary pins the door to the platform's own
// answer: the observation is exactly what os.Executable reports, admitted as
// an absolute path.
func TestExecutableNamesTheRunningBinary(t *testing.T) {
	t.Parallel()

	got, err := process.Executable()
	if err != nil {
		t.Fatalf("process.Executable() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Executable().Validate() error = %v, want nil", err)
	}
	want, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() oracle error = %v, want nil", err)
	}
	if got.String() != want {
		t.Fatalf("process.Executable() = %q, want the platform's own %q", got.String(), want)
	}
}

func TestLookupAmbientEnvironmentDistinguishesAbsentEmptyAndValue(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	const probeName = "PRIMITIVE_PROCESS_LOOKUP_PROBE"
	name, err := process.NewEnvironmentName(probeName)
	if err != nil {
		t.Fatalf("process.NewEnvironmentName() error = %v, want nil", err)
	}

	t.Setenv(probeName, "")
	empty, err := process.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("LookupAmbientEnvironment(present empty) error = %v, want nil", err)
	}
	emptyValue, valueErr := empty.Value.Value()
	if valueErr != nil {
		t.Fatalf("present-empty EnvironmentValue.Value() error = %v, want nil", valueErr)
	}
	if empty.Presence != process.EnvironmentPresencePresent || emptyValue != "" {
		t.Fatalf("LookupAmbientEnvironment(present empty) = %+v, want present with exact empty value", empty)
	}
	if err := empty.Validate(); err != nil {
		t.Fatalf("present-empty EnvironmentLookup.Validate() error = %v, want nil", err)
	}

	t.Setenv(probeName, "ambient-value")
	present, err := process.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("LookupAmbientEnvironment(present value) error = %v, want nil", err)
	}
	presentValue, valueErr := present.Value.Value()
	if valueErr != nil {
		t.Fatalf("present-value EnvironmentValue.Value() error = %v, want nil", valueErr)
	}
	if present.Presence != process.EnvironmentPresencePresent || presentValue != "ambient-value" {
		t.Fatalf("LookupAmbientEnvironment(present value) = %+v/%q, want present/ambient-value", present, presentValue)
	}
	if err := present.Validate(); err != nil {
		t.Fatalf("present-value EnvironmentLookup.Validate() error = %v, want nil", err)
	}

	if err := os.Unsetenv(probeName); err != nil {
		t.Fatalf("os.Unsetenv(probe) error = %v, want nil", err)
	}
	absent, err := process.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("LookupAmbientEnvironment(absent) error = %v, want nil", err)
	}
	if absent.Presence != process.EnvironmentPresenceAbsent || absent.Value != (process.EnvironmentValue{}) {
		t.Fatalf("LookupAmbientEnvironment(absent) = %+v, want absent with zero value", absent)
	}
	if err := absent.Validate(); err != nil {
		t.Fatalf("absent EnvironmentLookup.Validate() error = %v, want nil", err)
	}
}

func TestLookupAmbientEnvironmentRejectsZeroNameBeforeObservation(t *testing.T) {
	t.Parallel()

	got, err := process.LookupAmbientEnvironment(process.EnvironmentName{})
	if !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("LookupAmbientEnvironment(zero name) error = %v, want errors.Is(..., ErrProcessContract)", err)
	}
	if got != (process.EnvironmentLookup{}) {
		t.Fatalf("LookupAmbientEnvironment(zero name) = %+v, want zero result", got)
	}
}

// TestAmbientEnvironmentCarriesALiveVariableThroughTheTypedAdmission proves
// the door reads this process's real environment and answers in the typed
// Environment vocabulary: a variable set for this test is present in the
// projection, with the exact substrate spelling.
func TestAmbientEnvironmentCarriesALiveVariableThroughTheTypedAdmission(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	t.Setenv("PROCESS_AMBIENT_PROBE", "ambient-value")
	ambient, err := process.AmbientEnvironment()
	if err != nil {
		t.Fatalf("process.AmbientEnvironment() error = %v, want nil", err)
	}
	if err := ambient.Validate(); err != nil {
		t.Fatalf("AmbientEnvironment().Validate() error = %v, want nil", err)
	}
	values, err := ambient.Strings()
	if err != nil {
		t.Fatalf("AmbientEnvironment().Strings() error = %v, want nil", err)
	}
	if !slices.Contains(values, "PROCESS_AMBIENT_PROBE=ambient-value") {
		t.Fatalf("AmbientEnvironment() = %d entries without the probe, want PROCESS_AMBIENT_PROBE=ambient-value present", len(values))
	}
}

// TestAmbientEnvironmentIsTheEffectiveAdmission holds the two doors to one
// admission rule: what AmbientEnvironment answers is exactly what
// ParseEffectiveEnvironment admits from the same substrate read, so a
// product filtering its inheritance can round-trip through Strings and
// re-admission without changing a byte.
func TestAmbientEnvironmentIsTheEffectiveAdmission(t *testing.T) {
	t.Parallel()

	ambient, err := process.AmbientEnvironment()
	if err != nil {
		t.Fatalf("process.AmbientEnvironment() error = %v, want nil", err)
	}
	values, err := ambient.Strings()
	if err != nil {
		t.Fatalf("AmbientEnvironment().Strings() error = %v, want nil", err)
	}
	readmitted, err := process.ParseEffectiveEnvironment(values)
	if err != nil {
		t.Fatalf("ParseEffectiveEnvironment(round trip) error = %v, want nil", err)
	}
	roundTrip, err := readmitted.Strings()
	if err != nil {
		t.Fatalf("round-trip Strings() error = %v, want nil", err)
	}
	if !slices.Equal(roundTrip, values) {
		t.Fatalf("round-trip environment = %d entries, want the identical %d-entry projection", len(roundTrip), len(values))
	}
}

func TestAmbientArgumentsPreserveTheCommandOwnedArgvProjection(t *testing.T) {
	t.Parallel()

	got, err := process.AmbientArguments()
	if err != nil {
		t.Fatalf("process.AmbientArguments() error = %v, want nil", err)
	}
	want := os.Args[1:]
	if len(got) != len(want) {
		t.Fatalf("process.AmbientArguments() length = %d, want %d", len(got), len(want))
	}
	for index := range got {
		value, valueErr := got[index].Value()
		if valueErr != nil {
			t.Fatalf("AmbientArguments()[%d].Value() error = %v, want nil", index, valueErr)
		}
		if value != want[index] {
			t.Fatalf("AmbientArguments()[%d] = %q, want %q", index, value, want[index])
		}
	}
}

func TestStandardStreamsExposeTheExactCallingProcessCapabilities(t *testing.T) {
	t.Parallel()

	got, err := process.StandardStreams()
	if err != nil {
		t.Fatalf("process.StandardStreams() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("StandardStreams().Validate() error = %v, want nil", err)
	}
	if got.Stdin != os.Stdin || got.Stdout != os.Stdout || got.Stderr != os.Stderr {
		t.Fatalf("process.StandardStreams() = (%p, %p, %p), want exact standard capabilities (%p, %p, %p)", got.Stdin, got.Stdout, got.Stderr, os.Stdin, os.Stdout, os.Stderr)
	}
}
