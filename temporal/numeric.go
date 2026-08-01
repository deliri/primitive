package temporal

import (
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// NumericInstantCanonicalJSONMaximumBytes bounds one canonical numeric
	// instant: the sign plus the widest signed 64-bit decimal.
	NumericInstantCanonicalJSONMaximumBytes = 20
	// NumericDurationCanonicalJSONMaximumBytes bounds one canonical numeric
	// duration: the widest nonnegative signed 64-bit decimal.
	NumericDurationCanonicalJSONMaximumBytes = 19
)

// NumericInstant is an Instant whose JSON projection is a bare number rather
// than a string.
//
// Instant and Duration marshal exact nanoseconds as a JSON string, which keeps
// the value readable in protocols that route large integers through a
// double-precision float. A wire that has always carried those nanoseconds as
// a JSON integer cannot adopt the string form without rewriting bytes a
// signature already covers, so the numeric projection is a compiler-owned
// Primitive contract rather than a wrapper each consumer rewrites.
//
// The value semantics are Instant's. Only the projection differs, so a
// NumericInstant is placed directly in a wire struct with a json tag.
//
// NumericInstant owns encoding, not range policy. It admits the complete
// signed Unix-nanosecond domain, including pre-epoch instants. A consumer whose
// wire is post-epoch enforces that boundary in the type that owns the fact.
type NumericInstant struct {
	value Instant
}

// NewNumericInstant admits one validated instant for numeric projection.
func NewNumericInstant(instant Instant) (NumericInstant, error) {
	if err := instant.Validate(); err != nil {
		return NumericInstant{}, err
	}
	return NumericInstant{value: instant}, nil
}

// Validate rejects the unset zero value, matching Instant.
func (n NumericInstant) Validate() error {
	return n.value.Validate()
}

// IsSet reports whether n crossed a constructor or decode boundary.
func (n NumericInstant) IsSet() bool {
	return n.value.IsSet()
}

// Instant returns the projected value for arithmetic, ordering, and
// truncation. It returns an error rather than an unset Instant because the
// Go zero value of NumericInstant carries no instant at all.
func (n NumericInstant) Instant() (Instant, error) {
	if err := n.value.Validate(); err != nil {
		return Instant{}, err
	}
	return n.value, nil
}

// MarshalJSON emits exact signed nanoseconds as a canonical JSON number.
func (n NumericInstant) MarshalJSON() ([]byte, error) {
	nanoseconds, err := n.value.Nanoseconds()
	if err != nil {
		return nil, err
	}
	return strconv.AppendInt(nil, nanoseconds, 10), nil
}

// UnmarshalJSON accepts one canonical JSON number and leaves the receiver
// untouched on every rejection.
func (n *NumericInstant) UnmarshalJSON(data []byte) error {
	if n == nil {
		return jsonContractError("numeric instant receiver is nil")
	}
	nanoseconds, err := decodeNumericNanoseconds(data, NumericInstantCanonicalJSONMaximumBytes)
	if err != nil {
		return err
	}
	n.value = InstantFromNanoseconds(nanoseconds)
	return nil
}

// NumericDuration is a Duration whose JSON projection is a bare number rather
// than a string. Its value semantics, including the nonnegative bound and the
// meaning of a real zero duration, are Duration's.
type NumericDuration struct {
	value Duration
}

// NewNumericDuration admits one validated duration for numeric projection.
func NewNumericDuration(duration Duration) (NumericDuration, error) {
	if err := duration.Validate(); err != nil {
		return NumericDuration{}, err
	}
	return NumericDuration{value: duration}, nil
}

// Validate rejects a negative duration, matching Duration.
func (n NumericDuration) Validate() error {
	return n.value.Validate()
}

// IsZero reports a real zero duration, which is a valid observation.
func (n NumericDuration) IsZero() bool {
	return n.value.IsZero()
}

// Duration returns the projected value for arithmetic and ordering. Unlike
// NumericInstant.Instant it cannot fail: the Go zero value of NumericDuration
// is an exact zero duration, which Duration admits.
func (n NumericDuration) Duration() Duration {
	return n.value
}

// MarshalJSON emits exact nonnegative nanoseconds as a canonical JSON number.
func (n NumericDuration) MarshalJSON() ([]byte, error) {
	if err := n.value.Validate(); err != nil {
		return nil, err
	}
	return strconv.AppendInt(nil, n.value.Nanoseconds(), 10), nil
}

// UnmarshalJSON accepts one canonical JSON number and leaves the receiver
// untouched on every rejection.
func (n *NumericDuration) UnmarshalJSON(data []byte) error {
	if n == nil {
		return jsonContractError("numeric duration receiver is nil")
	}
	nanoseconds, err := decodeNumericNanoseconds(data, NumericDurationCanonicalJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := DurationFromNanoseconds(nanoseconds)
	if err != nil {
		return err
	}
	n.value = parsed
	return nil
}

var (
	_ core.ValidatedJSONMarshaler = NumericInstant{}
	_ core.ValidatedJSONMarshaler = NumericDuration{}
)
