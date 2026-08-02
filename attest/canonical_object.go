package attest

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// CanonicalFieldNameMaximumBytes bounds one canonical member name.
	CanonicalFieldNameMaximumBytes = 64
	// CanonicalObjectMaximumFields bounds the members of one canonical object.
	CanonicalObjectMaximumFields = 64
	// CanonicalFieldNameSeparator is the only punctuation a canonical member
	// name may carry. It may not be doubled and may not sit at either end.
	CanonicalFieldNameSeparator = '_'

	canonicalObjectOpen     = '{'
	canonicalObjectClose    = '}'
	canonicalMemberComma    = ','
	canonicalMemberColon    = ':'
	canonicalNameQuote      = '"'
	canonicalTrueText       = "true"
	canonicalFalseText      = "false"
	canonicalMemberNullText = "null"
	canonicalDecimalBase    = 10
	// A base-10 uint64 uses at most 20 bytes; MinInt64 also uses 20 bytes
	// including its sign.
	canonicalDecimalMaximumBytes = 20
)

// canonicalNameSpan locates one already-encoded member name inside the object's
// buffer. Storing the span rather than a second copy of the name keeps
// duplicate detection free of per-member allocation.
type canonicalNameSpan struct {
	start uint32
	end   uint32
}

// CanonicalObject appends exactly one canonical JSON object to a caller-owned
// buffer. It is the mechanic a CanonicalBody uses to emit the precise bytes a
// signature covers: member order is the caller's writing order, so one typed
// value always produces one document.
//
// This is deliberately not a general JSON canonicalizer. It serves closed
// structs whose member set is fixed at compile time, and every member arrives
// through a typed method rather than a reflected Go value, so no signing path
// depends on struct tags, declaration order, or map iteration order.
//
// Errors are threaded. A caller writes one straight-line sequence of members
// and checks once at End; the first failure is retained and every later member
// becomes a no-op, so a rejected document is never returned partially built.
//
// The zero value is not usable. Construct with BeginCanonicalObject.
type CanonicalObject struct {
	err    error
	buffer []byte
	names  [CanonicalObjectMaximumFields]canonicalNameSpan
	count  uint16
	opened bool
	closed bool
}

// BeginCanonicalObject starts one canonical object at the end of destination.
// The caller keeps ownership of the backing array. Scalar members use fixed
// stack storage; string and nested-value owners may allocate while producing
// their independently validated canonical encodings. String input is admitted
// against the remaining object extent before encoding, so that allocation is
// bounded by CanonicalBodyMaximumBytes.
func BeginCanonicalObject(destination []byte) CanonicalObject {
	object := CanonicalObject{buffer: destination, opened: true}
	if len(destination) >= CanonicalBodyMaximumBytes {
		object.err = contractError(errors.New(canonicalObjectExtentErrorText))
		return object
	}
	object.buffer = append(object.buffer, canonicalObjectOpen)
	return object
}

// String appends one canonical JSON string member.
func (o *CanonicalObject) String(name string, value string) {
	span, ok := o.beginMember(name)
	if !ok {
		return
	}
	// Quotes are the smallest possible JSON string overhead. Escaping can only
	// increase it, so an input that fails this check cannot fit in the object.
	if len(value) > CanonicalBodyMaximumBytes-len(o.buffer)-2 {
		o.fail(errors.New(canonicalObjectExtentErrorText))
		return
	}
	encoded, err := core.MarshalCanonicalJSONString(value)
	if err != nil {
		o.fail(err)
		return
	}
	o.appendEncodedMember(span, encoded)
}

// Int64 appends the one signed decimal spelling emitted by strconv.AppendInt.
func (o *CanonicalObject) Int64(name string, value int64) {
	span, ok := o.beginMember(name)
	if !ok {
		return
	}
	var storage [canonicalDecimalMaximumBytes]byte
	encoded := strconv.AppendInt(storage[:0], value, canonicalDecimalBase)
	o.appendEncodedMember(span, encoded)
}

// Uint64 appends the one unsigned decimal spelling emitted by strconv.AppendUint.
func (o *CanonicalObject) Uint64(name string, value uint64) {
	span, ok := o.beginMember(name)
	if !ok {
		return
	}
	var storage [canonicalDecimalMaximumBytes]byte
	encoded := strconv.AppendUint(storage[:0], value, canonicalDecimalBase)
	o.appendEncodedMember(span, encoded)
}

// Bool appends one canonical JSON boolean member.
func (o *CanonicalObject) Bool(name string, value bool) {
	span, ok := o.beginMember(name)
	if !ok {
		return
	}
	if value {
		o.appendEncodedMember(span, []byte(canonicalTrueText))
	} else {
		o.appendEncodedMember(span, []byte(canonicalFalseText))
	}
}

// Value appends one nested member whose owner supplies both its invariant and
// its exact JSON projection. The owner runs under a panic guard and its output
// must be non-null valid JSON, so a hostile or defective member cannot escape
// as signed bytes or replace this package's error identities.
func (o *CanonicalObject) Value(name string, value core.ValidatedJSONMarshaler) {
	span, ok := o.beginMember(name)
	if !ok {
		return
	}
	encoded, err := marshalCanonicalMember(value)
	if err != nil {
		o.fail(err)
		return
	}
	o.appendEncodedMember(span, encoded)
}

// End closes the object and returns the complete buffer. It reports the first
// threaded failure and refuses an object with no members, because an empty
// signed body carries no fact.
func (o *CanonicalObject) End() ([]byte, error) {
	if !o.opened {
		return nil, contractError(errors.New(canonicalObjectUnopenedErrorText))
	}
	if o.closed {
		return nil, contractError(errors.New(canonicalObjectClosedErrorText))
	}
	o.closed = true
	if o.err != nil {
		return nil, o.err
	}
	if o.count == 0 {
		return nil, contractError(errors.New(canonicalObjectEmptyErrorText))
	}
	if len(o.buffer) >= CanonicalBodyMaximumBytes {
		return nil, contractError(errors.New(canonicalObjectExtentErrorText))
	}
	o.buffer = append(o.buffer, canonicalObjectClose)
	return o.buffer, nil
}

// ready reports whether another member may be written, recording the reason it
// may not. It is the single admission gate every member method passes through.
func (o *CanonicalObject) ready() bool {
	if o.err != nil {
		return false
	}
	if !o.opened || o.closed {
		o.fail(errors.New(canonicalObjectClosedErrorText))
		return false
	}
	return true
}

func (o *CanonicalObject) fail(err error) {
	if o.err == nil {
		o.err = contractError(err)
	}
}

func (o *CanonicalObject) appendEncodedMember(span canonicalNameSpan, encoded []byte) {
	if len(encoded) > CanonicalBodyMaximumBytes-len(o.buffer) {
		o.fail(errors.New(canonicalObjectExtentErrorText))
		return
	}
	o.buffer = append(o.buffer, encoded...)
	o.recordMember(span)
}

func (o *CanonicalObject) beginMember(name string) (canonicalNameSpan, bool) {
	if !o.ready() {
		return canonicalNameSpan{}, false
	}
	if int(o.count) >= CanonicalObjectMaximumFields {
		o.fail(errors.New(canonicalObjectFieldCountErrorText))
		return canonicalNameSpan{}, false
	}
	if err := validateCanonicalFieldName(name); err != nil {
		o.fail(err)
		return canonicalNameSpan{}, false
	}
	prefixLength := len(name) + 3 // two name quotes and one colon
	if o.count > 0 {
		prefixLength++ // member comma
	}
	if prefixLength > CanonicalBodyMaximumBytes-len(o.buffer) {
		o.fail(errors.New(canonicalObjectExtentErrorText))
		return canonicalNameSpan{}, false
	}
	if o.count > 0 {
		o.buffer = append(o.buffer, canonicalMemberComma)
	}
	span, err := o.appendName(name)
	if err != nil {
		o.fail(err)
		return canonicalNameSpan{}, false
	}
	if o.duplicateName(span) {
		o.fail(errors.New(canonicalObjectDuplicateErrorText))
		return canonicalNameSpan{}, false
	}
	o.buffer = append(o.buffer, canonicalMemberColon)
	return span, true
}

func (o *CanonicalObject) appendName(name string) (canonicalNameSpan, error) {
	start, err := core.CheckedUint32FromInt(len(o.buffer))
	if err != nil {
		return canonicalNameSpan{}, err
	}
	o.buffer = append(o.buffer, canonicalNameQuote)
	o.buffer = append(o.buffer, name...)
	o.buffer = append(o.buffer, canonicalNameQuote)
	end, err := core.CheckedUint32FromInt(len(o.buffer))
	if err != nil {
		return canonicalNameSpan{}, err
	}
	return canonicalNameSpan{start: start, end: end}, nil
}

func (o *CanonicalObject) recordMember(span canonicalNameSpan) {
	o.names[o.count] = span
	o.count++
}

func (o *CanonicalObject) duplicateName(candidate canonicalNameSpan) bool {
	name := o.buffer[candidate.start:candidate.end]
	for index := range int(o.count) {
		existing := o.names[index]
		if bytes.Equal(o.buffer[existing.start:existing.end], name) {
			return true
		}
	}
	return false
}

// validateCanonicalFieldName owns the member-name grammar. Names are lowercase
// ASCII words joined by single separators, so an encoded name never needs an
// escape and two names can never collide only by case folding, which is the
// shape strict JSON decoding refuses.
func validateCanonicalFieldName(name string) error {
	if len(name) == 0 || len(name) > CanonicalFieldNameMaximumBytes {
		return errors.New(canonicalFieldNameExtentErrorText)
	}
	if name[0] == CanonicalFieldNameSeparator ||
		name[len(name)-1] == CanonicalFieldNameSeparator {
		return errors.New(canonicalFieldNameGrammarErrorText)
	}
	previousSeparator := false
	for index := range len(name) {
		value := name[index]
		if !canonicalFieldNameByte(value) {
			return errors.New(canonicalFieldNameGrammarErrorText)
		}
		if value == CanonicalFieldNameSeparator && previousSeparator {
			return errors.New(canonicalFieldNameGrammarErrorText)
		}
		previousSeparator = value == CanonicalFieldNameSeparator
	}
	return nil
}

func canonicalFieldNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == CanonicalFieldNameSeparator
}

func marshalCanonicalMember(value core.ValidatedJSONMarshaler) ([]byte, error) {
	if value == nil {
		return nil, errors.New(canonicalMemberMissingErrorText)
	}
	encoded, err := guardedCall(func() ([]byte, error) {
		if validateErr := value.Validate(); validateErr != nil {
			return nil, validateErr
		}
		return value.MarshalJSON()
	})
	if err != nil {
		return nil, err
	}
	return encoded, validateCanonicalMemberEncoding(encoded)
}

func validateCanonicalMemberEncoding(encoded []byte) error {
	if len(encoded) == 0 || len(encoded) > CanonicalBodyMaximumBytes {
		return errors.New(canonicalMemberExtentErrorText)
	}
	if !utf8.Valid(encoded) {
		return errors.New(canonicalMemberEncodingErrorText)
	}
	limits := core.DefaultStrictJSONLimits()
	// doctrine:local-allowed=external-wire
	if _, err := core.DecodeStrictJSONStructure[json.RawMessage](encoded, limits); err != nil {
		return errors.Join(errors.New(canonicalMemberEncodingErrorText), err)
	}
	if bytes.Equal(bytes.TrimSpace(encoded), []byte(canonicalMemberNullText)) {
		return errors.New(canonicalMemberNullErrorText)
	}
	return nil
}
