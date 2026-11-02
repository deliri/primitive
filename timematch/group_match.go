package timematch

import (
	"context"
	"errors"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// GroupParticipant uses a caller-owned ordinal, not calendar names or events.
// Required is explicit policy. Any unknown constituent calendar makes this
// participant unknown; overlapping constituent calendars count only once.
type GroupParticipant struct {
	Ordinal      uint64
	Required     bool
	Availability Availability
}

// GroupMatchRequest finds appointment starts whose complete occupied interval
// [start-Before,start+Duration+After) fits Window. Duration is positive.
// All participants cover exactly Window. At least one participant is required.
// MaximumWindows selects the best windows across the entire horizon. Zero
// returns every actual boundary window, with caller-owned output allocation.
type GroupMatchRequest struct {
	Window         temporal.IntervalBounds
	Duration       temporal.Duration
	Before         temporal.Duration
	After          temporal.Duration
	Participants   []GroupParticipant
	MaximumWindows uint64
}

// GroupStartRange includes BOTH endpoints at nanosecond precision. An exact
// fit is a singleton with Earliest == Latest, not an empty half-open interval.
type GroupStartRange struct {
	Earliest temporal.Instant
	Latest   temporal.Instant
}

func (r GroupStartRange) Validate() error {
	_, err := r.Latest.Since(r.Earliest)
	if err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// GroupWindow is a maximal continuous start range with constant attendance
// score. Every required participant is known and free throughout every occupied
// appointment in it. OptionalAvailable counts people, never their calendars.
// Known attendance is result.RequiredCount + OptionalAvailable.
type GroupWindow struct {
	Starts            GroupStartRange
	OptionalAvailable uint64
}

// GroupRemovalImpact is the gain from relaxing only this participant's required
// constraint. AdditionalStartNanoseconds counts discrete nanosecond-valued start
// positions, NOT elapsed duration or appointments. Optional participants gain 0.
// Unknown distinguishes unavailable knowledge from a known calendar conflict.
// EarliestUnlocked is unset exactly when the gain is zero.
type GroupRemovalImpact struct {
	Ordinal                    uint64
	Required                   bool
	Unknown                    bool
	AdditionalStartNanoseconds uint64
	EarliestUnlocked           temporal.Instant
}

// GroupMatchResult contains no calendar IDs, appointments or participant names.
// Windows rank by OptionalAvailable descending, then Earliest ascending.
// RemovalImpacts rank by gain descending, then ordinal ascending. Unknown lists
// are ordinal-sorted. Counts and impacts cover the FULL horizon even if the
// caller limits Windows. Truncated means actual score windows were omitted.
type GroupMatchResult struct {
	Windows                  []GroupWindow
	RemovalImpacts           []GroupRemovalImpact
	UnknownRequired          []uint64
	UnknownOptional          []uint64
	RequiredCount            uint64
	KnownOptionalCount       uint64
	FeasibleStartNanoseconds uint64
	Truncated                bool
}

func (r GroupMatchRequest) Validate() error {
	_, _, _, err := groupInputs(nil, r)
	return err
}

func groupInputs(ctx context.Context, r GroupMatchRequest) ([]GroupParticipant, int64, int64, error) {
	var contextErr error
	if ctx != nil {
		contextErr = groupContext(ctx)
	}
	if err := errors.Join(contextErr, positiveInterval(r.Window), r.Duration.Validate(), r.Before.Validate(), r.After.Validate()); err != nil {
		return nil, 0, 0, errors.Join(core.ErrTimeMatchContract, err)
	}
	if r.Duration.IsZero() || len(r.Participants) == 0 {
		return nil, 0, 0, core.ErrTimeMatchContract
	}
	span, err := r.Before.Add(r.Duration)
	if err == nil {
		span, err = span.Add(r.After)
	}
	if err != nil {
		return nil, 0, 0, errors.Join(core.ErrTimeMatchContract, err)
	}
	horizon, err := r.Window.End.Since(r.Window.Start)
	if err != nil {
		return nil, 0, 0, errors.Join(core.ErrTimeMatchContract, err)
	}
	people := slices.Clone(r.Participants)
	slices.SortFunc(people, func(a, b GroupParticipant) int { return groupCompareOrdinal(a.Ordinal, b.Ordinal) })
	required := false
	for i, p := range people {
		if ctx != nil {
			if err := groupContext(ctx); err != nil {
				return nil, 0, 0, errors.Join(core.ErrTimeMatchContract, err)
			}
		}
		if i > 0 && p.Ordinal == people[i-1].Ordinal || p.Availability.Window != r.Window {
			return nil, 0, 0, core.ErrTimeMatchContract
		}
		if err := p.Availability.Validate(); err != nil {
			return nil, 0, 0, errors.Join(core.ErrTimeMatchContract, err)
		}
		required = required || p.Required
	}
	if !required {
		return nil, 0, 0, core.ErrTimeMatchContract
	}
	return people, span.Nanoseconds(), horizon.Nanoseconds(), nil
}

// MatchGroup is an observation-only interval-boundary sweep. It never contacts
// or mutates calendars. Working allocation follows participant and busy-boundary
// input, not the number of minutes in Window; selected output is bounded by a
// positive MaximumWindows. Inputs and nested busy slices are never modified.
// Every error, including cancellation, returns an entirely zero result.
func MatchGroup(ctx context.Context, r GroupMatchRequest) (GroupMatchResult, error) {
	if err := groupContext(ctx); err != nil {
		return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	people, span, horizon, err := groupInputs(ctx, r)
	if err != nil {
		return GroupMatchResult{}, err
	}
	result := GroupMatchResult{RemovalImpacts: make([]GroupRemovalImpact, len(people))}
	var boundaries []groupBoundary
	limit := int64(0)
	if span <= horizon {
		limit = horizon - span + 1
	}
	for i, person := range people {
		if err := groupContext(ctx); err != nil {
			return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		unknown := false
		for _, calendar := range person.Availability.Calendars {
			unknown = unknown || calendar.State == AvailabilityUnknown
		}
		result.RemovalImpacts[i] = GroupRemovalImpact{Ordinal: person.Ordinal, Required: person.Required, Unknown: unknown}
		if person.Required {
			result.RequiredCount++
		}
		if unknown {
			if person.Required {
				result.UnknownRequired = append(result.UnknownRequired, person.Ordinal)
				if limit < 0 {
					boundaries = append(boundaries, groupBoundary{at: 0, person: i, begin: true}, groupBoundary{at: limit, person: i})
				}
			} else {
				result.UnknownOptional = append(result.UnknownOptional, person.Ordinal)
			}
			continue
		}
		if !person.Required {
			result.KnownOptionalCount++
		}
		if limit == 0 {
			continue
		}
		blocked, err := groupForbidden(ctx, person.Availability, r.Window.Start, span, limit)
		if err != nil {
			return GroupMatchResult{}, err
		}
		for _, b := range blocked {
			boundaries = append(boundaries, groupBoundary{at: b.start, person: i, begin: true}, groupBoundary{at: b.end, person: i})
		}
	}
	if limit == 0 {
		if err := groupContext(ctx); err != nil {
			return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		return result, nil
	}
	slices.SortFunc(boundaries, func(a, b groupBoundary) int {
		if a.at < b.at {
			return -1
		}
		if a.at > b.at {
			return 1
		}
		return groupCompareOrdinal(uint64(a.person), uint64(b.person))
	})
	var selected []groupCandidate
	var pending groupCandidate
	pendingSet := false
	var windowCount uint64
	flush := func() {
		if !pendingSet {
			return
		}
		windowCount++
		selected = groupKeepWindow(selected, pending, r.MaximumWindows)
		pendingSet = false
	}
	requiredBlocked, optionalBlocked := 0, 0
	var requiredXOR uint64
	cursor, event := int64(0), 0
	for cursor < limit {
		if err := groupContext(ctx); err != nil {
			return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		for event < len(boundaries) && boundaries[event].at == cursor {
			b := boundaries[event]
			change := -1
			if b.begin {
				change = 1
			}
			if people[b.person].Required {
				requiredBlocked += change
				requiredXOR ^= uint64(b.person) + 1
			} else {
				optionalBlocked += change
			}
			event++
		}
		next := limit
		if event < len(boundaries) {
			next = boundaries[event].at
		}
		if requiredBlocked == 0 {
			result.FeasibleStartNanoseconds += uint64(next - cursor)
			available := result.KnownOptionalCount - uint64(optionalBlocked)
			if pendingSet && pending.end == cursor && pending.optional == available {
				pending.end = next
			} else {
				flush()
				pending = groupCandidate{start: cursor, end: next, optional: available}
				pendingSet = true
			}
		} else {
			flush()
			if requiredBlocked == 1 {
				impact := &result.RemovalImpacts[requiredXOR-1]
				if impact.AdditionalStartNanoseconds == 0 {
					impact.EarliestUnlocked, err = groupStart(r, cursor)
					if err != nil {
						return GroupMatchResult{}, err
					}
				}
				impact.AdditionalStartNanoseconds += uint64(next - cursor)
			}
		}
		cursor = next
	}
	flush()
	slices.SortFunc(selected, groupCandidateOrder)
	result.Windows = make([]GroupWindow, len(selected))
	for i, candidate := range selected {
		if err := groupContext(ctx); err != nil {
			return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		first, firstErr := groupStart(r, candidate.start)
		last, lastErr := groupStart(r, candidate.end-1)
		if err := errors.Join(firstErr, lastErr); err != nil {
			return GroupMatchResult{}, err
		}
		result.Windows[i] = GroupWindow{Starts: GroupStartRange{Earliest: first, Latest: last}, OptionalAvailable: candidate.optional}
	}
	result.Truncated = r.MaximumWindows > 0 && windowCount > r.MaximumWindows
	slices.SortFunc(result.RemovalImpacts, func(a, b GroupRemovalImpact) int {
		if a.AdditionalStartNanoseconds > b.AdditionalStartNanoseconds {
			return -1
		}
		if a.AdditionalStartNanoseconds < b.AdditionalStartNanoseconds {
			return 1
		}
		return groupCompareOrdinal(a.Ordinal, b.Ordinal)
	})
	if err := groupContext(ctx); err != nil {
		return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return result, nil
}

// A participant's forbidden starts are unioned AFTER duration expansion. This
// prevents both overlapping calendars and nearby busy spans counting twice.
func groupForbidden(ctx context.Context, a Availability, origin temporal.Instant, span, limit int64) ([]groupCandidate, error) {
	var blocked []groupCandidate
	for _, calendar := range a.Calendars {
		for _, busy := range calendar.Busy {
			if err := groupContext(ctx); err != nil {
				return nil, errors.Join(core.ErrTimeMatchContract, err)
			}
			left, leftErr := busy.Start.Since(origin)
			right, rightErr := busy.End.Since(origin)
			if err := errors.Join(leftErr, rightErr); err != nil {
				return nil, errors.Join(core.ErrTimeMatchContract, err)
			}
			start, end := max(int64(0), left.Nanoseconds()-span+1), min(limit, right.Nanoseconds())
			if start < end {
				blocked = append(blocked, groupCandidate{start: start, end: end})
			}
		}
	}
	slices.SortFunc(blocked, func(a, b groupCandidate) int {
		if a.start < b.start {
			return -1
		}
		if a.start > b.start {
			return 1
		}
		if a.end < b.end {
			return -1
		}
		if a.end > b.end {
			return 1
		}
		return 0
	})
	merged := blocked[:0]
	for _, candidate := range blocked {
		if len(merged) > 0 && candidate.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = max(merged[len(merged)-1].end, candidate.end)
		} else {
			merged = append(merged, candidate)
		}
	}
	return merged, nil
}

func groupStart(r GroupMatchRequest, offset int64) (temporal.Instant, error) {
	delta, err := temporal.DurationFromNanoseconds(r.Before.Nanoseconds() + offset)
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	instant, err := r.Window.Start.Add(delta)
	if err != nil {
		return temporal.Instant{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	return instant, nil
}

func groupCompareOrdinal(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func groupCandidateOrder(a, b groupCandidate) int {
	if a.optional > b.optional {
		return -1
	}
	if a.optional < b.optional {
		return 1
	}
	if a.start < b.start {
		return -1
	}
	if a.start > b.start {
		return 1
	}
	return 0
}

// Native typed max-heap of the WORST selected candidate. It stores at most the
// caller's requested output count while still examining the complete horizon.
func groupKeepWindow(selected []groupCandidate, candidate groupCandidate, limit uint64) []groupCandidate {
	if limit == 0 {
		return append(selected, candidate)
	}
	if uint64(len(selected)) < limit {
		selected = append(selected, candidate)
		for i := len(selected) - 1; i > 0; {
			parent := (i - 1) / 2
			if groupCandidateOrder(selected[parent], selected[i]) >= 0 {
				break
			}
			selected[parent], selected[i] = selected[i], selected[parent]
			i = parent
		}
		return selected
	}
	if groupCandidateOrder(candidate, selected[0]) >= 0 {
		return selected
	}
	selected[0] = candidate
	for i := 0; ; {
		child := i*2 + 1
		if child >= len(selected) {
			break
		}
		if child+1 < len(selected) && groupCandidateOrder(selected[child+1], selected[child]) > 0 {
			child++
		}
		if groupCandidateOrder(selected[i], selected[child]) >= 0 {
			break
		}
		selected[i], selected[child] = selected[child], selected[i]
		i = child
	}
	return selected
}

type groupBoundary struct {
	at     int64
	person int
	begin  bool
}
type groupCandidate struct {
	start, end int64
	optional   uint64
}

func (GroupParticipant) timematchProtocolFact()   {}
func (GroupMatchRequest) timematchProtocolFact()  {}
func (GroupStartRange) timematchProtocolFact()    {}
func (GroupWindow) timematchProtocolFact()        {}
func (GroupRemovalImpact) timematchProtocolFact() {}
func (GroupMatchResult) timematchProtocolFact()   {}
func (groupBoundary) timematchInternalFlow()      {}
func (groupCandidate) timematchInternalFlow()     {}

var _ core.Validatable = GroupMatchRequest{}
var _ core.Validatable = GroupStartRange{}

// Preserve the caller's standard cancellation identity and explicit cause.
func groupContext(ctx context.Context) error {
	err := contextstate.Validate(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(err, context.Cause(ctx))
	}
	return err
}
