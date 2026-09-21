package runnercontrol

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"hash"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// This is the flat scalar grammar of cmd/go events, not a general JSON runtime.
// Go's JSON decoder validates/unescapes each complete scalar string fragment.
// Only identity digests and mechanical accounting projections survive a fragment.
type GoEventField uint8

const (
	GoEventFieldUnknown GoEventField = iota
	GoEventFieldAction
	GoEventFieldPackage
	GoEventFieldTest
	GoEventFieldOutput
	GoEventFieldOutputType
	GoEventFieldTime
	GoEventFieldFailedBuild
	GoEventFieldElapsed
	GoEventFieldImportPath
	GoEventFieldKey
	GoEventFieldValue
	GoEventFieldPath
)

type goJSONState uint8

const (
	goJSONStart goJSONState = iota
	goJSONKey
	goJSONColon
	goJSONValue
	goJSONString
	goJSONAfterValue
	goJSONNextKey
	goJSONEnd
	goJSONNumber
)

type goJSONProjection struct {
	benchmark      goBenchmarkStream
	fields         uint16
	packageID      [sha256.Size]byte
	action         GoEventAction
	outputType     GoEventOutputKind
	packagePresent bool
	testPresent    bool
	outputPresent  bool
}

type goJSONStream struct {
	observeString func(GoEventStringFragment) error
	digest        hash.Hash
	projection    goJSONProjection
	fragment      [256]byte
	scalar        [16]byte
	used          int
	scalarUsed    int
	field         GoEventField
	state         goJSONState
	key           bool
	escaped       bool
	unicodeDigits uint8
	unicodeValue  uint16
	highSurrogate bool
	numberState   uint8
	number        goJSONFloat
}

func (goJSONProjection) runnerControlInternalFlow()  {}
func (goJSONStream) runnerControlCapabilityWrapper() {}

func (s *goJSONStream) consume(value byte, emit func(goJSONProjection) error) error {
	if s.state == goJSONString {
		return s.stringByte(value)
	}
	if s.state == goJSONNumber {
		if !goJSONNumberDelimiter(value) {
			return s.numberByte(value)
		}
		if err := s.endNumber(); err != nil {
			return err
		}
	}
	if value == '\n' {
		if s.state != goJSONEnd {
			return goJSONFailure()
		}
		s.state = goJSONStart
		return emit(s.projection)
	}
	if value == ' ' || value == '\r' || value == '\t' {
		return nil
	}
	return s.consumeStructure(value)
}

func goJSONNumberDelimiter(value byte) bool {
	switch value {
	case ',', '}', ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func (s *goJSONStream) endNumber() error {
	if s.numberState != 2 && s.numberState != 3 && s.numberState != 5 && s.numberState != 8 {
		return goJSONFailure()
	}
	if err := s.number.validate(); err != nil {
		return err
	}
	s.state = goJSONAfterValue
	return nil
}

func (s *goJSONStream) consumeStructure(value byte) error {
	switch s.state {
	case goJSONStart:
		if value != '{' {
			return goJSONFailure()
		}
		s.projection = goJSONProjection{}
		if s.digest == nil {
			s.digest = sha256.New()
		} else {
			s.digest.Reset()
		}
		s.state = goJSONKey
	case goJSONKey, goJSONNextKey:
		return s.beginKey(value)
	case goJSONColon:
		if value != ':' {
			return goJSONFailure()
		}
		s.state = goJSONValue
	case goJSONValue:
		return s.beginValue(value)
	case goJSONAfterValue:
		return s.endValue(value)
	default:
		return goJSONFailure()
	}
	return nil
}

func (s *goJSONStream) beginKey(value byte) error {
	if value == '}' && s.state == goJSONKey {
		s.state = goJSONEnd
		return nil
	}
	if value != '"' {
		return goJSONFailure()
	}
	s.startString(true)
	return nil
}

func (s *goJSONStream) beginValue(value byte) error {
	if s.field == GoEventFieldElapsed {
		s.numberState = 0
		s.number = goJSONFloat{}
		s.state = goJSONNumber
		return s.numberByte(value)
	}
	if value != '"' {
		return goJSONFailure()
	}
	s.startString(false)
	return nil
}

func (s *goJSONStream) endValue(value byte) error {
	switch value {
	case ',':
		s.state = goJSONNextKey
	case '}':
		s.state = goJSONEnd
	default:
		return goJSONFailure()
	}
	return nil
}

func (s *goJSONStream) numberByte(value byte) error {
	s.number.consume(value)
	switch s.numberState {
	case 0, 1:
		return s.numberIntegerStart(value)
	case 2, 3:
		return s.numberIntegerDigit(value)
	case 4, 5:
		return s.numberFractionDigit(value)
	case 6, 7:
		return s.numberExponentStart(value)
	case 8:
		if value >= '0' && value <= '9' {
			return nil
		}
	}
	return goJSONFailure()
}

func (s *goJSONStream) numberIntegerStart(value byte) error {
	if value == '-' && s.numberState == 0 {
		s.numberState = 1
		return nil
	}
	if value == '0' {
		s.numberState = 2
		return nil
	}
	if value >= '1' && value <= '9' {
		s.numberState = 3
		return nil
	}
	return goJSONFailure()
}

func (s *goJSONStream) numberIntegerDigit(value byte) error {
	if value >= '0' && value <= '9' && s.numberState == 3 {
		return nil
	}
	if value == '.' {
		s.numberState = 4
		return nil
	}
	if value == 'e' || value == 'E' {
		s.numberState = 6
		return nil
	}
	return goJSONFailure()
}

func (s *goJSONStream) numberFractionDigit(value byte) error {
	if value >= '0' && value <= '9' {
		s.numberState = 5
		return nil
	}
	if s.numberState == 5 && (value == 'e' || value == 'E') {
		s.numberState = 6
		return nil
	}
	return goJSONFailure()
}

func (s *goJSONStream) numberExponentStart(value byte) error {
	if s.numberState == 6 && (value == '+' || value == '-') {
		s.numberState = 7
		return nil
	}
	if value >= '0' && value <= '9' {
		s.numberState = 8
		return nil
	}
	return goJSONFailure()
}

func (s *goJSONStream) startString(key bool) {
	s.key, s.escaped, s.highSurrogate = key, false, false
	s.unicodeDigits, s.unicodeValue = 0, 0
	s.used, s.scalarUsed = 1, 0
	s.fragment[0] = '"'
	s.state = goJSONString
}

func (s *goJSONStream) stringByte(value byte) error {
	if value < 0x20 {
		return goJSONFailure()
	}
	if value == '"' && !s.escaped && s.unicodeDigits == 0 {
		return s.finishString()
	}
	if s.used >= len(s.fragment)-1 {
		return goJSONFailure()
	}
	s.fragment[s.used] = value
	s.used++
	if err := s.advanceStringEscape(value); err != nil {
		return err
	}

	if s.stringFragmentReady() {
		return s.flushString()
	}
	return nil
}

func (s *goJSONStream) stringFragmentReady() bool {
	return s.used >= 192 && !s.escaped && s.unicodeDigits == 0 && !s.highSurrogate && utf8.Valid(s.fragment[1:s.used])
}

func (s *goJSONStream) advanceStringEscape(value byte) error {
	switch {
	case s.unicodeDigits > 0:
		return s.consumeUnicodeDigit(value)
	case s.escaped:
		s.escaped = false
		if value == 'u' {
			s.unicodeDigits, s.unicodeValue = 4, 0
		} else if s.highSurrogate {
			return goJSONFailure()
		}
	case value == '\\':
		s.escaped = true
	case s.highSurrogate:
		return goJSONFailure()
	}
	return nil
}

func (s *goJSONStream) consumeUnicodeDigit(value byte) error {
	digit, err := goJSONHexDigit(value)
	if err != nil {
		return err
	}

	s.unicodeValue = s.unicodeValue*16 + digit
	s.unicodeDigits--
	if s.unicodeDigits == 0 {
		if s.highSurrogate {
			if s.unicodeValue < 0xdc00 || s.unicodeValue > 0xdfff {
				return goJSONFailure()
			}
			s.highSurrogate = false
		} else if s.unicodeValue >= 0xd800 && s.unicodeValue <= 0xdbff {
			s.highSurrogate = true
		}
	}
	return nil
}

func goJSONHexDigit(value byte) (uint16, error) {
	var digit uint16
	switch {
	case value >= '0' && value <= '9':
		digit = uint16(value - '0')
	case value >= 'a' && value <= 'f':
		digit = uint16(value - 'a' + 10)
	case value >= 'A' && value <= 'F':
		digit = uint16(value - 'A' + 10)
	default:
		return 0, goJSONFailure()
	}
	return digit, nil
}

func (s *goJSONStream) finishString() error {
	if s.highSurrogate {
		return goJSONFailure()
	}
	if err := s.flushString(); err != nil {
		return err
	}
	return s.endString()
}

func (s *goJSONStream) flushString() error {
	s.fragment[s.used] = '"'
	var decoded string
	if err := json.Unmarshal(s.fragment[:s.used+1], &decoded); err != nil {
		return observationFailure("go event string cannot be decoded", core.ErrJSONContract, err)
	}
	s.used = 1
	if !s.key && s.observeString != nil {
		if err := s.observeString(GoEventStringFragment{Field: s.field, Data: []byte(decoded)}); err != nil {
			return err
		}
	}
	return s.retainString(decoded)
}

func (s *goJSONStream) retainString(decoded string) error {
	if s.key || s.field == GoEventFieldAction || s.field == GoEventFieldOutputType {
		if len(decoded) > len(s.scalar)-s.scalarUsed {
			return goJSONFailure()
		}
		copy(s.scalar[s.scalarUsed:], decoded)
		s.scalarUsed += len(decoded)
		return nil
	}
	return s.projectString(decoded)
}

func (s *goJSONStream) projectString(decoded string) error {
	switch s.field {
	case GoEventFieldPackage:
		s.projection.packagePresent = s.projection.packagePresent || decoded != ""
		if _, err := s.digest.Write([]byte(decoded)); err != nil {
			return err
		}
	case GoEventFieldTest:
		s.projection.testPresent = s.projection.testPresent || decoded != ""
	case GoEventFieldOutput:
		s.projection.outputPresent = s.projection.outputPresent || decoded != ""
		s.projection.benchmark.write(decoded)
	}
	return nil
}

func (s *goJSONStream) endString() error {
	if s.key {
		return s.endKey()
	}
	switch s.field {
	case GoEventFieldAction:
		action, err := decodeGoEventAction(string(s.scalar[:s.scalarUsed]))
		if err != nil {
			return err
		}
		s.projection.action = action
	case GoEventFieldOutputType:
		kind, err := decodeGoOutputKind(string(s.scalar[:s.scalarUsed]))
		if err != nil {
			return err
		}
		s.projection.outputType = kind
	case GoEventFieldPackage:
		s.digest.Sum(s.projection.packageID[:0])
	case GoEventFieldOutput:
		s.projection.benchmark.endToken()
	}
	if s.observeString != nil {
		if err := s.observeString(GoEventStringFragment{Field: s.field, Final: true}); err != nil {
			return err
		}
	}
	s.state = goJSONAfterValue
	return nil
}

func (s *goJSONStream) endKey() error {
	field, err := decodeGoEventField(string(s.scalar[:s.scalarUsed]))
	if err != nil {
		return err
	}
	s.field = field

	mask := uint16(1) << s.field
	if s.projection.fields&mask != 0 {
		return goJSONFailure()
	}
	s.projection.fields |= mask
	s.state = goJSONColon
	return nil
}

func (s *goJSONStream) finish(emit func(goJSONProjection) error) error {
	if s.state == goJSONStart {
		return nil
	}
	if s.state != goJSONEnd {
		return goJSONFailure()
	}
	s.state = goJSONStart
	return emit(s.projection)
}

func goJSONFailure() error {
	return observationFailure("go event violates its flat JSON wire grammar", core.ErrJSONContract)
}
