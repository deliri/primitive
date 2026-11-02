package temporal_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

// Consumers validate a zone name inside every resource-day, phase and hours
// check. Re-decoding the 598-member archive directory on each call allocated
// well over a thousand objects per validation; the directory is authored data
// and is decoded once, while every call still decodes its own TZif member.
func TestNewLocationDoesNotRedecodeTheEmbeddedDirectoryPerCall(t *testing.T) {
	// AllocsPerRun observes process-wide allocation and sets GOMAXPROCS to 1,
	// so this test runs serially under the declared isolation hazard.
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation,
		Scope:  core.TestIsolationScopePackageProcess,
	})
	var location temporal.Location
	var err error
	allocations := testing.AllocsPerRun(20, func() { location, err = temporal.NewLocation("America/Toronto") })
	if err != nil || location.Name() != "America/Toronto" {
		t.Fatalf("NewLocation(America/Toronto) = (%v, %v), want the embedded zone", location, err)
	}
	const directoryEntries = 598
	if allocations >= directoryEntries {
		t.Fatalf("NewLocation allocations = %v, want fewer than one per archive member (%d): the directory must not be decoded per call", allocations, directoryEntries)
	}
	instant, err := temporal.ParseRFC3339UTC("2026-03-08T07:00:00Z")
	if err != nil {
		t.Fatalf("owned instant = %v, want nil", err)
	}
	got, err := location.Format(instant, "2006-01-02T15:04:05Z07:00")
	if err != nil || got != "2026-03-08T03:00:00-04:00" {
		t.Fatalf("embedded transition = %q/%v, want Toronto first post-gap clock", got, err)
	}
}

// Independent capabilities from concurrent callers share only the authored
// directory; the race detector proves the shared reader is read-only.
func TestNewLocationConcurrentCallersOwnIndependentCapabilities(t *testing.T) {
	t.Parallel()
	for _, name := range []temporal.LocationName{"America/Toronto", "Europe/London", "Australia/Sydney", "Pacific/Apia", temporal.LocationUTC} {
		t.Run(string(name), func(t *testing.T) {
			t.Parallel()
			first, err := temporal.NewLocation(name)
			if err != nil || first.Name() != name {
				t.Fatalf("NewLocation(%s) = (%v, %v), want the embedded zone", name, first, err)
			}
			second, err := temporal.NewLocation(name)
			if err != nil || second.Name() != name {
				t.Fatalf("second NewLocation(%s) = (%v, %v), want the embedded zone", name, second, err)
			}
		})
	}
}

func BenchmarkNewLocation(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := temporal.NewLocation("America/Toronto"); err != nil {
			b.Fatalf("NewLocation() error = %v, want nil", err)
		}
	}
}
