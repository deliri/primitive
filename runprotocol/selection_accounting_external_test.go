package runprotocol_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func selectionAccountingObservation(planned, admitted, refused, executed, notRun uint16) runprotocol.SelectionObservation {
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	return runprotocol.SelectionObservation{ExpansionIdentity: digest, ManifestDigest: digest, Planned: planned, Admitted: admitted, Refused: refused, Executed: executed, NotRun: notRun}
}

func TestSelectionAccountingRefusesWrappedCounts(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			// Each identity was admitted by uint16 addition: 65535 + n
			// wraps to n-1. Widened arithmetic must refuse both equations.
			n := uint16(index + 1)
			plannedWrap := selectionAccountingObservation(n-1, 65535, n, 65535, 0)
			executionWrap := selectionAccountingObservation(n-1, n-1, 0, 65535, n)
			for _, observation := range [...]runprotocol.SelectionObservation{plannedWrap, executionWrap} {
				if err := observation.Validate(); !errors.Is(err, core.ErrRunProtocolConflict) {
					t.Fatalf("wrapped counts admitted: %+v: %v", observation, err)
				}
			}
			valid := selectionAccountingObservation(65535, 65535-n, n, 65535-n, 0)
			if err := valid.Validate(); err != nil {
				t.Fatalf("exact maximum-width accounting refused: %v", err)
			}
		})
	}
}

func FuzzSelectionAccountingClosesWithoutOverflow(f *testing.F) {
	f.Add(uint16(0), uint16(65535), uint16(1), uint16(65535), uint16(0))
	f.Add(uint16(0), uint16(0), uint16(0), uint16(65535), uint16(1))
	f.Add(uint16(65535), uint16(65534), uint16(1), uint16(65534), uint16(0))
	f.Fuzz(func(t *testing.T, planned, admitted, refused, executed, notRun uint16) {
		// int64 is independent of the production width and covers every input.
		closes := int64(admitted)+int64(refused) == int64(planned) && int64(executed)+int64(notRun) == int64(admitted)
		err := selectionAccountingObservation(planned, admitted, refused, executed, notRun).Validate()
		if closes && err != nil || !closes && !errors.Is(err, core.ErrRunProtocolConflict) {
			t.Fatalf("accounting closure=%v: %v", closes, err)
		}
	})
}
