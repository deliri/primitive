package permit

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// ReportHead is a caller-supplied committed identity, not owned mutable state.
// Compare proves only sequence/content agreement; callers own persistence.
type ReportHead struct {
	Scope    ReportScope
	Sequence uint64
	Digest   core.SHA256Digest
}

func (h ReportHead) Validate() error {
	if err := errors.Join(h.Scope.Validate(), h.Digest.Validate()); err != nil {
		return err
	}
	if h.Sequence > math.MaxInt64 {
		return core.ErrReportSequence
	}
	return nil
}

// Compare returns true only for exact replay. False/nil means exactly next.
func (h ReportHead) Compare(report SignedReport) (bool, error) {
	if err := errors.Join(h.Validate(), report.Validate()); err != nil {
		return false, err
	}
	if h.Scope != report.Payload.Scope {
		return false, core.ErrReportBinding
	}
	digest, err := report.Digest()
	if err != nil {
		return false, err
	}
	if report.Payload.Sequence == h.Sequence && h.Sequence != 0 {
		if digest != h.Digest {
			return false, core.ErrReportConflict
		}
		return true, nil
	}
	if h.Sequence == math.MaxInt64 || report.Payload.Sequence != h.Sequence+1 || report.Payload.Previous != h.Digest {
		return false, core.ErrReportSequence
	}
	return false, nil
}
