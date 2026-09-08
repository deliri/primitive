package hostfacts

import (
	"errors"
	"io/fs"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestPhysicalMemoryPlatformResultLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		total    uint64
		cause    error
		identity core.ErrorIdentity
	}{
		{name: "one byte remains an exact nonzero fact", total: 1},
		{name: "maximum signed extent remains exact", total: math.MaxInt64},
		{name: "one above signed extent is refused", total: uint64(math.MaxInt64) + 1, identity: core.ErrHostFactsObservation},
		{name: "maximum unsigned total cannot wrap", total: math.MaxUint64, identity: core.ErrHostFactsObservation},
		{name: "zero total cannot become an absent success", identity: core.ErrHostFactsObservation},
		{name: "unsupported leaf retains unsupported identity", cause: core.ErrHostFactsUnsupported, identity: core.ErrHostFactsUnsupported},
		{name: "wrapped unsupported leaf retains unsupported identity", cause: errors.Join(core.ErrHostFactsUnsupported, fs.ErrInvalid), identity: core.ErrHostFactsUnsupported},
		{name: "native failure cannot become unsupported", cause: fs.ErrPermission, identity: core.ErrHostFactsObservation},
		{name: "native failure discards nonzero partial total", total: 1, cause: fs.ErrPermission, identity: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := observedPhysicalMemory(tc.total, tc.cause)
			if tc.identity == core.ErrUnknown {
				if err != nil || got.Validate() != nil || got.TotalBytes().Uint64() != tc.total {
					t.Fatalf("memory = %+v/%v, want exact %d", got, err, tc.total)
				}
				return
			}
			var failure Failure
			if got != (PhysicalMemory{}) || !errors.As(err, &failure) || failure.Operation != OperationPhysicalMemory || failure.Identity != tc.identity || !errors.Is(err, tc.identity) || (tc.cause != nil && !errors.Is(err, tc.cause)) {
				t.Fatalf("memory = %+v/%v (%+v), want zero with exact %v and original cause", got, err, failure, tc.identity)
			}
			if tc.identity == core.ErrHostFactsUnsupported && errors.Is(err, core.ErrHostFactsObservation) {
				t.Fatalf("unsupported refusal also matches observation: %v", err)
			}
		})
	}
}
