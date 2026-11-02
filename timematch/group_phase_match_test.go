package timematch

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	t.Parallel()
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
			left := start + p.Offset.Nanoseconds() - p.OffsetBefore.Nanoseconds() - p.Before.Nanoseconds()
			right := start + p.Offset.Nanoseconds() - p.OffsetBefore.Nanoseconds() + p.Duration.Nanoseconds() + p.After.Nanoseconds()
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
	t.Parallel()
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
		if !phaseSameRequest(r, before) {
			t.Fatalf("mask=%d mutated input", mask)
		}
	}
}
func phaseMask(mask uint8) [][2]int64 {
	var spans [][2]int64
	for i := int64(0); i < 6; i++ {
		if mask&(1<<i) != 0 {
			if len(spans) > 0 && spans[len(spans)-1][1] == i {
				spans[len(spans)-1][1] = i + 1
			} else {
				spans = append(spans, [2]int64{i, i + 1})
			}
		}
	}
	return spans
}

func TestGroupPhaseHostileContracts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*GroupMatchRequest)
	}{
		{"both signed offset arms", func(r *GroupMatchRequest) {
			r.PhaseConstraints[0].Offset = groupDuration(t, 1)
			r.PhaseConstraints[0].OffsetBefore = groupDuration(t, 1)
		}},
		{"duplicate ordinal", func(r *GroupMatchRequest) { r.PhaseConstraints[1].Ordinal = 0 }},
		{"window mismatch", func(r *GroupMatchRequest) {
			r.PhaseConstraints[0].Availability.Window.End = temporal.InstantFromNanoseconds(7)
		}},
		{"zero duration", func(r *GroupMatchRequest) { r.PhaseConstraints[0].Duration = temporal.Duration{} }},
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
			t.Parallel()
			r := phaseFixture(t)
			c.change(&r)
			before := phaseClone(r)
			got, err := MatchGroup(t.Context(), r)
			if !errors.Is(err, core.ErrTimeMatchContract) || !phaseResultZero(got) || r.Validate() == nil {
				t.Fatalf("got=%+v/%v want zero contract refusal", got, err)
			}
			if !phaseSameRequest(r, before) {
				t.Fatalf("request after rejection = %+v, want unchanged %+v", r, before)
			}
		})
	}
}

func TestGroupPhaseBoundaryTriads(t *testing.T) {
	t.Parallel()
	for index := 0; index < 24; index++ {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	for _, origin := range []int64{math.MinInt64, math.MaxInt64 - 8} {
		t.Run(fmt.Sprint(origin), func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	cause := errors.New("phase test cause")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, c := range []context.Context{nil, ctx, &groupTerminalContext{Context: t.Context(), threshold: 4, terminal: context.Canceled}, &groupTerminalContext{Context: t.Context(), threshold: 12, terminal: context.DeadlineExceeded}} {
		r := phaseFixture(t)
		r.PhaseConstraints[0].Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{2, 3}, [2]int64{5, 6}).Availability
		got, err := MatchGroup(c, r)
		if err == nil || !phaseResultZero(got) {
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
			p.Offset = groupDuration(t, int64(v>>uint(8+i*3)&7))
			if v>>uint(50+i)&1 != 0 {
				p.OffsetBefore = p.Offset
				p.Offset = temporal.Duration{}
			}
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
		if !phaseSameRequest(r, before) {
			t.Fatalf("request after matching = %+v, want unchanged %+v", r, before)
		}
		slices.Reverse(r.PhaseConstraints)
		again, err := MatchGroup(t.Context(), r)
		if err != nil {
			t.Fatal(err)
		}
		phaseCompare(t, again, got)
	})
}

// Extreme buffers are valid durations; fitting and overlap must be decided
// without overflowing start-shift subtraction or end-shift addition.
func TestGroupPhaseExtremeBuffers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                                   string
		offset, duration, before, after, service, globalBefore int64
		busy                                                   bool
		wantCount                                              uint64
		wantStart                                              int64
	}{
		{name: "max prefix exact one start", duration: 1, before: math.MaxInt64 - 1, service: 1, wantCount: 1, wantStart: math.MaxInt64 - 1},
		{name: "max suffix exact one start", duration: 1, after: math.MaxInt64 - 1, service: 1, wantCount: 1},
		{name: "max prefix overlap clips before subtraction", duration: 1, before: math.MaxInt64 - 1, service: 1, busy: true},
		{name: "offset and suffix exceed horizon without wrap", offset: 1, duration: 1, after: math.MaxInt64 - 1, service: 2},
		{name: "global prefix and phase prefix exact", duration: 1, before: math.MaxInt64 - 1, service: 1, globalBefore: 1, wantCount: 1, wantStart: math.MaxInt64 - 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(math.MaxInt64)}, Duration: groupDuration(t, c.service), Before: groupDuration(t, c.globalBefore)}
			a := groupPerson(t, r.Window, 0, true, AvailabilityKnown).Availability
			if c.busy {
				a = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 1}).Availability
			}
			r.PhaseConstraints = []GroupPhaseConstraint{{Availability: a, Offset: groupDuration(t, c.offset), Duration: groupDuration(t, c.duration), Before: groupDuration(t, c.before), After: groupDuration(t, c.after)}}
			got, err := MatchGroup(t.Context(), r)
			if err != nil || got.FeasibleStartNanoseconds != c.wantCount {
				t.Fatalf("match=%+v/%v want count%d", got, err, c.wantCount)
			}
			if c.wantCount == 1 && (len(got.Windows) != 1 || groupNS(t, got.Windows[0].Starts.Earliest) != c.wantStart || groupNS(t, got.Windows[0].Starts.Latest) != c.wantStart) {
				t.Fatalf("exact start=%+v want%d", got, c.wantStart)
			}
		})
	}
}

func phaseResultZero(r GroupMatchResult) bool {
	return groupResultZero(r) && r.UnknownPhaseConstraints == nil
}
func phaseSameRequest(a, b GroupMatchRequest) bool {
	if !groupSameRequest(a, b) {
		return false
	}
	return slices.EqualFunc(a.PhaseConstraints, b.PhaseConstraints, func(p, q GroupPhaseConstraint) bool {
		return p.Ordinal == q.Ordinal && p.Offset == q.Offset && p.OffsetBefore == q.OffsetBefore && p.Duration == q.Duration && p.Before == q.Before && p.After == q.After && p.Availability.Window == q.Availability.Window && slices.EqualFunc(p.Availability.Calendars, q.Availability.Calendars, func(c, d CalendarAvailability) bool {
			return c.ID == d.ID && c.State == d.State && slices.Equal(c.Busy, d.Busy)
		})
	})
}

func TestGroupPhaseOutsideVisitAnchors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                  string
		offset, before, duration, first, last int64
		busy                                  bool
	}{
		{name: "wholly before visit preserves exact range", before: 3, duration: 2, first: 3, last: 6},
		{name: "wholly after visit preserves exact range", offset: 4, duration: 1, first: 0, last: 3},
		{name: "before visit busy exclusion is translated", before: 3, duration: 2, first: 4, last: 6, busy: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := phaseFixture(t)
			r.Duration = groupDuration(t, 2)
			r.PhaseConstraints = r.PhaseConstraints[:1]
			p := &r.PhaseConstraints[0]
			p.Offset = groupDuration(t, c.offset)
			p.OffsetBefore = groupDuration(t, c.before)
			p.Duration = groupDuration(t, c.duration)
			if c.busy {
				p.Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 1}).Availability
			}
			got, err := MatchGroup(t.Context(), r)
			if err != nil || len(got.Windows) != 1 || groupNS(t, got.Windows[0].Starts.Earliest) != c.first || groupNS(t, got.Windows[0].Starts.Latest) != c.last {
				t.Fatalf("outside-visit match=%+v/%v want [%d,%d]", got, err, c.first, c.last)
			}
			phaseCompare(t, got, phaseBrute(t, r))
		})
	}
}

func TestGroupPhaseSignedOffsetsFiniteOracle(t *testing.T) {
	t.Parallel()
	for offset := int64(-5); offset <= 5; offset++ {
		for duration := int64(1); duration <= 3; duration++ {
			for before := int64(0); before < 2; before++ {
				for after := int64(0); after < 2; after++ {
					for mask := uint8(0); mask < 64; mask++ {
						r := phaseFixture(t)
						r.Duration = groupDuration(t, 2)
						r.PhaseConstraints = r.PhaseConstraints[:1]
						p := &r.PhaseConstraints[0]
						p.Offset = groupDuration(t, max(int64(0), offset))
						p.OffsetBefore = groupDuration(t, max(int64(0), -offset))
						p.Duration = groupDuration(t, duration)
						p.Before = groupDuration(t, before)
						p.After = groupDuration(t, after)
						p.Availability = groupPerson(t, r.Window, 0, true, AvailabilityKnown, phaseMask(mask)...).Availability
						got, err := MatchGroup(t.Context(), r)
						if err != nil {
							t.Fatalf("signed offset%d duration%d buffers%d/%d mask%d: %v", offset, duration, before, after, mask, err)
						}
						phaseCompare(t, got, phaseBrute(t, r))
					}
				}
			}
		}
	}
}

func TestGroupPhaseSignedOffsetExtrema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                       string
		offset, offsetBefore, before, globalBefore int64
		busyStart                                  int64
		wantCount                                  uint64
		wantStart                                  int64
	}{
		{name: "positive anchor beyond signed sum cannot fit", offset: math.MaxInt64, globalBefore: 1, busyStart: -1},
		{name: "minimum shift cannot fit any cursor", offsetBefore: math.MaxInt64, before: 1, busyStart: -1},
		{name: "underflowing prefix cannot fit", offsetBefore: math.MaxInt64, before: 2, busyStart: -1},
		{name: "negative extreme outside all cursors", offsetBefore: math.MaxInt64, busyStart: -1},
		{name: "negative extreme exact last cursor", offsetBefore: math.MaxInt64 - 1, busyStart: -1, wantCount: 1, wantStart: math.MaxInt64 - 1},
		{name: "negative extreme exact busy overlap", offsetBefore: math.MaxInt64 - 1, busyStart: 0},
		{name: "negative shift far busy skips overflowing expansion", offsetBefore: math.MaxInt64 - 1, busyStart: math.MaxInt64 - 1, wantCount: 1, wantStart: math.MaxInt64 - 1},
		{name: "positive extreme exact first cursor", offset: math.MaxInt64 - 1, busyStart: -1, wantCount: 1},
		{name: "positive shift early busy cannot overlap", offset: math.MaxInt64 - 1, busyStart: 0, wantCount: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(math.MaxInt64)}, Duration: groupDuration(t, 1), Before: groupDuration(t, c.globalBefore)}
			a := groupPerson(t, r.Window, 0, true, AvailabilityKnown).Availability
			if c.busyStart >= 0 {
				a = groupPerson(t, r.Window, 0, true, AvailabilityKnown, [2]int64{c.busyStart, c.busyStart + 1}).Availability
			}
			r.PhaseConstraints = []GroupPhaseConstraint{{Availability: a, Offset: groupDuration(t, c.offset), OffsetBefore: groupDuration(t, c.offsetBefore), Duration: groupDuration(t, 1), Before: groupDuration(t, c.before)}}
			got, err := MatchGroup(t.Context(), r)
			if err != nil || got.FeasibleStartNanoseconds != c.wantCount {
				t.Fatalf("signed extreme=%+v/%v want count%d", got, err, c.wantCount)
			}
			if c.wantCount == 1 && (len(got.Windows) != 1 || groupNS(t, got.Windows[0].Starts.Earliest) != c.wantStart || groupNS(t, got.Windows[0].Starts.Latest) != c.wantStart) {
				t.Fatalf("signed extreme exact=%+v want%d", got, c.wantStart)
			}
		})
	}
}
