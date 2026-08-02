package release

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// TargetCount is the exact number of artifacts in one release.
	TargetCount = 4
	// ReleaseLatestMaximumLifetimeNanoseconds is the exact 24-hour Latest
	// validity ceiling.
	ReleaseLatestMaximumLifetimeNanoseconds int64 = int64(temporal.NanosecondsPerDay)
	// ReleaseClockRollbackToleranceNanoseconds is the exact five-minute
	// correction tolerance against a signed issue instant.
	ReleaseClockRollbackToleranceNanoseconds int64 = 5 * int64(temporal.NanosecondsPerMinute)
)

// Revision is the closed Release wire revision.
type Revision uint8

const (
	RevisionUnknown Revision = iota
	Revision2026V1
	revisionLimit
)

func (r Revision) Validate() error {
	if r <= RevisionUnknown || r >= revisionLimit || revisionLabels()[r] == "" {
		return contractError(errors.New("revision is outside the closed domain"))
	}
	return nil
}

func (r Revision) IsValid() bool { return r.Validate() == nil }
func (Revision) OffWireEnum()    {}

func (r Revision) String() string {
	if r >= revisionLimit || revisionLabels()[r] == "" {
		return core.UnknownEnumDiagnostic
	}
	return revisionLabels()[r]
}

func revisionLabels() [revisionLimit]string {
	return [...]string{"", "2026-v1"}
}

// LatestFreshness classifies a verified Latest at one observation.
type LatestFreshness uint8

const (
	LatestFreshnessUnknown LatestFreshness = iota
	LatestFreshnessNotYetValid
	LatestFreshnessCurrent
	LatestFreshnessExpired
	latestFreshnessLimit
)

func (f LatestFreshness) Validate() error {
	if f <= LatestFreshnessUnknown || f >= latestFreshnessLimit ||
		latestFreshnessLabels()[f] == "" {
		return latestError(errors.New("freshness is outside the closed domain"))
	}
	return nil
}

func (f LatestFreshness) IsValid() bool { return f.Validate() == nil }
func (LatestFreshness) OffWireEnum()    {}

// String returns a stable diagnostic label.
func (f LatestFreshness) String() string {
	if f >= latestFreshnessLimit || latestFreshnessLabels()[f] == "" {
		return core.UnknownEnumDiagnostic
	}
	return latestFreshnessLabels()[f]
}

func latestFreshnessLabels() [latestFreshnessLimit]string {
	return [...]string{"", "not-yet-valid", currentDiagnostic, "expired"}
}

// LatestClockState records whether the signed issue floor corrected a local
// observation.
type LatestClockState uint8

const (
	LatestClockUnknown LatestClockState = iota
	LatestClockObserved
	LatestClockCorrected
	latestClockLimit
)

func (s LatestClockState) Validate() error {
	if s <= LatestClockUnknown || s >= latestClockLimit ||
		latestClockStateLabels()[s] == "" {
		return latestError(errors.New("clock state is outside the closed domain"))
	}
	return nil
}

func (s LatestClockState) IsValid() bool { return s.Validate() == nil }
func (LatestClockState) OffWireEnum()    {}

// String returns a stable diagnostic label.
func (s LatestClockState) String() string {
	if s >= latestClockLimit || latestClockStateLabels()[s] == "" {
		return core.UnknownEnumDiagnostic
	}
	return latestClockStateLabels()[s]
}

func latestClockStateLabels() [latestClockLimit]string {
	return [...]string{"", "observed", "corrected"}
}

// LatestAdvanceState is the complete successful advance result domain.
type LatestAdvanceState uint8

const (
	LatestAdvanceUnknown LatestAdvanceState = iota
	LatestAdvanceReplay
	LatestAdvanceAdvanced
	latestAdvanceLimit
)

func (s LatestAdvanceState) Validate() error {
	if s <= LatestAdvanceUnknown || s >= latestAdvanceLimit ||
		latestAdvanceStateLabels()[s] == "" {
		return contractError(errors.New("advance state is outside the closed domain"))
	}
	return nil
}

func (s LatestAdvanceState) IsValid() bool { return s.Validate() == nil }
func (LatestAdvanceState) OffWireEnum()    {}

// String returns a stable diagnostic label.
func (s LatestAdvanceState) String() string {
	if s >= latestAdvanceLimit || latestAdvanceStateLabels()[s] == "" {
		return core.UnknownEnumDiagnostic
	}
	return latestAdvanceStateLabels()[s]
}

func latestAdvanceStateLabels() [latestAdvanceLimit]string {
	return [...]string{"", "replay", "advanced"}
}

func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(r.String())
}

func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("revision receiver is nil"))
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	if value != Revision2026V1.String() {
		return jsonError(errors.New("revision token is unsupported"))
	}
	canonical, _ := json.Marshal(value)
	if string(canonical) != string(data) {
		return jsonError(errors.New("revision token is not canonical"))
	}
	*r = Revision2026V1
	return nil
}

var (
	_ core.OffWireEnum = RevisionUnknown
	_ core.OffWireEnum = LatestFreshnessUnknown
	_ core.OffWireEnum = LatestClockUnknown
	_ core.OffWireEnum = LatestAdvanceUnknown
)
