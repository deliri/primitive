package hostfacts_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
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

func TestAmbientLookupLayerTriad(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	const probeName = "PRIMITIVE_HOSTFACTS_LOOKUP_PROBE"
	name, err := process.NewEnvironmentName(probeName)
	if err != nil {
		t.Fatalf("probe name = %v, want nil", err)
	}
	for _, tc := range []struct {
		name, value string
		present     bool
		invalidName bool
		wantErr     error
	}{
		{name: "absent variable cannot become present empty"},
		{name: "zero name is caller refusal even with present OS value", present: true, value: "present", invalidName: true, wantErr: core.ErrProcessContract},
		{name: "present empty is distinct from absence", present: true},
		{name: "equals sign remains value data", present: true, value: "left=right"},
		{name: "unicode remains exact value data", present: true, value: "café"},
		{name: "multiline value is not trimmed", present: true, value: " first\nlast "},
		{name: "exact value ceiling remains admitted", present: true, value: strings.Repeat("v", int(process.EnvironmentValueMaximumBytes))},
		{name: "one above value ceiling returns no partial lookup", present: true, value: strings.Repeat("v", int(process.EnvironmentValueMaximumBytes)+1), wantErr: core.ErrProcessContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
			t.Setenv(probeName, tc.value)
			if !tc.present {
				if err := os.Unsetenv(probeName); err != nil {
					t.Fatalf("unset probe = %v, want nil", err)
				}
			}
			input := name
			if tc.invalidName {
				input = process.EnvironmentName{}
			}
			got, err := hostfacts.LookupAmbientEnvironment(input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lookup = %+v/%v, want %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				wantIdentity, forbiddenIdentity := core.ErrHostFactsObservation, core.ErrHostFactsContract
				if tc.invalidName {
					wantIdentity, forbiddenIdentity = forbiddenIdentity, wantIdentity
				}
				if got != (process.EnvironmentLookup{}) || !errors.Is(err, wantIdentity) || errors.Is(err, forbiddenIdentity) {
					t.Fatalf("refused lookup = %+v/%v, want zero with correct caller/OS identity", got, err)
				}
				return
			}
			want := process.EnvironmentLookup{Presence: process.EnvironmentPresenceAbsent}
			if tc.present {
				value, err := process.NewEnvironmentValue(tc.value)
				if err != nil {
					t.Fatalf("value fixture = %v, want nil", err)
				}
				want = process.EnvironmentLookup{Presence: process.EnvironmentPresencePresent, Value: value}
			}
			if got != want || got.Validate() != nil {
				t.Fatalf("lookup = %+v, want %+v", got, want)
			}
		})
	}
}

func TestResolveWorkingPathCallerBoundaryTable(t *testing.T) {
	t.Parallel()
	working, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, text string
		want       core.AbsolutePath
		wantErr    error
	}{
		{name: "relative dot retains observed coordinate", text: ".", want: working},
		{name: "absolute coordinate is not joined twice", text: working.String(), want: working},
		{name: "empty input cannot become working directory", wantErr: core.ErrHostFactsContract},
		{name: "embedded NUL cannot reach file system", text: "child\x00tail", wantErr: core.ErrHostFactsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := hostfacts.ResolveWorkingPath(t.Context(), tc.text)
			if got != tc.want || !errors.Is(err, tc.wantErr) || errors.Is(err, core.ErrHostFactsObservation) {
				t.Fatalf("resolve = %v/%v, want %v/%v without observation identity", got, err, tc.want, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("refusal = %v, lost Core contract", err)
			}
		})
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
