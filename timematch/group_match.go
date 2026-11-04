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
// All participants and phase constraints cover exactly Window. At least one
// required participant or hard phase constraint is required.
// MaximumWindows selects the best windows across the entire horizon. Zero
// returns every actual boundary window, with caller-owned output allocation.
type GroupMatchRequest struct {
	Window           temporal.IntervalBounds
	Duration         temporal.Duration
	Before           temporal.Duration
	After            temporal.Duration
	Participants     []GroupParticipant
	PhaseConstraints []GroupPhaseConstraint
	MaximumWindows   uint64
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
	UnknownPhaseConstraints  []uint64
	RequiredCount            uint64
	KnownOptionalCount       uint64
	FeasibleStartNanoseconds uint64
	Truncated                bool
}

func (r GroupMatchRequest) Validate() error {
	input, err := groupInputShape(r)
	if err != nil {
		return err
	}
	required := false
	for i, person := range input.people {
		if err := groupPersonContract(input.people, i, r.Window); err != nil {
			return err
		}
		required = required || person.Required
	}
	if !required && len(input.phases) == 0 {
		return core.ErrTimeMatchContract
	}
	for i := range input.phases {
		if err := groupPhaseContract(input.phases, i, r); err != nil {
			return err
		}
	}
	return nil
}

// Shape admission is pure; it constructs no execution context.
func groupInputShape(r GroupMatchRequest) (groupInput, error) {
	if err := errors.Join(positiveInterval(r.Window), r.Duration.Validate(), r.Before.Validate(), r.After.Validate()); err != nil {
		return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	if r.Duration.IsZero() || len(r.Participants) == 0 && len(r.PhaseConstraints) == 0 {
		return groupInput{}, core.ErrTimeMatchContract
	}
	span, err := r.Before.Add(r.Duration)
	if err == nil {
		span, err = span.Add(r.After)
	}
	if err != nil {
		return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	horizon, err := r.Window.End.Since(r.Window.Start)
	if err != nil {
		return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	people := slices.Clone(r.Participants)
	slices.SortFunc(people, func(a, b GroupParticipant) int { return groupCompareOrdinal(a.Ordinal, b.Ordinal) })
	phases := slices.Clone(r.PhaseConstraints)
	slices.SortFunc(phases, func(a, b GroupPhaseConstraint) int { return groupCompareOrdinal(a.Ordinal, b.Ordinal) })
	return groupInput{people: people, phases: phases, span: span.Nanoseconds(), horizon: horizon.Nanoseconds()}, nil
}

func groupPersonContract(people []GroupParticipant, index int, window temporal.IntervalBounds) error {
	person := people[index]
	if index > 0 && person.Ordinal == people[index-1].Ordinal || person.Availability.Window != window {
		return core.ErrTimeMatchContract
	}
	if err := person.Availability.Validate(); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// Runtime admission shares the pure validators and polls the caller's real
// context between people, including immediately after the private sort.
func groupAdmit(ctx context.Context, r GroupMatchRequest) (groupInput, error) {
	if err := groupContext(ctx); err != nil {
		return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
	}
	input, err := groupInputShape(r)
	if err != nil {
		return groupInput{}, err
	}
	required := false
	for i, person := range input.people {
		if err := groupContext(ctx); err != nil {
			return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		if err := groupPersonContract(input.people, i, r.Window); err != nil {
			return groupInput{}, err
		}
		required = required || person.Required
	}
	if !required && len(input.phases) == 0 {
		return groupInput{}, core.ErrTimeMatchContract
	}
	for i := range input.phases {
		if err := groupContext(ctx); err != nil {
			return groupInput{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		if err := groupPhaseContract(input.phases, i, r); err != nil {
			return groupInput{}, err
		}
	}
	return input, nil
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
	input, err := groupAdmit(ctx, r)
	if err != nil {
		return GroupMatchResult{}, err
	}
	prepared, err := groupPrepare(ctx, r, input)
	if err != nil {
		return GroupMatchResult{}, err
	}
	if prepared.limit == 0 {
		if err := groupContext(ctx); err != nil {
			return GroupMatchResult{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		return prepared.result, nil
	}
	selected, count, err := groupSweep(ctx, &prepared)
	if err != nil {
		return GroupMatchResult{}, err
	}
	return groupMaterialize(ctx, &prepared, selected, count)
}

func groupPrepare(ctx context.Context, r GroupMatchRequest, input groupInput) (groupPreparation, error) {
	prepared := groupPreparation{request: r, input: input, result: GroupMatchResult{RemovalImpacts: make([]GroupRemovalImpact, len(input.people))}}
	if input.span <= input.horizon {
		prepared.limit = input.horizon - input.span + 1
	}
	prepared.dead, prepared.boundaries = groupDeadAndCapacity(input)
	for i := range input.people {
		if err := groupContext(ctx); err != nil {
			return groupPreparation{}, errors.Join(core.ErrTimeMatchContract, err)
		}
		if err := groupPreparePerson(ctx, &prepared, i); err != nil {
			return groupPreparation{}, err
		}
	}
	for _, phase := range input.phases {
		if err := groupPreparePhase(ctx, &prepared, phase); err != nil {
			return groupPreparation{}, err
		}
	}
	if prepared.dead {
		groupHardBlock(&prepared, 0, prepared.limit)
	}
	slices.SortFunc(prepared.boundaries, func(a, b groupBoundary) int {
		if a.at < b.at {
			return -1
		}
		if a.at > b.at {
			return 1
		}
		return groupCompareOrdinal(uint64(a.person), uint64(b.person))
	})
	return prepared, nil
}

// groupDeadAndCapacity decides in one O(calendars) pass whether any start can
// be feasible and sizes the boundary slice once: two boundaries per blocked
// span, at most one span per busy interval, two per Unknown required person
// and per phase bound, and six per phase busy interval.
func groupDeadAndCapacity(input groupInput) (bool, []groupBoundary) {
	unknownRequired := 0
	capacity := 0
	for _, person := range input.people {
		unknown := false
		for _, calendar := range person.Availability.Calendars {
			unknown = unknown || calendar.State == AvailabilityUnknown
			capacity += 2 * len(calendar.Busy)
		}
		if unknown {
			capacity += 2
			if person.Required {
				unknownRequired++
			}
		}
	}
	dead := unknownRequired >= 2
	for _, phase := range input.phases {
		capacity += 4
		for _, calendar := range phase.Availability.Calendars {
			dead = dead || calendar.State == AvailabilityUnknown
			capacity += 6 * len(calendar.Busy)
		}
	}
	if dead {
		return true, make([]groupBoundary, 0, 2)
	}
	return false, make([]groupBoundary, 0, capacity)
}

func groupPreparePerson(ctx context.Context, p *groupPreparation, index int) error {
	person := p.input.people[index]
	unknown := false
	for _, calendar := range person.Availability.Calendars {
		unknown = unknown || calendar.State == AvailabilityUnknown
	}
	p.result.RemovalImpacts[index] = GroupRemovalImpact{Ordinal: person.Ordinal, Required: person.Required, Unknown: unknown}
	if person.Required {
		p.result.RequiredCount++
	}
	if unknown {
		if person.Required {
			p.result.UnknownRequired = append(p.result.UnknownRequired, person.Ordinal)
			if p.limit > 0 && !p.dead {
				p.boundaries = append(p.boundaries, groupBoundary{at: 0, person: index, begin: true}, groupBoundary{at: p.limit, person: index})
			}
		} else {
			p.result.UnknownOptional = append(p.result.UnknownOptional, person.Ordinal)
		}
		return nil
	}
	if !person.Required {
		p.result.KnownOptionalCount++
	}
	if p.limit == 0 || p.dead {
		return nil
	}
	blocked, blockErr := groupForbidden(ctx, groupForbiddenRequest{availability: person.Availability, origin: p.request.Window.Start, span: p.input.span, limit: p.limit})
	if blockErr != nil {
		return blockErr
	}
	for _, b := range blocked {
		p.boundaries = append(p.boundaries, groupBoundary{at: b.start, person: index, begin: true}, groupBoundary{at: b.end, person: index})
	}
	return nil
}

func groupSweep(ctx context.Context, p *groupPreparation) ([]groupCandidate, uint64, error) {
	people, boundaries, limit, result := p.input.people, p.boundaries, p.limit, &p.result
	var selected []groupCandidate
	var pending groupCandidate
	pendingSet := false
	var windowCount uint64
	flush := func() {
		if !pendingSet {
			return
		}
		windowCount++
		selected = groupKeepWindow(selected, pending, p.request.MaximumWindows)
		pendingSet = false
	}
	requiredBlocked, optionalBlocked, hardBlocked := 0, 0, 0
	var requiredXOR uint64
	cursor, event := int64(0), 0
	for step := 0; cursor < limit; step++ {
		// One cancellation observation per groupPollStride positions keeps the
		// hot loop free of a mutex and a deferred recover per boundary.
		if step&groupPollStride == 0 {
			if err := groupContext(ctx); err != nil {
				return nil, 0, errors.Join(core.ErrTimeMatchContract, err)
			}
		}
		for event < len(boundaries) && boundaries[event].at == cursor {
			b := boundaries[event]
			change := -1
			if b.begin {
				change = 1
			}
			if b.hard {
				hardBlocked += change
			} else if people[b.person].Required {
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
		if requiredBlocked == 0 && hardBlocked == 0 {
			result.FeasibleStartNanoseconds += uint64(next - cursor)
			available := result.KnownOptionalCount - uint64(optionalBlocked)
			if pendingSet && pending.end == cursor && pending.optional == available {
				pending.end = next
			} else {
				flush()
				pending = groupCandidate{start: cursor, end: next, optional: available}
				pendingSet = true
			}
			cursor = next
			continue
		}
		flush()
		if requiredBlocked == 1 && hardBlocked == 0 {
			if err := groupRemovalGain(p, requiredXOR, groupCandidate{start: cursor, end: next}); err != nil {
				return nil, 0, err
			}
		}
		cursor = next
	}
	flush()
	return selected, windowCount, nil
}

func groupRemovalGain(p *groupPreparation, owner uint64, span groupCandidate) error {
	impact := &p.result.RemovalImpacts[owner-1]
	if impact.AdditionalStartNanoseconds == 0 {
		first, err := groupStart(p.request, span.start)
		if err != nil {
			return err
		}
		impact.EarliestUnlocked = first
	}
	impact.AdditionalStartNanoseconds += uint64(span.end - span.start)
	return nil
}

func groupMaterialize(ctx context.Context, p *groupPreparation, selected []groupCandidate, windowCount uint64) (GroupMatchResult, error) {
	r, result := p.request, &p.result
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
	return *result, nil
}

// A participant's forbidden starts are unioned AFTER duration expansion. This
// prevents both overlapping calendars and nearby busy spans counting twice.
func groupForbidden(ctx context.Context, request groupForbiddenRequest) ([]groupCandidate, error) {
	a, origin, span, limit := request.availability, request.origin, request.span, request.limit
	total := 0
	for _, calendar := range a.Calendars {
		total += len(calendar.Busy)
	}
	blocked := make([]groupCandidate, 0, total)
	step := 0
	for _, calendar := range a.Calendars {
		for _, busy := range calendar.Busy {
			if step&groupPollStride == 0 {
				if err := groupContext(ctx); err != nil {
					return nil, errors.Join(core.ErrTimeMatchContract, err)
				}
			}
			step++
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
	// One calendar's busy intervals are validated sorted and disjoint, and the
	// expansion is monotone, so its blocked spans are already in order.
	if len(a.Calendars) > 1 {
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
	}
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

// The following carriers hold only one real stage's input or prepared facts.
type groupInput struct {
	people        []GroupParticipant
	phases        []GroupPhaseConstraint
	span, horizon int64
}
type groupPreparation struct {
	request    GroupMatchRequest
	input      groupInput
	result     GroupMatchResult
	boundaries []groupBoundary
	limit      int64
	// dead means no start can ever be feasible (an Unknown phase constraint or
	// two Unknown required participants), so busy expansion is skipped and one
	// hard block covers the horizon; counts and Unknown lists are still filled.
	dead bool
}
type groupForbiddenRequest struct {
	availability Availability
	origin       temporal.Instant
	span, limit  int64
}

func (groupInput) timematchInternalFlow()            {}
func (groupPreparation) timematchInternalFlow()      {}
func (groupForbiddenRequest) timematchInternalFlow() {}

type groupBoundary struct {
	at     int64
	person int
	begin  bool
	hard   bool
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
// groupPollStride masks the loop step at which the hot loops observe the
// caller's context: the first step and then every 1024th.
const groupPollStride = 1023

func groupContext(ctx context.Context) error {
	err := contextstate.Validate(ctx)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(err, context.Cause(ctx))
	}
	return err
}
