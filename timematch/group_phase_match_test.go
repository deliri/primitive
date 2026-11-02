package timematch

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func phaseFixture(tb testing.TB) GroupMatchRequest {
	tb.Helper()
	r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(8)}, Duration: groupDuration(tb, 4)}
	a := groupPerson(tb, r.Window, 0, true, AvailabilityKnown).Availability
	r.PhaseConstraints = []GroupPhaseConstraint{{Ordinal: 0, Availability: a, Duration: groupDuration(tb, 1)}, {Ordinal: 1, Availability: a, Offset: groupDuration(tb, 3), Duration: groupDuration(tb, 1)}}
	return r
}

// The archived old-API checkpoint demonstrates an inexpressible FEATURE GAP,
// not an old whole-service matching bug. This actual phase request is the
// correctness contract: [2,4) processing must not consume either staff phase.
func TestGroupPhaseProcessingGap(t *testing.T) {
	r := phaseFixture(t)
	r.Window.End = temporal.InstantFromNanoseconds(6)
	r.Duration = groupDuration(t, 6)
	for i := range r.PhaseConstraints {
		r.PhaseConstraints[i].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{2, 4}).Availability
		r.PhaseConstraints[i].Duration = groupDuration(t, 2)
	}
	r.PhaseConstraints[1].Offset = groupDuration(t, 4)
	got, err := MatchGroup(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || groupNS(t, got.Windows[0].Starts.Earliest) != 0 || groupNS(t, got.Windows[0].Starts.Latest) != 0 || got.RequiredCount != 0 || len(got.RemovalImpacts) != 0 {
		t.Fatalf("processing gap must retain start zero without a phantom person: %+v", got)
	}
	phaseCompare(t, got, phaseBrute(t, r))
}

// Direct occupancy oracle: enumerate actual starts and test half-open overlap.
// It never expands busy intervals, computes forbidden ranges or calls the sweep.
func phaseBrute(tb testing.TB, r GroupMatchRequest) GroupMatchResult {
	tb.Helper()
	out := GroupMatchResult{}
	people := slices.Clone(r.Participants)
	slices.SortFunc(people, func(a, b GroupParticipant) int {
		if a.Ordinal < b.Ordinal {
			return -1
		}
		if a.Ordinal > b.Ordinal {
			return 1
		}
		return 0
	})
	for _, p := range people {
		unknown := false
		for _, c := range p.Availability.Calendars {
			unknown = unknown || c.State == AvailabilityUnknown
		}
		out.RemovalImpacts = append(out.RemovalImpacts, GroupRemovalImpact{Ordinal: p.Ordinal, Required: p.Required, Unknown: unknown})
		if p.Required {
			out.RequiredCount++
		}
		if unknown {
			if p.Required {
				out.UnknownRequired = append(out.UnknownRequired, p.Ordinal)
			} else {
				out.UnknownOptional = append(out.UnknownOptional, p.Ordinal)
			}
		} else if !p.Required {
			out.KnownOptionalCount++
		}
	}
	for _, p := range r.PhaseConstraints {
		unknown := false
		for _, c := range p.Availability.Calendars {
			unknown = unknown || c.State == AvailabilityUnknown
		}
		if unknown {
			out.UnknownPhaseConstraints = append(out.UnknownPhaseConstraints, p.Ordinal)
		}
	}
	slices.Sort(out.UnknownPhaseConstraints)
	origin, end := groupNS(tb, r.Window.Start), groupNS(tb, r.Window.End)
	last := end - r.Duration.Nanoseconds() - r.After.Nanoseconds()
	for start := origin + r.Before.Nanoseconds(); start <= last; start++ {
		hardFree := true
		for _, p := range r.PhaseConstraints {
			left := start + p.Offset.Nanoseconds() - p.Before.Nanoseconds()
			right := start + p.Offset.Nanoseconds() + p.Duration.Nanoseconds() + p.After.Nanoseconds()
			if left < origin || right > end {
				hardFree = false
			}
			if !phaseAvailable(tb, p.Availability, left, right) {
				hardFree = false
			}
		}
		if !hardFree {
			continue
		}
		var blocked []int
		var score uint64
		for i, p := range people {
			free := phaseAvailable(tb, p.Availability, start-r.Before.Nanoseconds(), start+r.Duration.Nanoseconds()+r.After.Nanoseconds())
			if p.Required && !free {
				blocked = append(blocked, i)
			}
			if !p.Required && free {
				score++
			}
		}
		if len(blocked) == 0 {
			out.FeasibleStartNanoseconds++
			n := len(out.Windows)
			if n > 0 && out.Windows[n-1].OptionalAvailable == score && groupNS(tb, out.Windows[n-1].Starts.Latest) == start-1 {
				out.Windows[n-1].Starts.Latest = temporal.InstantFromNanoseconds(start)
			} else {
				out.Windows = append(out.Windows, GroupWindow{Starts: GroupStartRange{Earliest: temporal.InstantFromNanoseconds(start), Latest: temporal.InstantFromNanoseconds(start)}, OptionalAvailable: score})
			}
		}
		if len(blocked) == 1 {
			v := &out.RemovalImpacts[blocked[0]]
			if v.AdditionalStartNanoseconds == 0 {
				v.EarliestUnlocked = temporal.InstantFromNanoseconds(start)
			}
			v.AdditionalStartNanoseconds++
		}
	}
	slices.SortFunc(out.Windows, func(a, b GroupWindow) int {
		if a.OptionalAvailable > b.OptionalAvailable {
			return -1
		}
		if a.OptionalAvailable < b.OptionalAvailable {
			return 1
		}
		if groupNS(tb, a.Starts.Earliest) < groupNS(tb, b.Starts.Earliest) {
			return -1
		}
		return 1
	})
	if r.MaximumWindows > 0 && uint64(len(out.Windows)) > r.MaximumWindows {
		out.Windows = out.Windows[:int(r.MaximumWindows)]
		out.Truncated = true
	}
	slices.SortFunc(out.RemovalImpacts, func(a, b GroupRemovalImpact) int {
		if a.AdditionalStartNanoseconds > b.AdditionalStartNanoseconds {
			return -1
		}
		if a.AdditionalStartNanoseconds < b.AdditionalStartNanoseconds {
			return 1
		}
		if a.Ordinal < b.Ordinal {
			return -1
		}
		if a.Ordinal > b.Ordinal {
			return 1
		}
		return 0
	})
	return out
}
func phaseAvailable(tb testing.TB, a Availability, left, right int64) bool {
	tb.Helper()
	for _, c := range a.Calendars {
		if c.State != AvailabilityKnown {
			return false
		}
		for _, b := range c.Busy {
			if left < groupNS(tb, b.End) && right > groupNS(tb, b.Start) {
				return false
			}
		}
	}
	return true
}
func phaseCompare(t *testing.T, got, want GroupMatchResult) {
	t.Helper()
	groupCompareResult(t, got, want)
	if !slices.Equal(got.UnknownPhaseConstraints, want.UnknownPhaseConstraints) {
		t.Fatalf("unknown phases=%v want %v", got.UnknownPhaseConstraints, want.UnknownPhaseConstraints)
	}
}
func phaseClone(r GroupMatchRequest) GroupMatchRequest {
	r = groupCloneRequest(r)
	r.PhaseConstraints = slices.Clone(r.PhaseConstraints)
	for i := range r.PhaseConstraints {
		a := &r.PhaseConstraints[i].Availability
		a.Calendars = slices.Clone(a.Calendars)
		for j := range a.Calendars {
			a.Calendars[j].Busy = slices.Clone(a.Calendars[j].Busy)
		}
	}
	return r
}

func TestGroupPhaseFiniteOccupancyOracle(t *testing.T) {
	for mask := 0; mask < 4096; mask++ {
		r := phaseFixture(t)
		for i := range r.PhaseConstraints {
			r.PhaseConstraints[i].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, phaseMask(uint8(mask&63))...).Availability
		}
		r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown, phaseMask(uint8(mask>>6&15))...), groupPerson(t, r.Window, 1, false, AvailabilityKnown, phaseMask(uint8(mask>>10&3))...)}
		r.MaximumWindows = uint64(mask % 3)
		before := phaseClone(r)
		got, err := MatchGroup(t.Context(), r)
		if err != nil {
			t.Fatalf("mask=%d: %v", mask, err)
		}
		phaseCompare(t, got, phaseBrute(t, r))
		if !reflect.DeepEqual(r, before) {
			t.Fatalf("mask=%d mutated input", mask)
		}
	}
}
func phaseMask(mask uint8) [][2]int64 {
	var spans [][2]int64
	for i := int64(0); i < 6; i++ {
		if mask&(1<<i) != 0 {
			spans = append(spans, [2]int64{i, i + 1})
		}
	}
	return spans
}

func TestGroupPhaseHostileContracts(t *testing.T) {
	cases := []struct {
		name   string
		change func(*GroupMatchRequest)
	}{
		{"duplicate ordinal", func(r *GroupMatchRequest) { r.PhaseConstraints[1].Ordinal = 0 }},
		{"window mismatch", func(r *GroupMatchRequest) {
			r.PhaseConstraints[0].Availability.Window.End = temporal.InstantFromNanoseconds(7)
		}},
		{"zero duration", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Duration = temporal.Duration{} }},
		{"phase exceeds service", func(r *GroupMatchRequest) { r.PhaseConstraints[1].Duration = groupDuration(t, 2) }},
		{"offset exceeds service", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Offset = groupDuration(t, 5) }},
		{"offset add overflow", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Offset = groupDuration(t, math.MaxInt64) }},
		{"before add overflow", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Before = groupDuration(t, math.MaxInt64) }},
		{"after add overflow", func(r *GroupMatchRequest) { r.PhaseConstraints[0].After = groupDuration(t, math.MaxInt64) }},
		{"invalid state", func(r *GroupMatchRequest) {
			r.PhaseConstraints[0].Availability.Calendars[0].State = AvailabilityInvalid
		}},
		{"unknown with busy", func(r *GroupMatchRequest) {
			c := &r.PhaseConstraints[0].Availability.Calendars[0]
			c.State = AvailabilityUnknown
			c.Busy = []temporal.IntervalBounds{{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(1)}}
		}},
		{"busy outside window", func(r *GroupMatchRequest) {
			r.PhaseConstraints[0].Availability.Calendars[0].Busy = []temporal.IntervalBounds{{Start: temporal.InstantFromNanoseconds(-1), End: temporal.InstantFromNanoseconds(1)}}
		}},
		{"missing knowledge", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Availability.Calendars = nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := phaseFixture(t)
			c.change(&r)
			before := phaseClone(r)
			got, err := MatchGroup(t.Context(), r)
			if !errors.Is(err, core.ErrTimeMatchContract) || !reflect.DeepEqual(got, GroupMatchResult{}) || r.Validate() == nil {
				t.Fatalf("got=%+v/%v want zero contract refusal", got, err)
			}
			if !reflect.DeepEqual(r, before) {
				t.Fatal("mutated invalid input")
			}
		})
	}
}

func TestGroupPhaseBoundaryTriads(t *testing.T) {
	for index := 0; index < 24; index++ {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			r := phaseFixture(t)
			switch index {
			case 0:
				r.PhaseConstraints[0].Before = groupDuration(t, 1)
			case 1:
				r.PhaseConstraints[1].After = groupDuration(t, 1)
			case 2:
				r.Before = groupDuration(t, 1)
			case 3:
				r.After = groupDuration(t, 1)
			case 4:
				r.PhaseConstraints[0].Before = groupDuration(t, 7)
			case 5:
				r.PhaseConstraints[1].After = groupDuration(t, 7)
			case 6:
				r.PhaseConstraints[0].Before = groupDuration(t, 6)
			case 7:
				r.PhaseConstraints[1].After = groupDuration(t, 5)
			case 8:
				r.PhaseConstraints[0].Availability.Calendars[0].State = AvailabilityUnknown
			case 9:
				r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, false, AvailabilityUnknown)}
			case 10:
				r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityUnknown)}
				r.PhaseConstraints[0].Availability.Calendars[0].State = AvailabilityUnknown
			case 11:
				r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 8})}
				r.PhaseConstraints[0].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 8}).Availability
			case 12:
				r.PhaseConstraints[1].Ordinal = math.MaxUint64
			case 13:
				r.Duration = groupDuration(t, 8)
			case 14:
				r.Duration = groupDuration(t, 9)
			case 15:
				r.PhaseConstraints = r.PhaseConstraints[:1]
			case 16:
				r.PhaseConstraints[0].Duration = groupDuration(t, 4)
			case 17:
				slices.Reverse(r.PhaseConstraints)
			default:
				busy := [2]int64{int64(index - 18), int64(index - 17)}
				for i := range r.PhaseConstraints {
					r.PhaseConstraints[i].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, busy).Availability
				}
				if index == 23 {
					p := groupPerson(t, r.Window, 0, true, AvailabilityKnown, busy)
					groupAddCalendar(t, &p, busy)
					r.PhaseConstraints[0].Availability = p.Availability
				}
			}
			got, err := MatchGroup(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			phaseCompare(t, got, phaseBrute(t, r))
		})
	}
}

func TestGroupPhaseSignedExtremaAndCardinality(t *testing.T) {
	for _, origin := range []int64{math.MinInt64, math.MaxInt64 - 8} {
		t.Run(fmt.Sprint(origin), func(t *testing.T) {
			r := phaseFixture(t)
			r.Window.Start = temporal.InstantFromNanoseconds(origin)
			r.Window.End = temporal.InstantFromNanoseconds(origin + 8)
			for i := range r.PhaseConstraints {
				r.PhaseConstraints[i].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{origin + 1, origin + 3}).Availability
			}
			got, err := MatchGroup(t.Context(), r)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Windows) != 2 || groupNS(t, got.Windows[0].Starts.Earliest) != origin || groupNS(t, got.Windows[0].Starts.Latest) != origin || groupNS(t, got.Windows[1].Starts.Earliest) != origin+3 || groupNS(t, got.Windows[1].Starts.Latest) != origin+4 || got.FeasibleStartNanoseconds != 3 {
				t.Fatalf("extrema result=%+v", got)
			}
		})
	}
	r := phaseFixture(t)
	r.Window.End = temporal.InstantFromNanoseconds(math.MaxInt64)
	r.Duration = groupDuration(t, 1)
	r.PhaseConstraints = r.PhaseConstraints[:1]
	r.PhaseConstraints[0].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown).Availability
	got, err := MatchGroup(t.Context(), r)
	if err != nil || got.FeasibleStartNanoseconds != uint64(math.MaxInt64) || len(got.Windows) != 1 {
		t.Fatalf("max cardinality=%+v/%v", got, err)
	}
}

func TestGroupPhaseCancellationZeroResult(t *testing.T) {
	cause := errors.New("phase test cause")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, c := range []context.Context{nil, ctx, &groupTerminalContext{Context: t.Context(), threshold: 4, terminal: context.Canceled}, &groupTerminalContext{Context: t.Context(), threshold: 12, terminal: context.DeadlineExceeded}} {
		got, err := MatchGroup(c, phaseFixture(t))
		if err == nil || !reflect.DeepEqual(got, GroupMatchResult{}) {
			t.Fatalf("cancellation=%+v/%v", got, err)
		}
		if c == ctx && !errors.Is(err, cause) {
			t.Fatalf("lost cause %v", err)
		}
	}
}

func FuzzGroupPhaseSemanticOracle(f *testing.F) {
	for _, v := range []uint64{0, 1, 63, 255, 65535, 1 << 23, math.MaxUint64} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v uint64) {
		r := phaseFixture(t)
		r.Duration = groupDuration(t, int64(v%6)+1)
		r.Before = groupDuration(t, int64(v>>3&1))
		r.After = groupDuration(t, int64(v>>4&1))
		r.MaximumWindows = v >> 5 & 3
		for i := range r.PhaseConstraints {
			p := &r.PhaseConstraints[i]
			p.Offset = groupDuration(t, int64(v>>uint(8+i*3))%r.Duration.Nanoseconds())
			p.Duration = groupDuration(t, 1)
			p.Before = groupDuration(t, int64(v>>uint(14+i*2)&3))
			p.After = groupDuration(t, int64(v>>uint(18+i*2)&3))
			p.Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, phaseMask(uint8(v>>uint(24+i*6)))...).Availability
			if v>>uint(40+i)&1 != 0 {
				p.Availability.Calendars[0].State = AvailabilityUnknown
				p.Availability.Calendars[0].Busy = nil
			}
		}
		if v>>42&1 != 0 {
			r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, v>>43&1 != 0, AvailabilityKnown, phaseMask(uint8(v>>44))...)}
		}
		before := phaseClone(r)
		got, err := MatchGroup(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		phaseCompare(t, got, phaseBrute(t, r))
		if !reflect.DeepEqual(r, before) {
			t.Fatal("mutated source")
		}
		slices.Reverse(r.PhaseConstraints)
		again, err := MatchGroup(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		phaseCompare(t, again, got)
	})
}
