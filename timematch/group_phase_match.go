package timematch

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// GroupPhaseConstraint requires one resource only during its appointment phase,
// [start+Offset-OffsetBefore-Before,start+Offset-OffsetBefore+Duration+After).
// Offset and OffsetBefore are mutually exclusive, defining a signed relative
// anchor; a phase may lie wholly before or after the visit. The occupied phase
// must fit Window. It is always hard: it is never a participant, attendance
// score or removable-person bottleneck. Ordinals are unique within phases and
// independent of participant ordinals. Availability covers the request Window.
// Resource capacity and overlapping unit demand are caller policy, not inferred.
type GroupPhaseConstraint struct {
	Ordinal      uint64
	Availability Availability
	Offset       temporal.Duration
	OffsetBefore temporal.Duration
	Duration     temporal.Duration
	Before       temporal.Duration
	After        temporal.Duration
}

func groupPhaseContract(phases []GroupPhaseConstraint, index int, r GroupMatchRequest) error {
	p := phases[index]
	if index > 0 && p.Ordinal == phases[index-1].Ordinal || p.Availability.Window != r.Window || p.Duration.IsZero() || !p.Offset.IsZero() && !p.OffsetBefore.IsZero() {
		return core.ErrTimeMatchContract
	}
	if err := errors.Join(p.Availability.Validate(), p.Offset.Validate(), p.OffsetBefore.Validate(), p.Duration.Validate(), p.Before.Validate(), p.After.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	span, err := p.Before.Add(p.Duration)
	if err == nil {
		_, err = span.Add(p.After)
	}
	if err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	return nil
}

// Cursor coordinates refer to the complete appointment's occupied start;
// appointment start is Window.Start+request.Before+cursor. Bounds are clipped
// before arithmetic, so even signed-duration extrema never wrap.
type groupPhaseWindow struct{ first, limit, startShift, endShift int64 }

func groupPhaseBounds(p *groupPreparation, phase GroupPhaseConstraint) groupPhaseWindow {
	anchor := temporal.InstantFromNanoseconds(p.request.Before.Nanoseconds())
	var err error
	if phase.OffsetBefore.IsZero() {
		anchor, err = anchor.Add(phase.Offset)
	} else {
		anchor, err = anchor.Subtract(phase.OffsetBefore)
	}
	if err != nil {
		return groupPhaseWindow{}
	}
	left, leftErr := anchor.Subtract(phase.Before)
	right, rightErr := anchor.Add(phase.Duration)
	if rightErr == nil {
		right, rightErr = right.Add(phase.After)
	}
	if leftErr != nil || rightErr != nil {
		return groupPhaseWindow{}
	}
	startShift, leftErr := left.Nanoseconds()
	endShift, rightErr := right.Nanoseconds()
	if leftErr != nil || rightErr != nil || startShift < -(p.limit-1) || endShift > p.input.horizon {
		return groupPhaseWindow{}
	}
	// Earlier-than-MinInt64 shifts and later-than-MaxInt64 ends cannot fit any
	// nonnegative cursor in this finite horizon. Temporal overflow above is thus
	// an impossible fit, not permission to wrap or widen a resource constraint.
	first := max(int64(0), -startShift)
	limit := p.limit
	// A nonpositive end shift cannot constrain the latest cursor. Avoid adding
	// one to horizon-endShift, which need not have a signed representation.
	if endShift > 0 {
		limit = min(limit, p.input.horizon-endShift+1)
	}
	return groupPhaseWindow{first: first, limit: limit, startShift: startShift, endShift: endShift}
}

func groupHardBlock(p *groupPreparation, first, limit int64) {
	if first < limit {
		p.boundaries = append(p.boundaries, groupBoundary{at: first, begin: true, hard: true}, groupBoundary{at: limit, hard: true})
	}
}

func groupPreparePhase(ctx context.Context, p *groupPreparation, phase GroupPhaseConstraint) error {
	if err := groupContext(ctx); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	unknown := false
	for _, c := range phase.Availability.Calendars {
		unknown = unknown || c.State == AvailabilityUnknown
	}
	if unknown {
		p.result.UnknownPhaseConstraints = append(p.result.UnknownPhaseConstraints, phase.Ordinal)
	}
	// An Unknown phase makes the preparation dead: one hard block over the
	// whole horizon is added once by groupPrepare instead of per phase.
	if unknown || p.limit == 0 || p.dead {
		return nil
	}
	w := groupPhaseBounds(p, phase)
	if w.first >= w.limit {
		groupHardBlock(p, 0, p.limit)
		return nil
	}
	groupHardBlock(p, 0, w.first)
	groupHardBlock(p, w.limit, p.limit)
	step := 0
	for _, c := range phase.Availability.Calendars {
		for _, busy := range c.Busy {
			if step&groupPollStride == 0 {
				if err := groupContext(ctx); err != nil {
					return errors.Join(core.ErrTimeMatchContract, err)
				}
			}
			step++
			left, leftErr := busy.Start.Since(p.request.Window.Start)
			right, rightErr := busy.End.Since(p.request.Window.Start)
			if err := errors.Join(leftErr, rightErr); err != nil {
				return errors.Join(core.ErrTimeMatchContract, err)
			}
			// If even the last admissible phase ends before this busy period,
			// there is no overlap. This bounds subtraction with negative shifts.
			if left.Nanoseconds() >= w.limit-1+w.endShift {
				continue
			}
			first := max(w.first, left.Nanoseconds()-w.endShift+1)
			// w.limit+startShift is within [0,horizon]. Clip here before
			// subtracting a possibly negative shift from the busy end.
			limit := w.limit
			if right.Nanoseconds() < w.limit+w.startShift {
				limit = right.Nanoseconds() - w.startShift
			}
			groupHardBlock(p, first, limit)
		}
	}
	return nil
}
func (GroupPhaseConstraint) timematchProtocolFact() {}
func (groupPhaseWindow) timematchInternalFlow()     {}
