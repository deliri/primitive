package core

import (
	"bytes"
	"errors"
	"math"
	"strconv"
)

// ByteCount is a strictly positive byte quantity. Its zero value is invalid.
type ByteCount struct {
	value uint64
}

// NewByteCount constructs a positive byte count.
func NewByteCount(value uint64) (ByteCount, error) {
	count := ByteCount{value: value}
	if err := count.Validate(); err != nil {
		return ByteCount{}, err
	}
	return count, nil
}

// Uint64 returns the validated count.
func (c ByteCount) Uint64() (uint64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	return c.value, nil
}

// Int64 returns the validated count when it fits in int64.
func (c ByteCount) Int64() (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	return CheckedInt64FromUint64(c.value)
}

// Validate rejects a zero byte count.
func (c ByteCount) Validate() error {
	if c.value == 0 {
		return errors.Join(ErrPrimitiveContract, errors.New("byte count must be positive"))
	}
	return nil
}

// MarshalJSON emits the positive count as a canonical JSON integer.
func (c ByteCount) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return strconv.AppendUint(nil, c.value, 10), nil
}

// UnmarshalJSON accepts a canonical positive unsigned JSON integer.
func (c *ByteCount) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(ErrJSONContract, errors.New("nil byte count receiver"))
	}
	value, err := parseCanonicalUint64JSON(data)
	if err != nil {
		return err
	}
	decoded, err := NewByteCount(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*c = decoded
	return nil
}

// ByteLength is a non-negative byte length; unlike ByteCount, zero is meaningful.
type ByteLength struct {
	value uint64
}

// NewByteLength constructs a non-negative byte length.
func NewByteLength(value uint64) ByteLength {
	return ByteLength{value: value}
}

// Uint64 returns the length.
func (l ByteLength) Uint64() uint64 {
	return l.value
}

// Int64 returns the length when it fits in int64.
func (l ByteLength) Int64() (int64, error) {
	return CheckedInt64FromUint64(l.value)
}

// MarshalJSON emits the length as a canonical JSON integer.
func (l ByteLength) MarshalJSON() ([]byte, error) {
	return strconv.AppendUint(nil, l.value, 10), nil
}

// UnmarshalJSON accepts a canonical non-negative unsigned JSON integer.
func (l *ByteLength) UnmarshalJSON(data []byte) error {
	if l == nil {
		return errors.Join(ErrJSONContract, errors.New("nil byte length receiver"))
	}
	value, err := parseCanonicalUint64JSON(data)
	if err != nil {
		return err
	}
	*l = NewByteLength(value)
	return nil
}

// CheckedUint32FromInt converts value or returns ErrNumericOverflow.
func CheckedUint32FromInt(value int) (uint32, error) {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0, numericOverflow("int does not fit uint32")
	}
	return uint32(value), nil
}

// CheckedInt64FromUint64 converts value or returns ErrNumericOverflow.
func CheckedInt64FromUint64(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, numericOverflow("uint64 does not fit int64")
	}
	return int64(value), nil
}

// CheckedUint64FromInt64 converts a non-negative value or returns
// ErrNumericOverflow.
func CheckedUint64FromInt64(value int64) (uint64, error) {
	if value < 0 {
		return 0, numericOverflow("negative int64 does not fit uint64")
	}
	return uint64(value), nil
}

// ParseCanonicalUint64JSON admits exactly the unsigned decimal text strconv
// emits. It is the single owner of the canonical-integer rule: parse, re-encode,
// and require byte equality. That makes the accepted grammar the encoder's own
// output rather than a second hand-written grammar that can drift from it, so a
// quoted number, a plus sign, a leading zero, a fraction, an exponent, or
// surrounding whitespace is rejected without enumerating those cases.
//
// A value therefore has exactly one accepted encoding, which is the property a
// byte-signing protocol depends on.
func ParseCanonicalUint64JSON(data []byte) (uint64, error) {
	return parseCanonicalUint64JSON(data)
}

// ParseCanonicalInt64JSON admits exactly the signed decimal text strconv emits,
// under the same round-trip rule as ParseCanonicalUint64JSON. Negative zero is
// rejected because strconv never emits it.
func ParseCanonicalInt64JSON(data []byte) (int64, error) {
	if len(data) == 0 {
		return 0, errors.Join(ErrJSONContract, errors.New("empty signed integer"))
	}
	value, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, errors.Join(ErrJSONContract, err)
	}
	if !bytes.Equal(data, strconv.AppendInt(nil, value, 10)) {
		return 0, errors.Join(ErrJSONContract, errors.New("signed integer is not canonical"))
	}
	return value, nil
}

func parseCanonicalUint64JSON(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, errors.Join(ErrJSONContract, errors.New("empty unsigned integer"))
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil {
		return 0, errors.Join(ErrJSONContract, err)
	}
	if !bytes.Equal(data, strconv.AppendUint(nil, value, 10)) {
		return 0, errors.Join(ErrJSONContract, errors.New("unsigned integer is not canonical"))
	}
	return value, nil
}

func numericOverflow(message string) error {
	return errors.Join(ErrNumericOverflow, errors.New(message))
}
