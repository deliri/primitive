package process_test

import (
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
