package hostfacts_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestExecutableNamesTheRunningBinary(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.Executable()
	if err != nil {
		t.Fatalf("hostfacts.Executable() error = %v, want nil", err)
	}
	want, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() oracle error = %v, want nil", err)
	}
	if got.Validate() != nil || got.String() != want {
		t.Fatalf("hostfacts.Executable() = %q, want the platform's own %q", got.String(), want)
	}
}

func TestWorkingDirectoryNamesTheCallingProcessDirectory(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() oracle error = %v, want nil", err)
	}
	if got.Validate() != nil || got.String() != want {
		t.Fatalf("hostfacts.WorkingDirectory() = %q, want %q", got.String(), want)
	}

	wantResolved, err := got.ResolveText("hostfacts")
	if err != nil {
		t.Fatalf("AbsolutePath.ResolveText(hostfacts) oracle error = %v, want nil", err)
	}
	resolved, err := hostfacts.ResolveWorkingPath(t.Context(), "hostfacts")
	if err != nil || resolved != wantResolved {
		t.Fatalf("hostfacts.ResolveWorkingPath(hostfacts) = (%q, %v), want (%q, nil)", resolved.String(), err, wantResolved.String())
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	refused, gotErr := hostfacts.ResolveWorkingPath(ctx, "hostfacts")
	if !errors.Is(gotErr, context.Canceled) || refused != (core.AbsolutePath{}) {
		t.Fatalf("hostfacts.ResolveWorkingPath(cancelled) = (%q, %v), want zero and context.Canceled", refused.String(), gotErr)
	}
}

func TestLookupAmbientEnvironmentDistinguishesAbsentEmptyAndValue(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	const probeName = "PRIMITIVE_HOSTFACTS_LOOKUP_PROBE"
	name, err := process.NewEnvironmentName(probeName)
	if err != nil {
		t.Fatalf("process.NewEnvironmentName() error = %v, want nil", err)
	}

	t.Setenv(probeName, "")
	empty, err := hostfacts.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("LookupAmbientEnvironment(present empty) error = %v, want nil", err)
	}
	emptyValue, valueErr := empty.Value.Value()
	if valueErr != nil || empty.Presence != process.EnvironmentPresencePresent || emptyValue != "" {
		t.Fatalf("LookupAmbientEnvironment(present empty) = %+v/%q error:%v, want present empty", empty, emptyValue, valueErr)
	}

	t.Setenv(probeName, "ambient-value")
	present, err := hostfacts.LookupAmbientEnvironment(name)
	if err != nil {
		t.Fatalf("LookupAmbientEnvironment(present value) error = %v, want nil", err)
	}
	presentValue, valueErr := present.Value.Value()
	if valueErr != nil || present.Presence != process.EnvironmentPresencePresent || presentValue != "ambient-value" {
		t.Fatalf("LookupAmbientEnvironment(present value) = %+v/%q error:%v, want present ambient-value", present, presentValue, valueErr)
	}

}

func TestLookupAmbientEnvironmentRejectsZeroNameBeforeObservation(t *testing.T) {
	t.Parallel()

	got, err := hostfacts.LookupAmbientEnvironment(process.EnvironmentName{})
	if !errors.Is(err, core.ErrHostFactsObservation) || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("LookupAmbientEnvironment(zero name) error = %v, want HostFacts observation and Process contract", err)
	}
	if got != (process.EnvironmentLookup{}) {
		t.Fatalf("LookupAmbientEnvironment(zero name) = %+v, want zero", got)
	}
}

func TestAmbientEnvironmentCarriesALiveVariableThroughTypedAdmission(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	t.Setenv("HOSTFACTS_AMBIENT_PROBE", "ambient-value")
	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
	}
	values, err := ambient.Strings()
	if err != nil || !slices.Contains(values, "HOSTFACTS_AMBIENT_PROBE=ambient-value") {
		t.Fatalf("AmbientEnvironment() = %d entries error:%v, want exact probe", len(values), err)
	}
}

func TestAmbientEnvironmentRoundTripsThroughProcessAgreement(t *testing.T) {
	t.Parallel()

	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
	}
	values, err := ambient.Strings()
	if err != nil {
		t.Fatalf("AmbientEnvironment().Strings() error = %v, want nil", err)
	}
	readmitted, err := process.ParseEffectiveEnvironment(values)
	if err != nil {
		t.Fatalf("process.ParseEffectiveEnvironment(round trip) error = %v, want nil", err)
	}
	roundTrip, err := readmitted.Strings()
	if err != nil || !slices.Equal(roundTrip, values) {
		t.Fatalf("round-trip environment = %d entries error:%v, want identical %d entries", len(roundTrip), err, len(values))
	}
}
