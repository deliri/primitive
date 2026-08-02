package temporal

import (
	"math"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// NanosecondsPerMicrosecond is the exact microsecond magnitude.
	NanosecondsPerMicrosecond uint64 = 1_000
	// NanosecondsPerMillisecond is the exact millisecond magnitude.
	NanosecondsPerMillisecond uint64 = 1_000_000
	// NanosecondsPerSecond is the exact second magnitude.
	NanosecondsPerSecond uint64 = 1_000_000_000
	// NanosecondsPerMinute is the exact minute magnitude.
	NanosecondsPerMinute uint64 = 60 * NanosecondsPerSecond
	// NanosecondsPerHour is the exact hour magnitude.
	NanosecondsPerHour uint64 = 60 * NanosecondsPerMinute
	// NanosecondsPerDay is the exact 24-hour day magnitude.
	NanosecondsPerDay uint64 = 24 * NanosecondsPerHour
	// DurationMaximumNanoseconds is the largest bounded duration.
	DurationMaximumNanoseconds = int64(math.MaxInt64)
	// AggregateDurationMaximumDecimalDigits bounds an unsigned 128-bit decimal.
	AggregateDurationMaximumDecimalDigits = 39
	// InstantCanonicalJSONMaximumBytes bounds compact signed instant JSON.
	InstantCanonicalJSONMaximumBytes = 22
	// DurationCanonicalJSONMaximumBytes bounds compact duration JSON.
	DurationCanonicalJSONMaximumBytes = 21
	// AggregateDurationCanonicalJSONMaximumBytes bounds compact aggregate JSON.
	AggregateDurationCanonicalJSONMaximumBytes = 41
	// TemporalJSONDocumentSlackBytes bounds insignificant JSON whitespace.
	TemporalJSONDocumentSlackBytes = 256
	// InstantJSONMaximumBytes bounds accepted instant JSON.
	InstantJSONMaximumBytes = InstantCanonicalJSONMaximumBytes + TemporalJSONDocumentSlackBytes
	// DurationJSONMaximumBytes bounds accepted duration JSON.
	DurationJSONMaximumBytes = DurationCanonicalJSONMaximumBytes + TemporalJSONDocumentSlackBytes
	// AggregateDurationJSONMaximumBytes bounds accepted aggregate JSON.
	AggregateDurationJSONMaximumBytes = AggregateDurationCanonicalJSONMaximumBytes + TemporalJSONDocumentSlackBytes
)

const (
	precisionUnknownDiagnostic     = "unknown"
	precisionNanosecondDiagnostic  = "nanosecond"
	precisionMicrosecondDiagnostic = "microsecond"
	precisionMillisecondDiagnostic = "millisecond"
	precisionSecondDiagnostic      = "second"
	precisionInvalidReason         = "precision is outside the admitted domain"
	durationUnitOverflowReason     = "duration unit conversion exceeded int64 nanoseconds"
)

// Precision is the closed set of exact instant truncation boundaries.
type Precision uint8

const (
	// PrecisionUnknown is the invalid zero precision.
	PrecisionUnknown Precision = iota
	// PrecisionNanosecond preserves every nanosecond.
	PrecisionNanosecond
	// PrecisionMicrosecond truncates to a microsecond boundary.
	PrecisionMicrosecond
	// PrecisionMillisecond truncates to a millisecond boundary.
	PrecisionMillisecond
	// PrecisionSecond truncates to a second boundary.
	PrecisionSecond
	precisionLimit
)

// precisionFact is everything the closed domain knows about one precision. It
// exists so the domain has exactly one home: a member's diagnostic and its
// truncation magnitude cannot drift apart, and Validate cannot admit a member
// that has no magnitude.
// The magnitude is int64 because that is the type instant truncation needs. The
// table literal converts each uint64 magnitude constant at compile time, so an
// unrepresentable magnitude is a build failure rather than a runtime branch that
// no input can reach.
type precisionFact struct {
	diagnostic string
	magnitude  int64
}

// precisionFacts is indexed by Precision and sized by precisionLimit, so a new
// Precision constant that arrives without its row makes the array literal fail
// to compile. fact also rejects an explicitly empty row, preventing an admitted
// member from reaching truncation without a diagnostic and magnitude.
func precisionFacts() [precisionLimit]precisionFact {
	return [...]precisionFact{
		PrecisionNanosecond:  {diagnostic: precisionNanosecondDiagnostic, magnitude: 1},
		PrecisionMicrosecond: {diagnostic: precisionMicrosecondDiagnostic, magnitude: int64(NanosecondsPerMicrosecond)},
		PrecisionMillisecond: {diagnostic: precisionMillisecondDiagnostic, magnitude: int64(NanosecondsPerMillisecond)},
		PrecisionSecond:      {diagnostic: precisionSecondDiagnostic, magnitude: int64(NanosecondsPerSecond)},
	}
}

// fact is the single admission gate. Validate, String, and nanoseconds all
// project from it, so no member can be admitted by one and refused by another.
func (p Precision) fact() (precisionFact, error) {
	if p <= PrecisionUnknown || p >= precisionLimit {
		return precisionFact{}, contractError(precisionInvalidReason)
	}
	got := precisionFacts()[p]
	if got.magnitude == 0 || got.diagnostic == "" {
		return precisionFact{}, contractError(precisionInvalidReason)
	}
	return got, nil
}

// Validate rejects values outside the closed precision domain.
func (p Precision) Validate() error {
	_, err := p.fact()
	return err
}

// IsValid reports whether p belongs to the closed precision domain.
func (p Precision) IsValid() bool {
	return p.Validate() == nil
}

// OffWireEnum declares Precision as an off-wire enum. The declaration binds
// Precision to core.OffWireEnum below, so the marker is compiler-checked rather
// than a bare method name matched by convention.
func (Precision) OffWireEnum() {}

// String returns a diagnostic projection of p.
func (p Precision) String() string {
	got, err := p.fact()
	if err != nil {
		return precisionUnknownDiagnostic
	}
	return got.diagnostic
}

func (p Precision) nanoseconds() (int64, error) {
	got, err := p.fact()
	if err != nil {
		return 0, err
	}
	return got.magnitude, nil
}

func durationFromMagnitude(value, magnitude uint64) (Duration, error) {
	if value > uint64(DurationMaximumNanoseconds)/magnitude {
		return Duration{}, overflowError(durationUnitOverflowReason)
	}
	// #nosec G115 -- the quotient guard proves the product fits int64.
	return Duration{nanoseconds: int64(value * magnitude)}, nil
}

var (
	_ core.Validatable = PrecisionUnknown
	_ core.OffWireEnum = PrecisionUnknown
)
