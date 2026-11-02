package timematch

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// GroupPhaseConstraint requires one resource only during its appointment phase,
// [start+Offset-Before,start+Offset+Duration+After). The whole occupied phase
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
	if index > 0 && p.Ordinal == phases[index-1].Ordinal || p.Availability.Window != r.Window || p.Duration.IsZero() {
		return core.ErrTimeMatchContract
	}
	if err := errors.Join(p.Availability.Validate(), p.Offset.Validate(), p.Duration.Validate(), p.Before.Validate(), p.After.Validate()); err != nil {
		return errors.Join(core.ErrTimeMatchContract, err)
	}
	end, err := p.Offset.Add(p.Duration)
	if err != nil || end.Nanoseconds() > r.Duration.Nanoseconds() {
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
	base := p.request.Before.Nanoseconds() + phase.Offset.Nanoseconds()
	end := base + phase.Duration.Nanoseconds()
	if phase.After.Nanoseconds() > p.input.horizon-end {
		return groupPhaseWindow{}
	}
	end += phase.After.Nanoseconds()
	first := max(int64(0), phase.Before.Nanoseconds()-base)
	limit := min(p.limit, p.input.horizon-end+1)
	return groupPhaseWindow{first: first, limit: limit, startShift: base - phase.Before.Nanoseconds(), endShift: end}
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
		groupHardBlock(p, 0, p.limit)
		return nil
	}
	if p.limit == 0 {
		return nil
	}
	w := groupPhaseBounds(p, phase)
	if w.first >= w.limit {
		groupHardBlock(p, 0, p.limit)
		return nil
	}
	groupHardBlock(p, 0, w.first)
	groupHardBlock(p, w.limit, p.limit)
	for _, c := range phase.Availability.Calendars {
		for _, busy := range c.Busy {
			if err := groupContext(ctx); err != nil {
				return errors.Join(core.ErrTimeMatchContract, err)
			}
			left, leftErr := busy.Start.Since(p.request.Window.Start)
			right, rightErr := busy.End.Since(p.request.Window.Start)
			if err := errors.Join(leftErr, rightErr); err != nil {
				return errors.Join(core.ErrTimeMatchContract, err)
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
