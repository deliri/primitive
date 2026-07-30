package core

import "math"

// declaredBodyLengthAbsent is the single integer HTTP transports use to report a
// message that declares no body extent. It lives here so no caller has to
// remember which negative number means "unknown".
const declaredBodyLengthAbsent = -1

// DeclaredBodyLength is the body extent an HTTP message declares in advance.
//
// Absence is a first-class state rather than a magic number: a chunked message
// declares no extent at all, and that is a different fact from declaring zero
// bytes. The zero value is the absent declaration, which is the safe default.
type DeclaredBodyLength struct {
	length  ByteLength
	present bool
}

// ParseDeclaredBodyLength accepts the extent an HTTP transport reports: a
// non-negative declared length, or declaredBodyLengthAbsent for a message that
// declares none. Any other negative value is not an extent HTTP can express.
func ParseDeclaredBodyLength(value int64) (DeclaredBodyLength, error) {
	if value == declaredBodyLengthAbsent {
		return DeclaredBodyLength{}, nil
	}
	if value < declaredBodyLengthAbsent {
		return DeclaredBodyLength{}, httpContractError(
			"declared body length is not an expressible extent",
		)
	}
	length, err := CheckedUint64FromInt64(value)
	if err != nil {
		return DeclaredBodyLength{}, err
	}
	return DeclaredBodyLength{length: NewByteLength(length), present: true}, nil
}

// Present reports whether the message declared an extent at all.
func (d DeclaredBodyLength) Present() bool {
	return d.present
}

// Length returns the declared extent. An absent declaration reads as zero, so
// Present must be consulted before treating the result as the body size.
func (d DeclaredBodyLength) Length() ByteLength {
	return d.length
}

// Validate rejects an absent declaration that carries an extent, the one state
// no constructor can produce and therefore the one state that would mean a value
// was assembled outside ParseDeclaredBodyLength.
func (d DeclaredBodyLength) Validate() error {
	if !d.present && d.length.Uint64() != 0 {
		return httpContractError("absent declared body length carries an extent")
	}
	return nil
}

// ExceedsLimit reports whether the message already declares more bytes than the
// limit admits, so an oversized body is refused before a single byte is read.
// An absent declaration exceeds nothing: an undeclared extent is bounded while
// reading instead of before it.
func (d DeclaredBodyLength) ExceedsLimit(limit ByteCount) (bool, error) {
	allowed, err := limit.Uint64()
	if err != nil {
		return false, err
	}
	if err := d.Validate(); err != nil {
		return false, err
	}
	if !d.present {
		return false, nil
	}
	return d.length.Uint64() > allowed, nil
}

// ReservedExtent returns the number of bytes worth reserving up front to hold
// this message whole.
//
// The reservation never exceeds the limit the caller already authorized for this
// body, so a declaration cannot enlarge the memory an operation was permitted:
// the worst a hostile authority achieves is the budget its counterparty already
// granted. A message that declares no extent reserves nothing and grows as it is
// read.
func (d DeclaredBodyLength) ReservedExtent(limit ByteCount) (int, error) {
	exceeds, err := d.ExceedsLimit(limit)
	if err != nil {
		return 0, err
	}
	if !d.present {
		return 0, nil
	}
	if exceeds {
		allowed, allowedErr := limit.Uint64()
		if allowedErr != nil {
			return 0, allowedErr
		}
		return checkedIntFromUint64(allowed)
	}
	return checkedIntFromUint64(d.length.Uint64())
}

func checkedIntFromUint64(value uint64) (int, error) {
	if value > math.MaxInt {
		return 0, numericOverflow("uint64 does not fit int")
	}
	return int(value), nil
}

var _ Validatable = DeclaredBodyLength{}
