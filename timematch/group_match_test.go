package timematch

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func groupDuration(tb testing.TB, nanos int64) temporal.Duration {
	tb.Helper()
	d, err := temporal.DurationFromNanoseconds(nanos)
	if err != nil {
		tb.Fatalf("Duration(%d) error=%v, want nil", nanos, err)
	}
	return d
}
func groupNS(tb testing.TB, instant temporal.Instant) int64 {
	tb.Helper()
	n, err := instant.Nanoseconds()
	if err != nil {
		tb.Fatalf("Instant.Nanoseconds() error=%v, want nil", err)
	}
	return n
}
func groupFixture(tb testing.TB) GroupMatchRequest {
	tb.Helper()
	r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(16)}, Duration: groupDuration(tb, 3)}
	r.Participants = []GroupParticipant{groupPerson(tb, r.Window, 0, true, AvailabilityKnown)}
	return r
}
func groupPerson(tb testing.TB, window temporal.IntervalBounds, ordinal uint64, required bool, state AvailabilityState, spans ...[2]int64) GroupParticipant {
	tb.Helper()
	id, err := ParseCalendarID("primary")
	if err != nil {
		tb.Fatal(err)
	}
	calendar := CalendarAvailability{ID: id, State: state}
	for _, span := range spans {
		calendar.Busy = append(calendar.Busy, temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(span[0]), End: temporal.InstantFromNanoseconds(span[1])})
	}
	return GroupParticipant{Ordinal: ordinal, Required: required, Availability: Availability{Window: window, Calendars: []CalendarAvailability{calendar}}}
}
func groupAddCalendar(tb testing.TB, p *GroupParticipant, spans ...[2]int64) {
	tb.Helper()
	id, err := ParseCalendarID("second")
	if err != nil {
		tb.Fatal(err)
	}
	c := CalendarAvailability{ID: id, State: AvailabilityKnown}
	for _, span := range spans {
		c.Busy = append(c.Busy, temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(span[0]), End: temporal.InstantFromNanoseconds(span[1])})
	}
	p.Availability.Calendars = append(p.Availability.Calendars, c)
}

// This oracle scans actual nanosecond starts and directly tests occupied
// interval intersection. It does not expand busy intervals or use the sweep.
func groupBrute(tb testing.TB, r GroupMatchRequest) GroupMatchResult {
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
	horizon := groupNS(tb, r.Window.End) - groupNS(tb, r.Window.Start)
	if r.Before.Nanoseconds()+r.Duration.Nanoseconds()+r.After.Nanoseconds() > horizon {
		return out
	}
	first := groupNS(tb, r.Window.Start) + r.Before.Nanoseconds()
	last := groupNS(tb, r.Window.End) - r.Duration.Nanoseconds() - r.After.Nanoseconds()
	for start := first; start <= last; start++ {
		left, right := start-r.Before.Nanoseconds(), start+r.Duration.Nanoseconds()+r.After.Nanoseconds()
		blocked := []int{}
		available := uint64(0)
		for i, p := range people {
			free := true
			for _, c := range p.Availability.Calendars {
				if c.State == AvailabilityUnknown {
					free = false
				}
				for _, busy := range c.Busy {
					if left < groupNS(tb, busy.End) && right > groupNS(tb, busy.Start) {
						free = false
					}
				}
			}
			if p.Required && !free {
				blocked = append(blocked, i)
			}
			if !p.Required && free {
				available++
			}
		}
		if len(blocked) == 0 {
			out.FeasibleStartNanoseconds++
			n := len(out.Windows)
			if n > 0 && out.Windows[n-1].OptionalAvailable == available && groupNS(tb, out.Windows[n-1].Starts.Latest) == start-1 {
				out.Windows[n-1].Starts.Latest = temporal.InstantFromNanoseconds(start)
			} else {
				out.Windows = append(out.Windows, GroupWindow{Starts: GroupStartRange{Earliest: temporal.InstantFromNanoseconds(start), Latest: temporal.InstantFromNanoseconds(start)}, OptionalAvailable: available})
			}
		}
		if len(blocked) == 1 {
			i := blocked[0]
			if out.RemovalImpacts[i].AdditionalStartNanoseconds == 0 {
				out.RemovalImpacts[i].EarliestUnlocked = temporal.InstantFromNanoseconds(start)
			}
			out.RemovalImpacts[i].AdditionalStartNanoseconds++
		}
	}
	slices.SortFunc(out.Windows, func(a, b GroupWindow) int {
		if a.OptionalAvailable > b.OptionalAvailable {
			return -1
		}
		if a.OptionalAvailable < b.OptionalAvailable {
			return 1
		}
		an, bn := groupNS(tb, a.Starts.Earliest), groupNS(tb, b.Starts.Earliest)
		if an < bn {
			return -1
		}
		if an > bn {
			return 1
		}
		return 0
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
func groupCompareResult(t *testing.T, got, want GroupMatchResult) {
	t.Helper()
	if !slices.Equal(got.Windows, want.Windows) || !slices.Equal(got.RemovalImpacts, want.RemovalImpacts) || !slices.Equal(got.UnknownRequired, want.UnknownRequired) || !slices.Equal(got.UnknownOptional, want.UnknownOptional) || got.RequiredCount != want.RequiredCount || got.KnownOptionalCount != want.KnownOptionalCount || got.FeasibleStartNanoseconds != want.FeasibleStartNanoseconds || got.Truncated != want.Truncated {
		t.Fatalf("group result=%+v, want exact brute oracle %+v", got, want)
	}
}

func TestGroupMatchHostileLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		change  func(testing.TB, *GroupMatchRequest)
		wantErr error
	}{
		{name: "free required horizon"},
		{name: "required busy prefix", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 3})
		}},
		{name: "required busy suffix exact boundary", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{13, 16})
		}},
		{name: "required busy middle leaves two exact fits", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{3, 13})
		}},
		{name: "optional affects score not hard feasibility", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, false, AvailabilityKnown, [2]int64{6, 9}))
		}},
		{name: "later best window survives limit one", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.MaximumWindows = 1
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, false, AvailabilityKnown, [2]int64{0, 12}))
		}},
		{name: "overlapping constituent calendars count one participant", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{3, 7})
			groupAddCalendar(tb, &r.Participants[0], [2]int64{5, 9})
		}},
		{name: "expanded nearby busy spans merge", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{2, 3}, [2]int64{4, 5})
		}},
		{name: "two simultaneous blockers have no individual gain", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{4, 10})
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, true, AvailabilityKnown, [2]int64{4, 10}))
		}},
		{name: "unknown optional is explicitly separate", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, false, AvailabilityUnknown))
		}},
		{name: "one unknown required removal unlocks knowledge only", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityUnknown)
		}},
		{name: "two unknown required have no single removal gain", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityUnknown)
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, true, AvailabilityUnknown))
		}},
		{name: "any unknown calendar blocks participant", change: func(tb testing.TB, r *GroupMatchRequest) {
			groupAddCalendar(tb, &r.Participants[0])
			r.Participants[0].Availability.Calendars[1].State = AvailabilityUnknown
		}},
		{name: "optional membership swap keeps one score plateau", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Duration = groupDuration(tb, 1)
			r.Participants = append(r.Participants, groupPerson(tb, r.Window, 1, false, AvailabilityKnown, [2]int64{0, 8}), groupPerson(tb, r.Window, 2, false, AvailabilityKnown, [2]int64{8, 16}))
		}},
		{name: "before buffer alone", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Before = groupDuration(tb, 1)
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{4, 5})
		}},
		{name: "after buffer alone", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.After = groupDuration(tb, 1)
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{4, 5})
		}},
		{name: "both buffers exact occupied fit", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Duration = groupDuration(tb, 14)
			r.Before = groupDuration(tb, 1)
			r.After = groupDuration(tb, 1)
		}},
		{name: "smallest positive duration", change: func(tb testing.TB, r *GroupMatchRequest) { r.Duration = groupDuration(tb, 1) }},
		{name: "duration one below horizon", change: func(tb testing.TB, r *GroupMatchRequest) { r.Duration = groupDuration(tb, 15) }},
		{name: "duration exactly horizon singleton", change: func(tb testing.TB, r *GroupMatchRequest) { r.Duration = groupDuration(tb, 16) }},
		{name: "duration one above horizon neutral", change: func(tb testing.TB, r *GroupMatchRequest) { r.Duration = groupDuration(tb, 17) }},
		{name: "free gap one below appointment", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 5}, [2]int64{7, 16})
		}},
		{name: "free gap exact appointment", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 5}, [2]int64{8, 16})
		}},
		{name: "free gap one above appointment", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 5}, [2]int64{9, 16})
		}},
		{name: "busy singleton excludes one start at duration one", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Duration = groupDuration(tb, 1)
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{8, 9})
		}},
		{name: "all occupied by sole required participant", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{0, 16})
		}},
		{name: "ordinal maximum has same semantics", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants[0].Ordinal = math.MaxUint64 }},
		{name: "unknown future availability rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants[0].Availability.Calendars[0].State = 255 }, wantErr: core.ErrTimeMatchContract},
		{name: "missing participants rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants = nil }, wantErr: core.ErrTimeMatchContract},
		{name: "no required participant rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants[0].Required = false }, wantErr: core.ErrTimeMatchContract},
		{name: "duplicate ordinal rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants = append(r.Participants, r.Participants[0]) }, wantErr: core.ErrTimeMatchContract},
		{name: "zero duration rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Duration = temporal.Duration{} }, wantErr: core.ErrTimeMatchContract},
		{name: "foreign availability window rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0].Availability.Window.End = temporal.InstantFromNanoseconds(15)
		}, wantErr: core.ErrTimeMatchContract},
		{name: "unknown with busy facts rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityUnknown, [2]int64{1, 2})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "touching unmerged busy rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{1, 2}, [2]int64{2, 3})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "overlapping unmerged busy rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{1, 3}, [2]int64{2, 4})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "reversed busy order rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{8, 9}, [2]int64{1, 2})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "busy below window rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{-1, 2})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "busy past window rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0] = groupPerson(tb, r.Window, 0, true, AvailabilityKnown, [2]int64{15, 17})
		}, wantErr: core.ErrTimeMatchContract},
		{name: "missing calendars rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Participants[0].Availability.Calendars = nil }, wantErr: core.ErrTimeMatchContract},
		{name: "duplicate calendar rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Participants[0].Availability.Calendars = append(r.Participants[0].Availability.Calendars, r.Participants[0].Availability.Calendars[0])
		}, wantErr: core.ErrTimeMatchContract},
		{name: "unset window start rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Window.Start = temporal.Instant{} }, wantErr: core.ErrTimeMatchContract},
		{name: "zero horizon rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Window.End = r.Window.Start }, wantErr: core.ErrTimeMatchContract},
		{name: "reversed horizon rejected", change: func(tb testing.TB, r *GroupMatchRequest) { r.Window.End = temporal.InstantFromNanoseconds(-1) }, wantErr: core.ErrTimeMatchContract},
		{name: "span addition overflow rejected", change: func(tb testing.TB, r *GroupMatchRequest) {
			r.Duration = groupDuration(tb, math.MaxInt64)
			r.Before = groupDuration(tb, 1)
		}, wantErr: core.ErrTimeMatchContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := groupFixture(t)
			if tc.change != nil {
				tc.change(t, &r)
			}
			got, err := MatchGroup(t.Context(), r)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || !groupResultZero(got) {
					t.Fatalf("MatchGroup()=(%+v,%v), want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MatchGroup() error=%v, want nil", err)
			}
			groupCompareResult(t, got, groupBrute(t, r))
		})
	}
}

func groupResultZero(r GroupMatchResult) bool {
	return r.Windows == nil && r.RemovalImpacts == nil && r.UnknownRequired == nil && r.UnknownOptional == nil && r.RequiredCount == 0 && r.KnownOptionalCount == 0 && r.FeasibleStartNanoseconds == 0 && !r.Truncated
}

func groupBusyMask(mask uint8) [][2]int64 {
	var spans [][2]int64
	for i := 0; i < 4; {
		if mask&(1<<i) == 0 {
			i++
			continue
		}
		start := i
		for i < 4 && mask&(1<<i) != 0 {
			i++
		}
		spans = append(spans, [2]int64{int64(start), int64(i)})
	}
	return spans
}
func TestGroupMatchExhaustiveSmallHorizonOracle(t *testing.T) {
	t.Parallel()
	for first := uint8(0); first < 16; first++ {
		for second := uint8(0); second < 16; second++ {
			for optional := uint8(0); optional < 16; optional++ {
				for duration := int64(1); duration <= 4; duration++ {
					r := groupFixture(t)
					r.Window.End = temporal.InstantFromNanoseconds(4)
					r.Duration = groupDuration(t, duration)
					r.Participants = []GroupParticipant{groupPerson(t, r.Window, 7, true, AvailabilityKnown, groupBusyMask(first)...), groupPerson(t, r.Window, 3, true, AvailabilityKnown, groupBusyMask(second)...), groupPerson(t, r.Window, 11, false, AvailabilityKnown, groupBusyMask(optional)...)}
					r.MaximumWindows = uint64(optional % 3)
					got, err := MatchGroup(t.Context(), r)
					if err != nil {
						t.Fatalf("masks=%d/%d/%d duration%d error=%v, want nil", first, second, optional, duration, err)
					}
					groupCompareResult(t, got, groupBrute(t, r))
				}
			}
		}
	}
}

func TestGroupMatchMarginalImpactIsNotBusyness(t *testing.T) {
	t.Parallel()
	r := groupFixture(t)
	r.Duration = groupDuration(t, 1)
	r.Participants = []GroupParticipant{groupPerson(t, r.Window, 3, true, AvailabilityKnown, [2]int64{0, 10}), groupPerson(t, r.Window, 2, true, AvailabilityKnown, [2]int64{0, 10}), groupPerson(t, r.Window, 1, true, AvailabilityKnown, [2]int64{11, 12})}
	got, err := MatchGroup(t.Context(), r)
	if err != nil || len(got.RemovalImpacts) != 3 {
		t.Fatalf("impact result=%+v/%v, want3 impacts/nil", got, err)
	}
	if got.RemovalImpacts[0].Ordinal != 1 || got.RemovalImpacts[0].AdditionalStartNanoseconds != 1 || groupNS(t, got.RemovalImpacts[0].EarliestUnlocked) != 11 || got.RemovalImpacts[1].AdditionalStartNanoseconds != 0 {
		t.Fatalf("impacts=%+v, want short sole-blocker ordinal1 first; two busier shared blockers gain0", got.RemovalImpacts)
	}
}

func TestGroupMatchOrderImmutabilityAndWholeHorizonLimit(t *testing.T) {
	t.Parallel()
	r := groupFixture(t)
	r.Duration = groupDuration(t, 1)
	r.Participants = append(r.Participants, groupPerson(t, r.Window, 9, false, AvailabilityKnown, [2]int64{0, 12}), groupPerson(t, r.Window, 3, false, AvailabilityKnown, [2]int64{0, 4}, [2]int64{6, 8}))
	groupAddCalendar(t, &r.Participants[1], [2]int64{2, 5})
	before := groupCloneRequest(r)
	all, err := MatchGroup(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	r.MaximumWindows = 1
	limited, err := MatchGroup(t.Context(), r)
	if err != nil || len(limited.Windows) != 1 || limited.Windows[0] != all.Windows[0] || !limited.Truncated || limited.FeasibleStartNanoseconds != all.FeasibleStartNanoseconds || !slices.Equal(limited.RemovalImpacts, all.RemovalImpacts) {
		t.Fatalf("limit=%+v/%v, want full horizon best %+v and exact impacts", limited, err, all)
	}
	r.MaximumWindows = 0
	if !groupSameRequest(r, before) {
		t.Fatalf("request after=%+v, want exact unchanged typed snapshot %+v", r, before)
	}
	slices.Reverse(r.Participants)
	for i := range r.Participants {
		r.Participants[i].Availability.Calendars = slices.Clone(r.Participants[i].Availability.Calendars)
		slices.Reverse(r.Participants[i].Availability.Calendars)
	}
	permuted, err := MatchGroup(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	groupCompareResult(t, permuted, all)
}

func TestGroupMatchTemporalExtrema(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                string
		start, end, duration, before, after int64
	}{
		{"minimum narrow exact fit", math.MinInt64, math.MinInt64 + 4, 2, 1, 1}, {"maximum narrow exact fit", math.MaxInt64 - 4, math.MaxInt64, 2, 1, 1}, {"negative to positive epoch", -2, 2, 2, 1, 1}, {"maximum representable duration", math.MinInt64, -1, math.MaxInt64, 0, 0}, {"one below maximum with buffers", math.MinInt64, -1, math.MaxInt64 - 2, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := groupFixture(t)
			r.Window = temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(tc.start), End: temporal.InstantFromNanoseconds(tc.end)}
			r.Duration = groupDuration(t, tc.duration)
			r.Before = groupDuration(t, tc.before)
			r.After = groupDuration(t, tc.after)
			r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown)}
			got, err := MatchGroup(t.Context(), r)
			want := temporal.InstantFromNanoseconds(tc.start + tc.before)
			if err != nil || len(got.Windows) != 1 || got.Windows[0].Starts.Earliest != want || got.Windows[0].Starts.Latest != want || got.FeasibleStartNanoseconds != 1 {
				t.Fatalf("extreme result=%+v/%v, want singleton%v", got, err, want)
			}
		})
	}
}

type groupTerminalContext struct {
	context.Context
	calls     atomic.Int64
	threshold int64
	terminal  error
}

func (c *groupTerminalContext) Err() error {
	if c.calls.Add(1) >= c.threshold {
		return c.terminal
	}
	return nil
}
func TestGroupMatchCancellationPreservesCauseAndNoPartialResult(t *testing.T) {
	t.Parallel()
	r := groupFixture(t)
	cause := core.ErrTimeMatchConflict
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		want  error
		cause error
	}{
		{"nil context", nil, core.ErrNilContext, nil}, {"cancelled before validation", cancelled, context.Canceled, cause}, {"deadline during validation", &groupTerminalContext{Context: t.Context(), threshold: 2, terminal: context.DeadlineExceeded}, context.DeadlineExceeded, nil}, {"cancelled after materialization", &groupTerminalContext{Context: t.Context(), threshold: 7, terminal: context.Canceled}, context.Canceled, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := MatchGroup(tc.ctx, r)
			if !errors.Is(err, core.ErrTimeMatchContract) || !errors.Is(err, tc.want) || tc.cause != nil && !errors.Is(err, tc.cause) || !groupResultZero(got) {
				t.Fatalf("cancel result=%+v/%v, want zero and contract/%v/cause%v", got, err, tc.want, tc.cause)
			}
		})
	}
}

func TestGroupMatchTwentyAndOneHundredTwentyAcrossEighteenMonths(t *testing.T) {
	t.Parallel()
	for _, count := range []int{20, 120} {
		for _, minutes := range []uint64{15, 180} {
			t.Run(fmt.Sprintf("participants%d_duration%dminutes", count, minutes), func(t *testing.T) {
				t.Parallel()
				start, err := temporal.ParseRFC3339("2026-01-01T00:00:00Z")
				if err != nil {
					t.Fatal(err)
				}
				end, err := temporal.ParseRFC3339("2027-07-01T00:00:00Z")
				if err != nil {
					t.Fatal(err)
				}
				duration, err := temporal.DurationFromMinutes(minutes)
				if err != nil {
					t.Fatal(err)
				}
				r := GroupMatchRequest{Window: temporal.IntervalBounds{Start: start, End: end}, Duration: duration, MaximumWindows: 3}
				oneHour, err := temporal.DurationFromHours(1)
				if err != nil {
					t.Fatal(err)
				}
				busyEnd, err := start.Add(oneHour)
				if err != nil {
					t.Fatal(err)
				}
				for person := 0; person < count; person++ {
					p := groupPerson(t, r.Window, uint64(person), person%2 == 0, AvailabilityKnown)
					p.Availability.Calendars[0].Busy = []temporal.IntervalBounds{{Start: start, End: busyEnd}}
					r.Participants = append(r.Participants, p)
				}
				got, err := MatchGroup(t.Context(), r)
				latest, latestErr := end.Subtract(duration)
				if err != nil || latestErr != nil || len(got.Windows) != 1 || got.Windows[0].Starts.Earliest != busyEnd || got.Windows[0].Starts.Latest != latest || got.Windows[0].OptionalAvailable != uint64(count/2) || got.Truncated {
					t.Fatalf("18month result=%+v/%v latestErr%v, want one complete horizon range and%d optional people", got, err, latestErr, count/2)
				}
				for _, impact := range got.RemovalImpacts {
					if impact.AdditionalStartNanoseconds != 0 {
						t.Fatalf("shared busy impact=%+v, want zero single-person gain", impact)
					}
				}
			})
		}
	}
}

// MatchGroup is a pure typed entry, not a calendar JSON transport. Scalar
// selectors generate bounded valid/invalid typed availability and reach the
// real public matcher; every admitted state is compared with direct overlap.
func FuzzGroupMatchSmallHorizonSemanticOracle(f *testing.F) {
	seed := groupFixture(f)
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), uint8(0), uint8(0), uint8(seed.Duration.Nanoseconds()), uint8(0))
	f.Add(uint8(5), uint8(10), uint8(3), uint8(1), uint8(0))
	f.Add(uint8(0), uint8(0), uint8(0), uint8(4), uint8(1))
	f.Add(uint8(19), uint8(6), uint8(12), uint8(9), uint8(0))
	f.Add(uint8(0), uint8(0), uint8(0), uint8(0), uint8(128))
	f.Fuzz(func(t *testing.T, a, b, c, span, flags uint8) {
		r := groupFixture(t)
		r.Window.End = temporal.InstantFromNanoseconds(4)
		r.Duration = groupDuration(t, int64(span&7))
		r.Before = groupDuration(t, int64(span>>3&3))
		r.After = groupDuration(t, int64(span>>5&3))
		r.MaximumWindows = uint64(a >> 5)
		r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown, groupBusyMask(a&15)...), groupPerson(t, r.Window, 1, true, AvailabilityKnown, groupBusyMask(b&15)...), groupPerson(t, r.Window, 2, c&16 != 0, AvailabilityKnown, groupBusyMask(c&15)...)}
		if a&16 != 0 {
			groupAddCalendar(t, &r.Participants[0], groupBusyMask(b&15)...)
		}
		for i := range r.Participants {
			for j := range r.Participants[i].Availability.Calendars {
				calendar := &r.Participants[i].Availability.Calendars[j]
				switch flags >> (i * 2) & 3 {
				case 0:
				case 1:
					calendar.State = AvailabilityUnknown
					calendar.Busy = nil
				case 2:
					calendar.State = AvailabilityInvalid
				case 3:
					calendar.State = AvailabilityState(255)
				}
			}
		}
		if flags&64 != 0 {
			r.Participants[1].Ordinal = r.Participants[0].Ordinal
		}
		if flags&128 != 0 {
			for i := range r.Participants {
				r.Participants[i].Required = false
			}
		}
		before := groupCloneRequest(r)
		validation := r.Validate()
		got, err := MatchGroup(t.Context(), r)
		if !groupSameRequest(r, before) {
			t.Fatalf("mutated input=%+v, want unchanged %+v", r, before)
		}
		if validation != nil {
			if !errors.Is(err, core.ErrTimeMatchContract) || !groupResultZero(got) {
				t.Fatalf("rejected match=%+v/%v, want zero and typed contract", got, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("validated match error=%v, want nil", err)
		}
		groupCompareResult(t, got, groupBrute(t, r))
		slices.Reverse(r.Participants)
		reordered, reorderErr := MatchGroup(t.Context(), r)
		if reorderErr != nil {
			t.Fatal(reorderErr)
		}
		groupCompareResult(t, reordered, got)
	})
}

func groupCloneRequest(r GroupMatchRequest) GroupMatchRequest {
	r.Participants = slices.Clone(r.Participants)
	for i := range r.Participants {
		a := &r.Participants[i].Availability
		a.Calendars = slices.Clone(a.Calendars)
		for j := range a.Calendars {
			a.Calendars[j].Busy = slices.Clone(a.Calendars[j].Busy)
		}
	}
	return r
}
func groupSameRequest(a, b GroupMatchRequest) bool {
	if a.Window != b.Window || a.Duration != b.Duration || a.Before != b.Before || a.After != b.After || a.MaximumWindows != b.MaximumWindows || len(a.Participants) != len(b.Participants) {
		return false
	}
	for i, p := range a.Participants {
		q := b.Participants[i]
		if p.Ordinal != q.Ordinal || p.Required != q.Required || p.Availability.Window != q.Availability.Window || len(p.Availability.Calendars) != len(q.Availability.Calendars) {
			return false
		}
		for j, c := range p.Availability.Calendars {
			d := q.Availability.Calendars[j]
			if c.ID != d.ID || c.State != d.State || !slices.Equal(c.Busy, d.Busy) {
				return false
			}
		}
	}
	return true
}

func TestGroupMatchMaximumStartCardinality(t *testing.T) {
	t.Parallel()
	r := groupFixture(t)
	r.Window = temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(math.MinInt64), End: temporal.InstantFromNanoseconds(-1)}
	r.Duration = groupDuration(t, 1)
	r.Participants = []GroupParticipant{groupPerson(t, r.Window, 0, true, AvailabilityKnown)}
	got, err := MatchGroup(t.Context(), r)
	if err != nil || len(got.Windows) != 1 || got.Windows[0].Starts.Earliest != r.Window.Start || groupNS(t, got.Windows[0].Starts.Latest) != -2 || got.FeasibleStartNanoseconds != uint64(math.MaxInt64) {
		t.Fatalf("maximum count=%+v/%v, want MaxInt64 starts from MinInt64 through -2 without overflow", got, err)
	}
}
