package controlplane

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// UsageWindowJSONMaximumBytes bounds one reported usage window.
	UsageWindowJSONMaximumBytes = 32 << 10
	// WorkUnitClassMaximum is the highest work-unit class ordinal a product may
	// report, and therefore also the longest legal unit list: the classes are
	// closed and the list is strictly ascending, so a longer one repeats a class.
	WorkUnitClassMaximum = 32
	// OutcomeClassMaximum is the same ceiling for result classes.
	OutcomeClassMaximum = 32
)

// WorkUnitClass names one kind of work an installation performed, as an ordinal
// this package deliberately cannot interpret.
//
// The authority validates the ordinal's range, the window it falls in, and the
// arithmetic between the counts. It never learns what a class means. Which
// command ran, what it was pointed at, and what it found stay on the machine
// that did the work, and the mapping from a product's own closed vocabulary into
// these ordinals is the product's, never crossing the wire in either direction.
//
// This is the same reason a border agent reads a nationality field instead of
// keeping one reader per country. A named class here would write one product's
// vocabulary into a contract both ends share, and every product after it would
// have to add its own.
type WorkUnitClass uint8

// NewWorkUnitClass admits one work-unit class ordinal.
func NewWorkUnitClass(ordinal uint8) (WorkUnitClass, error) {
	class := WorkUnitClass(ordinal)
	if err := class.Validate(); err != nil {
		return 0, err
	}
	return class, nil
}

// Validate rejects the unset ordinal and every ordinal above the ceiling.
func (c WorkUnitClass) Validate() error {
	if c == 0 || c > WorkUnitClassMaximum {
		return usageWindowError()
	}
	return nil
}

// IsValid reports whether c is an admitted class ordinal.
func (c WorkUnitClass) IsValid() bool { return c.Validate() == nil }

// String renders the opaque ordinal for diagnostics. A number is all either
// end knows about a class, so a number is all either end may print.
func (c WorkUnitClass) String() string {
	if !c.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(c), 10)
}

// MarshalJSON emits the canonical decimal ordinal and refuses an inadmissible
// class, the same bytes the bare integer produced before the contract was
// explicit.
func (c WorkUnitClass) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return []byte(strconv.FormatUint(uint64(c), 10)), nil
}

// UnmarshalJSON accepts only the canonical decimal spelling of an admitted
// ordinal and leaves c unchanged on every rejection.
func (c *WorkUnitClass) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(usageWindowError())
	}
	value, err := strconv.ParseUint(string(data), 10, 8)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonError(usageWindowError(err))
	}
	candidate := WorkUnitClass(value)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

// OutcomeClass names one way a unit of work ended, as an ordinal this package
// deliberately cannot interpret. It carries a kind, never a finding.
type OutcomeClass uint8

// NewOutcomeClass admits one outcome class ordinal.
func NewOutcomeClass(ordinal uint8) (OutcomeClass, error) {
	class := OutcomeClass(ordinal)
	if err := class.Validate(); err != nil {
		return 0, err
	}
	return class, nil
}

// Validate rejects the unset ordinal and every ordinal above the ceiling.
func (c OutcomeClass) Validate() error {
	if c == 0 || c > OutcomeClassMaximum {
		return usageWindowError()
	}
	return nil
}

// IsValid reports whether c is an admitted class ordinal.
func (c OutcomeClass) IsValid() bool { return c.Validate() == nil }

// String renders the opaque ordinal for diagnostics, exactly as WorkUnitClass
// does and for the same reason.
func (c OutcomeClass) String() string {
	if !c.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(c), 10)
}

// MarshalJSON emits the canonical decimal ordinal and refuses an inadmissible
// class.
func (c OutcomeClass) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return []byte(strconv.FormatUint(uint64(c), 10)), nil
}

// UnmarshalJSON accepts only the canonical decimal spelling of an admitted
// ordinal and leaves c unchanged on every rejection.
func (c *OutcomeClass) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(usageWindowError())
	}
	value, err := strconv.ParseUint(string(data), 10, 8)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonError(usageWindowError(err))
	}
	candidate := OutcomeClass(value)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

// WorkUnitCount is one class of work and how many units of it ran.
type WorkUnitCount struct {
	Class WorkUnitClass `json:"class"`
	Count uint64        `json:"count"`
}

// Validate rejects an inadmissible class and a zero count. A class that did no
// work is absent from the list rather than reported as having done none.
func (c WorkUnitCount) Validate() error {
	if err := c.Class.Validate(); err != nil {
		return usageWindowError(err)
	}
	if c.Count == 0 {
		return usageWindowError()
	}
	return nil
}

// OutcomeCount is one class of result and how many units ended that way.
type OutcomeCount struct {
	Class OutcomeClass `json:"class"`
	Count uint64       `json:"count"`
}

// Validate rejects an inadmissible class and a zero count.
func (c OutcomeCount) Validate() error {
	if err := c.Class.Validate(); err != nil {
		return usageWindowError(err)
	}
	if c.Count == 0 {
		return usageWindowError()
	}
	return nil
}

// UsageWindow is the complete bounded aggregate one installation reports for one
// interval.
//
// It carries counts over opaque classes and the exact interval they fall in, and
// nothing else. There is no name, path, source, input, output, or finding
// anywhere in it, and no field one could be smuggled through, because the type is
// closed and every member is an ordinal, a count, or an instant. That is the
// agreement with the customer expressed as a struct rather than as a promise.
//
// It is also why there is one check-in rather than one per product. A window
// carrying a product's own counters would be a shape only that product could
// fill and only an authority that already knew that product could read.
type UsageWindow struct {
	Units     []WorkUnitCount         `json:"units"`
	Outcomes  []OutcomeCount          `json:"outcomes"`
	Bounds    temporal.IntervalBounds `json:"bounds"`
	Freshness temporal.Instant        `json:"freshness"`
}

// usageWindowBoundsWire is the wire spelling of the reporting interval.
//
// The member names of a signed document belong to the document's owner, and
// temporal owns a mechanism, not a wire: leaving the interval to marshal
// through temporal's own untagged identifiers put two protocol member names
// under a package that does not know it is on a wire, where a routine rename
// would silently change signed bytes. The projection here pins them in the
// same snake_case vocabulary as every other member the signature covers.
type usageWindowBoundsWire struct {
	Start temporal.Instant `json:"start"`
	End   temporal.Instant `json:"end"`
}

type usageWindowWire struct {
	Units     []WorkUnitCount       `json:"units"`
	Outcomes  []OutcomeCount        `json:"outcomes"`
	Bounds    usageWindowBoundsWire `json:"bounds"`
	Freshness temporal.Instant      `json:"freshness"`
}

func (w UsageWindow) wire() usageWindowWire {
	return usageWindowWire{
		Units:     w.Units,
		Outcomes:  w.Outcomes,
		Bounds:    usageWindowBoundsWire{Start: w.Bounds.Start, End: w.Bounds.End},
		Freshness: w.Freshness,
	}
}

func (w usageWindowWire) window() UsageWindow {
	return UsageWindow{
		Units:     w.Units,
		Outcomes:  w.Outcomes,
		Bounds:    temporal.IntervalBounds{Start: w.Bounds.Start, End: w.Bounds.End},
		Freshness: w.Freshness,
	}
}

// Validate closes every reported fact and every relationship between them.
func (w UsageWindow) Validate() error {
	if err := errors.Join(w.Bounds.Validate(), w.Freshness.Validate()); err != nil {
		return usageWindowError(err)
	}
	if err := validateUsageFreshness(w.Bounds, w.Freshness); err != nil {
		return err
	}
	units, err := validateWorkUnitCounts(w.Units)
	if err != nil {
		return err
	}
	outcomes, err := validateOutcomeCounts(w.Outcomes)
	if err != nil {
		return err
	}
	return validateUsageTotals(units, outcomes)
}

// validateUsageTotals proves the classifications account for exactly the work.
//
// Every unit an installation reports ended exactly one way, so the two totals
// are the same number counted twice. A window whose outcomes do not account for
// its own units describes no run that could have happened, and an authority that
// accepted it would be metering arithmetic nobody can reproduce.
func validateUsageTotals(units, outcomes uint64) error {
	if units != outcomes {
		return usageWindowError()
	}
	return nil
}

// validateWorkUnitCounts requires strictly ascending classes, so a repeated
// class is a decode failure rather than a silent double count, and returns the
// total units reported.
//
// The walk needs no length bound of its own. Classes are closed at
// WorkUnitClassMaximum and must strictly increase, so entry number
// WorkUnitClassMaximum plus one cannot hold an admissible class whatever the
// caller sends: the walk returns by then however long the list is. A separate
// length check would read as the bound while never being the branch that fires.
func validateWorkUnitCounts(counts []WorkUnitCount) (uint64, error) {
	var total uint64
	for index, count := range counts {
		if err := count.Validate(); err != nil {
			return 0, err
		}
		if index > 0 && counts[index-1].Class >= count.Class {
			return 0, usageWindowError()
		}
		if math.MaxUint64-total < count.Count {
			return 0, usageWindowError()
		}
		total += count.Count
	}
	return total, nil
}

// validateOutcomeCounts applies the same ordering and overflow rules to the
// result classes, under the same closed ceiling, and returns the total units
// classified.
func validateOutcomeCounts(counts []OutcomeCount) (uint64, error) {
	var total uint64
	for index, count := range counts {
		if err := count.Validate(); err != nil {
			return 0, err
		}
		if index > 0 && counts[index-1].Class >= count.Class {
			return 0, usageWindowError()
		}
		if math.MaxUint64-total < count.Count {
			return 0, usageWindowError()
		}
		total += count.Count
	}
	return total, nil
}

// MarshalJSON emits one bounded canonical usage window.
func (w UsageWindow) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(w.wire())
	if err != nil || len(encoded) > UsageWindowJSONMaximumBytes {
		return nil, jsonError(usageWindowError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (w *UsageWindow) UnmarshalJSON(data []byte) error {
	if w == nil {
		return jsonError(usageWindowError())
	}
	limits, err := documentJSONLimits(UsageWindowJSONMaximumBytes)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[usageWindowWire](data, limits)
	if err != nil {
		return jsonError(usageWindowError(err))
	}
	candidate := wire.window()
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*w = candidate
	return nil
}

var (
	_ core.Validatable = WorkUnitClass(0)
	_ core.Validatable = OutcomeClass(0)
	_ core.Validatable = WorkUnitCount{}
	_ core.Validatable = OutcomeCount{}
	_ core.Validatable = UsageWindow{}

	_ core.ValidatedJSONMarshaler = UsageWindow{}
	_ core.ValidatedJSONMarshaler = WorkUnitClass(0)
	_ core.ValidatedJSONMarshaler = OutcomeClass(0)

	_ json.Unmarshaler = (*UsageWindow)(nil)
	_ json.Unmarshaler = (*WorkUnitClass)(nil)
	_ json.Unmarshaler = (*OutcomeClass)(nil)
)
