package controlwire

import (
	"encoding/binary"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// PolicyRevisionTextLength is the exact rendered width of a policy revision.
	PolicyRevisionTextLength = 26
	// policyRevisionAlphabet is Crockford base32, which omits I, L, O, and U so
	// no two symbols can be confused when a revision is read aloud or retyped.
	policyRevisionAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	// policyRevisionLeadingSymbolMaximum bounds the first symbol. Twenty-six
	// symbols carry 130 bits and the identifier is 128, so the leading symbol
	// contributes three usable bits and a larger value names a number that does
	// not fit in sixteen bytes.
	policyRevisionLeadingSymbolMaximum = 7
	// policyRevisionSymbolBits is the width one base32 symbol contributes.
	policyRevisionSymbolBits = 5
	// policyRevisionSymbolMask selects one symbol from the low end of a word.
	policyRevisionSymbolMask = 1<<policyRevisionSymbolBits - 1
	// policyRevisionCarryShift moves the low word's top symbol into the high
	// word during rendering, and back during parsing.
	policyRevisionCarryShift = 64 - policyRevisionSymbolBits
)

// PolicyRevisionID identifies one exact published control policy.
//
// A control plane publishes policy revisions and every installation reports
// which one it is actually running under. This package owns the identifier's
// encoding, not the policy: what a revision permits, when it activates, and
// what an installation must do about it are decisions the control plane makes
// and this type never reads.
type PolicyRevisionID [16]byte

// PolicyActivation orders activations within one policy revision history.
//
// Left as a plain unsigned integer with no JSON methods of its own. The wire
// form is a bare JSON number and the standard decoder already refuses a quoted
// number, a fraction, an exponent, a sign, and a leading zero. Restating that
// here could only create an opportunity to accept something it rejects. The
// zero value is refused, by this type and again by PolicyCursor.
type PolicyActivation uint64

// PolicyCursor is one installation's exact position in a control plane's
// policy history: which revision, and which activation of it.
//
// Declaration order is the wire order and also the layout the machine prefers,
// so this fact needs no separate wire adapter to keep the two apart.
type PolicyCursor struct {
	Revision   PolicyRevisionID `json:"revision"`
	Activation PolicyActivation `json:"activation"`
}

type policyCursorWire PolicyCursor

// Validate rejects an unset revision and an unset activation.
//
// There is no permissive spelling of an absent cursor. A control plane may
// distinguish "no policy yet" from an active policy in its own records, but a
// cursor only reaches this package inside an authenticated response, where the
// sender has already committed to a revision. Admitting the zero here would
// give a blank or truncated record a second meaning that reads as valid.
func (c PolicyCursor) Validate() error {
	if err := c.Revision.Validate(); err != nil {
		return err
	}
	return c.Activation.Validate()
}

// Validate rejects the all-zero identifier.
//
// All-zero renders as a well-formed twenty-six symbol identifier, so it would
// otherwise survive parsing. It is what a blank, truncated, or
// default-initialised record decodes to, and admitting it would let two such
// records agree that they are running the same policy.
func (id PolicyRevisionID) Validate() error {
	if id == (PolicyRevisionID{}) {
		return policyCursorError()
	}
	return nil
}

// Validate rejects the unset activation.
func (a PolicyActivation) Validate() error {
	if a == 0 {
		return policyCursorError()
	}
	return nil
}

// Uint64 returns the activation as a plain counter.
func (a PolicyActivation) Uint64() uint64 { return uint64(a) }

// Compare orders two activations within one revision history.
func (a PolicyActivation) Compare(other PolicyActivation) core.Comparison {
	switch {
	case a < other:
		return core.ComparisonLess
	case a > other:
		return core.ComparisonGreater
	}
	return core.ComparisonEqual
}

// String renders the canonical uppercase Crockford base32 identifier.
func (id PolicyRevisionID) String() string {
	high := binary.BigEndian.Uint64(id[0:8])
	low := binary.BigEndian.Uint64(id[8:16])
	var text [PolicyRevisionTextLength]byte
	for position := PolicyRevisionTextLength - 1; position >= 0; position-- {
		text[position] = policyRevisionAlphabet[low&policyRevisionSymbolMask]
		low = low>>policyRevisionSymbolBits | high<<policyRevisionCarryShift
		high >>= policyRevisionSymbolBits
	}
	return string(text[:])
}

// ParsePolicyRevisionID accepts one exact canonical rendering.
//
// Crockford base32 ordinarily admits lowercase and reads I and L as one and O
// as zero. This refuses all of it. A control plane emits one spelling and an
// installation echoes the identifier back on every later exchange, so a value
// decoded from an alias would re-encode to different bytes than arrived. The
// leniency would not tolerate a harmless variant; it would break the echo.
func ParsePolicyRevisionID(value string) (PolicyRevisionID, error) {
	if len(value) != PolicyRevisionTextLength {
		return PolicyRevisionID{}, policyCursorError()
	}
	var high, low uint64
	for position := 0; position < PolicyRevisionTextLength; position++ {
		symbol := strings.IndexByte(policyRevisionAlphabet, value[position])
		if symbol < 0 {
			return PolicyRevisionID{}, policyCursorError()
		}
		if position == 0 && symbol > policyRevisionLeadingSymbolMaximum {
			return PolicyRevisionID{}, policyCursorError()
		}
		high = high<<policyRevisionSymbolBits | low>>policyRevisionCarryShift
		low = low<<policyRevisionSymbolBits | uint64(symbol)
	}
	var parsed PolicyRevisionID
	binary.BigEndian.PutUint64(parsed[0:8], high)
	binary.BigEndian.PutUint64(parsed[8:16], low)
	return parsed, parsed.Validate()
}

// MarshalJSON emits the canonical identifier and refuses the unset value.
func (id PolicyRevisionID) MarshalJSON() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(id.String())
	if err != nil {
		return nil, jsonError(policyCursorError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a canonical identifier and leaves the receiver
// unchanged on every rejection.
func (id *PolicyRevisionID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return jsonError(policyCursorError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(policyCursorError(err))
	}
	parsed, err := ParsePolicyRevisionID(token)
	if err != nil {
		return jsonError(err)
	}
	*id = parsed
	return nil
}

// MarshalJSON emits one validated cursor.
func (c PolicyCursor) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(policyCursorWire(c))
	if err != nil {
		return nil, jsonError(policyCursorError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (c *PolicyCursor) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(policyCursorError())
	}
	wire, err := core.DecodeStrictJSONStructure[policyCursorWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return jsonError(policyCursorError(err))
	}
	candidate := PolicyCursor(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}
