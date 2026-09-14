package permit

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// QuietPeriodMaximum bounds one permission document, not retained evidence.
const QuietPeriodMaximum = 32

// QuietPeriod excludes [Start, End). Instants are exact UTC Unix nanoseconds;
// timezone and recurring calendar policy belong to the issuing product.
type QuietPeriod struct {
	Start temporal.Instant `json:"start"`
	End   temporal.Instant `json:"end"`
}

func (p QuietPeriod) Validate() error {
	order, err := p.Start.Compare(p.End)
	if err != nil || order != core.ComparisonLess {
		return errors.Join(core.ErrReportSchedule, err)
	}
	return nil
}

// QuietPeriods is an immutable bounded ordered set. Empty means no exclusions.
type QuietPeriods struct {
	periods [QuietPeriodMaximum]QuietPeriod
	count   int
}

func NewQuietPeriods(periods ...QuietPeriod) (QuietPeriods, error) {
	if len(periods) > QuietPeriodMaximum {
		return QuietPeriods{}, core.ErrReportSchedule
	}
	var got QuietPeriods
	copy(got.periods[:], periods)
	got.count = len(periods)
	if err := got.Validate(); err != nil {
		return QuietPeriods{}, err
	}
	return got, nil
}
func (q QuietPeriods) Validate() error {
	if q.count < 0 || q.count > QuietPeriodMaximum {
		return core.ErrReportSchedule
	}
	for i, p := range q.periods {
		if i >= q.count {
			if p != (QuietPeriod{}) {
				return core.ErrReportSchedule
			}
			continue
		}
		if err := p.Validate(); err != nil {
			return err
		}
		if i > 0 {
			order, _ := q.periods[i-1].End.Compare(p.Start)
			if order == core.ComparisonGreater {
				return core.ErrReportSchedule
			}
		}
	}
	return nil
}
func (q QuietPeriods) NextAllowed(at temporal.Instant) (temporal.Instant, error) {
	if err := errors.Join(q.Validate(), at.Validate()); err != nil {
		return temporal.Instant{}, err
	}
	for _, p := range q.periods[:q.count] {
		before, _ := at.Compare(p.Start)
		if before == core.ComparisonLess {
			break
		}
		after, _ := at.Compare(p.End)
		if after == core.ComparisonLess {
			at = p.End
		}
	}
	return at, nil
}
func (q QuietPeriods) MarshalJSON() ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(q.periods[:q.count])
}
func (q *QuietPeriods) UnmarshalJSON(data []byte) error {
	if q == nil {
		return core.ErrReportSchedule
	}
	limits := reportLimits()
	limits.ArrayItemMaximum = QuietPeriodMaximum
	periods, err := core.DecodeStrictJSONStructure[[]QuietPeriod](data, limits)
	if err != nil {
		return errors.Join(core.ErrReportSchedule, err)
	}
	if periods == nil {
		return core.ErrReportSchedule
	}
	got, err := NewQuietPeriods(periods...)
	if err != nil {
		return err
	}
	*q = got
	return nil
}

// SendOutsideQuietPeriods intersects admission with caller-owned exclusions.
// Every retry of the calculation crosses at least one exclusion; missed periods
// are still skipped arithmetically. No timer or network request is created here.
func (s ReportSchedule) SendOutsideQuietPeriods(t ReportTiming, jitter temporal.Duration, quiet QuietPeriods) (temporal.Instant, error) {
	if err := quiet.Validate(); err != nil {
		return temporal.Instant{}, err
	}
	for i := 0; i <= quiet.count; i++ {
		candidate, err := s.SendAt(t, jitter)
		if err != nil {
			return temporal.Instant{}, err
		}
		next, err := quiet.NextAllowed(candidate)
		if err != nil {
			return temporal.Instant{}, err
		}
		if next == candidate {
			return candidate, nil
		}
		t.ObservedAt = next
	}
	return temporal.Instant{}, core.ErrReportSchedule
}

// TransmissionPolicy keeps separate report and object-upload exclusions. The
// caller chooses which operation it is performing; Primitive invents no work.
type TransmissionPolicy struct {
	Reports       QuietPeriods `json:"reports"`
	ObjectUploads QuietPeriods `json:"object_uploads"`
}

func (p TransmissionPolicy) Validate() error {
	return errors.Join(p.Reports.Validate(), p.ObjectUploads.Validate())
}
