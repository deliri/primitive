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

// ByteLength is a non-negative byte length in Go's signed size domain; unlike
// ByteCount, zero is meaningful. The upper bound matches the int64 quantities
// exposed by the standard library for file, stream, and HTTP body sizes.
type ByteLength struct {
	value uint64
}

// NewByteLength constructs a non-negative byte length.
func NewByteLength(value uint64) (ByteLength, error) {
	length := ByteLength{value: value}
	if err := length.Validate(); err != nil {
		return ByteLength{}, err
	}
	return length, nil
}

// Uint64 returns the length.
func (l ByteLength) Uint64() uint64 {
	return l.value
}

// Int64 returns the length when it fits in int64.
func (l ByteLength) Int64() (int64, error) {
	if err := l.Validate(); err != nil {
		return 0, err
	}
	return int64(l.value), nil // #nosec G115 -- Validate proves the conversion is in range.
}

// Validate rejects lengths outside Go's signed size domain.
func (l ByteLength) Validate() error {
	if l.value > math.MaxInt64 {
		return numericOverflow("byte length exceeds Go's signed size domain")
	}
	return nil
}

// MarshalJSON emits the length as a canonical JSON integer.
func (l ByteLength) MarshalJSON() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
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
	decoded, err := NewByteLength(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*l = decoded
	return nil
}

// CheckedUint32FromInt converts value or returns ErrNumericOverflow.
func CheckedUint32FromInt(value int) (uint32, error) {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0, numericOverflow("int does not fit uint32")
	}
	return uint32(value), nil
}

// CheckedUint16FromInt converts value or returns ErrNumericOverflow.
func CheckedUint16FromInt(value int) (uint16, error) {
	if value < 0 || uint64(value) > math.MaxUint16 {
		return 0, numericOverflow("int does not fit uint16")
	}
	return uint16(value), nil
}

// CheckedUint8FromInt converts value or returns ErrNumericOverflow.
func CheckedUint8FromInt(value int) (uint8, error) {
	if value < 0 || uint64(value) > math.MaxUint8 {
		return 0, numericOverflow("int does not fit uint8")
	}
	return uint8(value), nil
}

// CheckedInt32FromInt converts value or returns ErrNumericOverflow.
func CheckedInt32FromInt(value int) (int32, error) {
	if int64(value) < math.MinInt32 || int64(value) > math.MaxInt32 {
		return 0, numericOverflow("int does not fit int32")
	}
	return int32(value), nil
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
