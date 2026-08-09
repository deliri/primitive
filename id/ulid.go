package id

import (
	"encoding"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// ulidTextBytes is the exact extent of the one canonical spelling.
	ulidTextBytes = 26
	// crockfordAlphabet is Crockford base32: I, L, O, and U are excluded so a
	// spelling cannot be misread back into a different value.
	crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	// ulidFirstValueCeiling bounds the first character's group. Twenty-six
	// five-bit groups carry one hundred thirty bits, so the leading group
	// holds only the three bits that keep the value inside sixteen bytes.
	ulidFirstValueCeiling = 7
	// ulidLowHalfShift is where a five-bit group stops fitting entirely in
	// the low sixty-four bits of the value.
	ulidLowHalfShift = 59
	// ulidHighHalfShift is where a five-bit group fits entirely in the high
	// sixty-four bits of the value.
	ulidHighHalfShift = 64
)

// ULID is one canonical ULID value: the observation's Unix milliseconds in
// the first forty-eight bits and caller-drawn entropy in the remaining
// eighty. Its zero value is invalid.
type ULID struct {
	value [identityBytes]byte
}

// NewULID builds one canonical value from the validated request. The same
// request always builds the same value: construction performs no effect, and
// the entropy copy is cleared before returning.
func NewULID(request Request) (ULID, error) {
	if err := request.Validate(); err != nil {
		return ULID{}, err
	}
	milliseconds, err := observedMilliseconds(request.Observation)
	if err != nil {
		return ULID{}, err
	}
	entropy, err := request.Entropy.CopyBytes()
	if err != nil {
		return ULID{}, contractCause(entropyUnreadableDiagnostic, err)
	}
	defer clear(entropy)
	var value ULID
	putTimestamp(value.value[:], milliseconds)
	copy(value.value[timestampBytes:], entropy[:entropyBytes])
	// Nothing here forces a bit: the epoch stamp beside an all zero entropy
	// head builds the unset value, and this check is what refuses it.
	if err := value.Validate(); err != nil {
		return ULID{}, err
	}
	return value, nil
}

// ParseULID admits exactly one canonical spelling: twenty-six uppercase
// Crockford base32 bytes whose first byte never exceeds seven. Lowercase,
// padded, separated, and ambiguous-letter spellings are refused, because a
// persisted identity has one spelling or it is not this identity.
func ParseULID(value string) (ULID, error) {
	parsed, ok := decodeCanonicalULIDText(value)
	if !ok {
		return ULID{}, contractError("ulid text is outside the canonical form")
	}
	if err := parsed.Validate(); err != nil {
		return ULID{}, err
	}
	return parsed, nil
}

// decodeCanonicalULIDText folds the twenty-six five-bit groups into sixteen
// bytes and refuses every byte outside the alphabet and every leading group
// past the ceiling.
func decodeCanonicalULIDText(value string) (ULID, bool) {
	if len(value) != ulidTextBytes {
		return ULID{}, false
	}
	var high, low uint64
	for position := range ulidTextBytes {
		group := strings.IndexByte(crockfordAlphabet, value[position])
		if group < 0 {
			return ULID{}, false
		}
		if position == 0 && group > ulidFirstValueCeiling {
			return ULID{}, false
		}
		high = high<<5 | low>>ulidLowHalfShift
		low = low<<5 | uint64(group)
	}
	var parsed ULID
	binary.BigEndian.PutUint64(parsed.value[0:8], high)
	binary.BigEndian.PutUint64(parsed.value[8:16], low)
	return parsed, true
}

// canonicalText spells the value into one stack-owned canonical form shared
// by String and AppendText, so the spelling has exactly one implementation.
func (u ULID) canonicalText() [ulidTextBytes]byte {
	high := binary.BigEndian.Uint64(u.value[0:8])
	low := binary.BigEndian.Uint64(u.value[8:16])
	var text [ulidTextBytes]byte
	for position := range ulidTextBytes {
		shift := uint(125 - 5*position)
		text[position] = crockfordAlphabet[crockfordGroup(high, low, shift)]
	}
	return text
}

// String returns the canonical uppercase spelling, or the empty string for a
// value that fails its own contract.
func (u ULID) String() string {
	if !u.IsValid() {
		return ""
	}
	text := u.canonicalText()
	return string(text[:])
}

// AppendText appends the canonical uppercase spelling to destination and
// returns the extended slice. It is the streaming spelling for canonical
// writers that spell many identities into one caller-owned buffer: the append
// allocates only when destination lacks capacity, and the unset value is
// refused instead of spelled.
func (u ULID) AppendText(destination []byte) ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, err
	}
	text := u.canonicalText()
	return append(destination, text[:]...), nil
}

// crockfordGroup extracts the five-bit group whose lowest bit sits at shift
// inside the hundred-twenty-eight bit value high carries above low.
func crockfordGroup(high, low uint64, shift uint) byte {
	switch {
	case shift >= ulidHighHalfShift:
		return byte(high >> (shift - ulidHighHalfShift) & 31)
	case shift > ulidLowHalfShift:
		return byte((high<<(ulidHighHalfShift-shift) | low>>shift) & 31)
	default:
		return byte(low >> shift & 31)
	}
}

// NewULIDFromBytes admits one identity from the sixteen bytes another door
// handed out, so a value that crossed a byte-shaped boundary can be rebuilt
// without a detour through its text. The all zero array is refused: it is the
// unset value, not an identity, and admitting it here would let an absent
// identity re-enter as a real one.
func NewULIDFromBytes(value [identityBytes]byte) (ULID, error) {
	parsed := ULID{value: value}
	if err := parsed.Validate(); err != nil {
		return ULID{}, err
	}
	return parsed, nil
}

// Bytes returns the value's sixteen bytes, for a caller that must key or
// digest on the identity itself rather than on its spelling. The array is a
// copy, so a caller cannot reach back through it into the value. The unset
// value is refused: an absent identity must not silently digest as sixteen
// zeros, which is the one input that would collide across every caller.
func (u ULID) Bytes() ([identityBytes]byte, error) {
	if err := u.Validate(); err != nil {
		return [identityBytes]byte{}, err
	}
	return u.value, nil
}

// IsZero reports the unset value.
func (u ULID) IsZero() bool {
	return u.value == [identityBytes]byte{}
}

// IsValid reports a set value. A ULID carries no version marks, so set is
// the whole shape rule.
func (u ULID) IsValid() bool {
	return !u.IsZero()
}

// Validate rejects the unset value.
func (u ULID) Validate() error {
	if !u.IsValid() {
		return contractError("ulid is unset")
	}
	return nil
}

// MarshalJSON emits the canonical spelling after validation.
func (u ULID) MarshalJSON() ([]byte, error) {
	if err := u.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(u.String())
}

// UnmarshalJSON admits exactly one canonical spelling. The receiver is
// unchanged on rejection.
func (u *ULID) UnmarshalJSON(data []byte) error {
	if u == nil {
		return errors.Join(core.ErrJSONContract, contractError("nil ulid receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseULID(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*u = parsed
	return nil
}

var (
	_ core.Validatable            = ULID{}
	_ core.ValidatedJSONMarshaler = ULID{}
	_ encoding.TextAppender       = ULID{}
)
