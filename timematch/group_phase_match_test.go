package timematch

import (
	"context"
	"github.com/deliri/primitive/v2026/temporal"
	"testing"
)

// This initial old-API fixture is a FEATURE GAP, not a bug in whole-service
// matching: a resource is needed at [0,2) and [4,6), leaving [2,4) processing.
// A whole-service approximation cannot express its valid start at zero.
func TestGroupPhaseProcessingGap(t *testing.T) {
	r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(6)}, Duration: groupDuration(t, 6)}
	r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{2, 4})}
	got, err := MatchGroup(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || groupNS(t, got.Windows[0].Starts.Earliest) != 0 || groupNS(t, got.Windows[0].Starts.Latest) != 0 {
		t.Fatalf("processing-gap start zero must remain available for phases [0,2),[4,6); whole-service approximation result=%+v", got)
	}
}
