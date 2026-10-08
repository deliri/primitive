package hostfacts_test

import (
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestGoMemoryLimitReportsApplicationFromAnExistingNativeZeroLimit(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	previous := debug.SetMemoryLimit(0)
	defer debug.SetMemoryLimit(previous)
	limit, err := core.NewByteLength(1 << 30)
	if err != nil {
		t.Fatal(err)
	}
	got, err := hostfacts.ApplyGoMemoryLimit(t.Context(), hostfacts.GoMemoryLimitRequest{Limit: limit})
	actual := debug.SetMemoryLimit(-1)
	if err != nil || got.Validate() != nil || got.Previous.Uint64() != 0 || got.Applied != limit || actual != 1<<30 {
		t.Fatalf("application from native zero = (%v, %v), runtime:%d, want admitted application of %d bytes with exact zero before state", got, err, actual, 1<<30)
	}
}

func TestGoMemoryLimitDistinguishesRealZeroApplicationFromMissingObservation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	previous := debug.SetMemoryLimit(0)
	defer debug.SetMemoryLimit(previous)
	request := hostfacts.GoMemoryLimitRequest{}
	got, err := hostfacts.ApplyGoMemoryLimit(t.Context(), request)
	if request.Validate() != nil || err != nil || got.Validate() != nil || got == (hostfacts.GoMemoryLimitResult{}) || got.Previous.Uint64() != 0 || got.Applied.Uint64() != 0 || debug.SetMemoryLimit(-1) != 0 {
		t.Fatalf("native zero application = (%v, %v), want valid zero request, exact zero coordinates and completed observation", got, err)
	}
	if err := (hostfacts.GoMemoryLimitResult{}).Validate(); err == nil {
		t.Fatal("missing memory observation validated as an applied operation")
	}
}
