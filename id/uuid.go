package id

import (
	"encoding"
	"encoding/hex"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// uuidTextBytes is the exact extent of the one canonical spelling.
	uuidTextBytes = 36
	// uuidCompactBytes is that spelling with its four dashes removed.
	uuidCompactBytes = 32
)

// UUIDv7 is one canonical RFC 9562 version 7 value: the observation's Unix
// milliseconds in the first forty-eight bits, caller-drawn entropy behind
// them, and the version and variant marks set by this package. Its zero
// value is invalid.
type UUIDv7 struct {
	value [identityBytes]byte
}

// NewUUIDv7 builds one canonical version 7 value from the validated request.
// The same request always builds the same value: construction performs no
// effect, and the entropy copy is cleared before returning.
func NewUUIDv7(request Request) (UUIDv7, error) {
	if err := request.Validate(); err != nil {
		return UUIDv7{}, err
	}
	milliseconds, err := observedMilliseconds(request.Observation)
	if err != nil {
		return UUIDv7{}, err
	}
	entropy, err := request.Entropy.CopyBytes()
	if err != nil {
		return UUIDv7{}, contractCause(entropyUnreadableDiagnostic, err)
	}
	defer clear(entropy)
	var value UUIDv7
	putTimestamp(value.value[:], milliseconds)
	copy(value.value[timestampBytes:], entropy[:entropyBytes])
	value.value[6] = value.value[6]&0x0f | 0x70
	value.value[8] = value.value[8]&0x3f | 0x80
	// The marks above force a nonzero value, so this check cannot fire; it
	// stays to catch layout drift, the one shape no compile-time witness
	// covers.
	if err := value.Validate(); err != nil {
		return UUIDv7{}, err
	}
	return value, nil
}

// uuidOutsideCanonicalFormDiagnostic is the one spelling of the canonical
// form refusal, answered with and without an underlying decode cause.
const uuidOutsideCanonicalFormDiagnostic = "uuid text is outside the canonical form"

// ParseUUIDv7 admits exactly one canonical spelling: thirty-six lowercase
// hexadecimal and dash bytes in 8-4-4-4-12 groups carrying the version 7 and
// RFC 9562 variant marks. Uppercase, braced, urn-prefixed, hyphenless, and
// padded spellings are refused, because a persisted identity has one
// spelling or it is not this identity.
func ParseUUIDv7(value string) (UUIDv7, error) {
	compact, ok := compactCanonicalUUIDText(value)
	if !ok {
		return UUIDv7{}, contractError(uuidOutsideCanonicalFormDiagnostic)
	}
	var parsed UUIDv7
	if _, err := hex.Decode(parsed.value[:], compact[:]); err != nil {
		return UUIDv7{}, contractCause(uuidOutsideCanonicalFormDiagnostic, err)
	}
	if err := parsed.Validate(); err != nil {
		return UUIDv7{}, err
	}
	return parsed, nil
}

// compactCanonicalUUIDText strips the four fixed dashes and refuses every
// spelling that is not the lowercase canonical form.
func compactCanonicalUUIDText(value string) ([uuidCompactBytes]byte, bool) {
	var compact [uuidCompactBytes]byte
	if len(value) != uuidTextBytes || !uuidDashesPlaced(value) {
		return compact, false
	}
	index := 0
	for position := range uuidTextBytes {
		if uuidDashPosition(position) {
			continue
		}
		character := value[position]
		if character >= 'A' && character <= 'F' {
			return compact, false
		}
		compact[index] = character
		index++
	}
	return compact, true
}

// uuidDashesPlaced reports the four canonical separator positions hold dashes.
func uuidDashesPlaced(value string) bool {
	return value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-'
}

// uuidDashPosition reports whether position is one of the four separators.
func uuidDashPosition(position int) bool {
	return position == 8 || position == 13 || position == 18 || position == 23
}

// canonicalText spells the value into one stack-owned canonical form shared
// by String and AppendText, so the spelling has exactly one implementation.
func (u UUIDv7) canonicalText() [uuidTextBytes]byte {
	var text [uuidTextBytes]byte
	hex.Encode(text[0:8], u.value[0:4])
	text[8] = '-'
	hex.Encode(text[9:13], u.value[4:6])
	text[13] = '-'
	hex.Encode(text[14:18], u.value[6:8])
	text[18] = '-'
	hex.Encode(text[19:23], u.value[8:10])
	text[23] = '-'
	hex.Encode(text[24:36], u.value[10:16])
	return text
}

// String returns the canonical lowercase spelling, or the empty string for a
// value that fails its own contract.
func (u UUIDv7) String() string {
	if !u.IsValid() {
		return ""
	}
	text := u.canonicalText()
	return string(text[:])
}

// AppendText appends the canonical lowercase spelling to destination and
// returns the extended slice. It is the streaming spelling for canonical
// writers that spell many identities into one caller-owned buffer: the append
// allocates only when destination lacks capacity, and the unset value is
// refused instead of spelled.
func (u UUIDv7) AppendText(destination []byte) ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, err
	}
	text := u.canonicalText()
	return append(destination, text[:]...), nil
}

// IsZero reports the unset value.
func (u UUIDv7) IsZero() bool {
	return u.value == [identityBytes]byte{}
}

// IsValid reports a set value carrying the version 7 and RFC 9562 variant
// marks.
func (u UUIDv7) IsValid() bool {
	return !u.IsZero() && u.value[6]>>4 == 7 && u.value[8]&0xc0 == 0x80
}

// Validate rejects the unset value and every non version 7 shape.
func (u UUIDv7) Validate() error {
	if !u.IsValid() {
		return contractError("uuid is not a set version 7 value")
	}
	return nil
}

// MarshalJSON emits the canonical spelling after validation.
func (u UUIDv7) MarshalJSON() ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(u.String())
}

// UnmarshalJSON admits exactly one canonical spelling. The receiver is
// unchanged on rejection.
func (u *UUIDv7) UnmarshalJSON(data []byte) error {
	if u == nil {
		return errors.Join(core.ErrJSONContract, contractError("nil uuid receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseUUIDv7(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*u = parsed
	return nil
}

var (
	_ core.Validatable            = UUIDv7{}
	_ core.ValidatedJSONMarshaler = UUIDv7{}
	_ encoding.TextAppender       = UUIDv7{}
)
