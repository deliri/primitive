package permit

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ReportSchedule is a server-issued recurring half-open admission window.
// Product cadence and spread are selected by the caller, never by Primitive.
type ReportSchedule struct {
	NextReportAt   temporal.Instant         `json:"next_report_at"`
	WindowDuration temporal.Duration        `json:"window_duration"`
	RepeatInterval temporal.Duration        `json:"repeat_interval"`
	JitterMaximum  temporal.Duration        `json:"jitter_maximum"`
	Policy         controlwire.PolicyCursor `json:"policy"`
}

func (s ReportSchedule) Validate() error {
	if err := errors.Join(s.NextReportAt.Validate(), s.WindowDuration.Validate(), s.RepeatInterval.Validate(), s.JitterMaximum.Validate(), s.Policy.Validate()); err != nil {
		return errors.Join(core.ErrReportSchedule, err)
	}
	w, p, j := s.WindowDuration.Nanoseconds(), s.RepeatInterval.Nanoseconds(), s.JitterMaximum.Nanoseconds()
	if w <= 0 || w >= p || j >= w {
		return core.ErrReportSchedule
	}
	if _, err := s.NextReportAt.Add(s.WindowDuration); err != nil {
		return errors.Join(core.ErrReportSchedule, err)
	}
	return nil
}

// ReportOccurrence is the next usable intersection with current authorization.
// It contains no decision that a future instant has already been reached.
type ReportOccurrence struct {
	Open  temporal.Instant `json:"open"`
	Close temporal.Instant `json:"close"`
}

func (o ReportOccurrence) Validate() error {
	order, err := o.Open.Compare(o.Close)
	if err != nil || order != core.ComparisonLess {
		return errors.Join(core.ErrReportSchedule, err)
	}
	return nil
}

type ReportTiming struct {
	ObservedAt temporal.Instant
	NotBefore  temporal.Instant
	ExpiresAt  temporal.Instant
}

func (t ReportTiming) Validate() error {
	if err := errors.Join(t.ObservedAt.Validate(), t.NotBefore.Validate(), t.ExpiresAt.Validate()); err != nil {
		return errors.Join(core.ErrReportSchedule, err)
	}
	order, _ := t.NotBefore.Compare(t.ExpiresAt)
	if order != core.ComparisonLess {
		return core.ErrReportSchedule
	}
	return nil
}

// Occurrence jumps over missed periods arithmetically, using bounded memory.
// Unsigned distance avoids overflow across the signed Unix epoch extremes.
func (s ReportSchedule) Occurrence(t ReportTiming) (ReportOccurrence, error) {
	if err := errors.Join(s.Validate(), t.Validate()); err != nil {
		return ReportOccurrence{}, err
	}
	o, _ := s.NextReportAt.Nanoseconds()
	now, _ := t.ObservedAt.Nanoseconds()
	nb, _ := t.NotBefore.Nanoseconds()
	expiry, _ := t.ExpiresAt.Nanoseconds()
	q := max(now, nb)
	p, w := s.RepeatInterval.Nanoseconds(), s.WindowDuration.Nanoseconds()
	open := o
	if q >= o {
		remainder := int64((uint64(q) - uint64(o)) % uint64(p))
		open = q - remainder
		if remainder >= w {
			if open > math.MaxInt64-p {
				return ReportOccurrence{}, core.ErrReportOverflow
			}
			open += p
		}
	}
	if open > math.MaxInt64-w {
		return ReportOccurrence{}, core.ErrReportOverflow
	}
	closeAt := min(open+w, expiry)
	open = max(open, nb)
	if open >= closeAt || now >= expiry {
		return ReportOccurrence{}, core.ErrPermitValidity
	}
	return ReportOccurrence{Open: temporal.InstantFromNanoseconds(open), Close: temporal.InstantFromNanoseconds(closeAt)}, nil
}

// Admit checks the observed instant, not the future eligible instant.
func (s ReportSchedule) Admit(t ReportTiming) (ReportOccurrence, error) {
	o, err := s.Occurrence(t)
	if err != nil {
		return ReportOccurrence{}, err
	}
	now, _ := t.ObservedAt.Nanoseconds()
	open, _ := o.Open.Nanoseconds()
	initial, _ := s.NextReportAt.Nanoseconds()
	if now < open {
		if now < initial {
			return o, core.ErrReportTooEarly
		}
		return o, core.ErrReportOutsideWindow
	}
	return o, nil
}

// SendAt clips a caller-sampled jitter to the remaining half-open interval.
// Callers wait with cancellation and re-observe before opening the request.
func (s ReportSchedule) SendAt(t ReportTiming, jitter temporal.Duration) (temporal.Instant, error) {
	if err := jitter.Validate(); err != nil {
		return temporal.Instant{}, errors.Join(core.ErrReportSchedule, err)
	}
	if jitter.Nanoseconds() > s.JitterMaximum.Nanoseconds() {
		return temporal.Instant{}, core.ErrReportSchedule
	}
	o, err := s.Occurrence(t)
	if err != nil {
		return temporal.Instant{}, err
	}
	now, _ := t.ObservedAt.Nanoseconds()
	open, _ := o.Open.Nanoseconds()
	closeAt, _ := o.Close.Nanoseconds()
	start := max(now, open)
	remaining := uint64(closeAt) - uint64(start) - 1
	delay := min(uint64(jitter.Nanoseconds()), remaining)
	return temporal.InstantFromNanoseconds(start + int64(delay)), nil
}
