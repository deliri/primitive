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
type goJSONField uint8

const (
	goJSONNoField goJSONField = iota
	goJSONAction
	goJSONPackage
	goJSONTest
	goJSONOutput
	goJSONOutputType
	goJSONTime
	goJSONFailedBuild
	goJSONElapsed
	goJSONImportPath
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
	action         goEventAction
	outputType     goOutputKind
	packageID      [sha256.Size]byte
	packagePresent bool
	testPresent    bool
	outputPresent  bool
	fields         uint16
	benchmark      goBenchmarkStream
}

type goJSONStream struct {
	digest        hash.Hash
	projection    goJSONProjection
	fragment      [256]byte
	scalar        [16]byte
	used          int
	scalarUsed    int
	field         goJSONField
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
		if value != ',' && value != '}' && value != ' ' && value != '\t' && value != '\r' && value != '\n' {
			return s.numberByte(value)
		}
		if s.numberState != 2 && s.numberState != 3 && s.numberState != 5 && s.numberState != 8 {
			return goJSONFailure()
		}
		if err := s.number.validate(); err != nil {
			return err
		}
		s.state = goJSONAfterValue
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
		if value == '}' && s.state == goJSONKey {
			s.state = goJSONEnd
			return nil
		}
		if value != '"' {
			return goJSONFailure()
		}
		s.startString(true)
	case goJSONColon:
		if value != ':' {
			return goJSONFailure()
		}
		s.state = goJSONValue
	case goJSONValue:
		if s.field == goJSONElapsed {
			s.numberState = 0
			s.number = goJSONFloat{}
			s.state = goJSONNumber
			return s.numberByte(value)
		}
		if value != '"' {
			return goJSONFailure()
		}
		s.startString(false)
	case goJSONAfterValue:
		switch value {
		case ',':
			s.state = goJSONNextKey
		case '}':
			s.state = goJSONEnd
		default:
			return goJSONFailure()
		}
	default:
		return goJSONFailure()
	}
	return nil
}

func (s *goJSONStream) numberByte(value byte) error {
	s.number.consume(value)
	digit := value >= '0' && value <= '9'
	switch s.numberState {
	case 0, 1:
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
	case 2, 3:
		if digit && s.numberState == 3 {
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
	case 4, 5:
		if digit {
			s.numberState = 5
			return nil
		}
		if s.numberState == 5 && (value == 'e' || value == 'E') {
			s.numberState = 6
			return nil
		}
	case 6, 7:
		if s.numberState == 6 && (value == '+' || value == '-') {
			s.numberState = 7
			return nil
		}
		if digit {
			s.numberState = 8
			return nil
		}
	case 8:
		if digit {
			return nil
		}
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
		if s.highSurrogate {
			return goJSONFailure()
		}
		if err := s.flushString(); err != nil {
			return err
		}
		return s.endString()
	}
	if s.used >= len(s.fragment)-1 {
		return goJSONFailure()
	}
	s.fragment[s.used] = value
	s.used++
	switch {
	case s.unicodeDigits > 0:
		var digit uint16
		switch {
		case value >= '0' && value <= '9':
			digit = uint16(value - '0')
		case value >= 'a' && value <= 'f':
			digit = uint16(value - 'a' + 10)
		case value >= 'A' && value <= 'F':
			digit = uint16(value - 'A' + 10)
		default:
			return goJSONFailure()
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
	if s.used >= 192 && !s.escaped && s.unicodeDigits == 0 && !s.highSurrogate && utf8.Valid(s.fragment[1:s.used]) {
		return s.flushString()
	}
	return nil
}

func (s *goJSONStream) flushString() error {
	s.fragment[s.used] = '"'
	var decoded string
	if err := json.Unmarshal(s.fragment[:s.used+1], &decoded); err != nil {
		return observationFailure("go event string cannot be decoded", core.ErrJSONContract, err)
	}
	s.used = 1
	if s.key || s.field == goJSONAction || s.field == goJSONOutputType {
		if len(decoded) > len(s.scalar)-s.scalarUsed {
			return goJSONFailure()
		}
		copy(s.scalar[s.scalarUsed:], decoded)
		s.scalarUsed += len(decoded)
		return nil
	}
	switch s.field {
	case goJSONPackage:
		s.projection.packagePresent = s.projection.packagePresent || decoded != ""
		if _, err := s.digest.Write([]byte(decoded)); err != nil {
			return err
		}
	case goJSONTest:
		s.projection.testPresent = s.projection.testPresent || decoded != ""
	case goJSONOutput:
		s.projection.outputPresent = s.projection.outputPresent || decoded != ""
		s.projection.benchmark.write(decoded)
	}
	return nil
}

func (s *goJSONStream) endString() error {
	if s.key {
		switch string(s.scalar[:s.scalarUsed]) {
		case "Action":
			s.field = goJSONAction
		case "Package":
			s.field = goJSONPackage
		case "Test":
			s.field = goJSONTest
		case "Output":
			s.field = goJSONOutput
		case "OutputType":
			s.field = goJSONOutputType
		case "Time":
			s.field = goJSONTime
		case "FailedBuild":
			s.field = goJSONFailedBuild
		case "Elapsed":
			s.field = goJSONElapsed
		case "ImportPath":
			s.field = goJSONImportPath
		default:
			return goJSONFailure()
		}
		mask := uint16(1) << s.field
		if s.projection.fields&mask != 0 {
			return goJSONFailure()
		}
		s.projection.fields |= mask
		s.state = goJSONColon
		return nil
	}
	switch s.field {
	case goJSONAction:
		action, err := decodeGoEventAction(string(s.scalar[:s.scalarUsed]))
		if err != nil {
			return err
		}
		s.projection.action = action
	case goJSONOutputType:
		kind, err := decodeGoOutputKind(string(s.scalar[:s.scalarUsed]))
		if err != nil {
			return err
		}
		s.projection.outputType = kind
	case goJSONPackage:
		s.digest.Sum(s.projection.packageID[:0])
	case goJSONOutput:
		s.projection.benchmark.endToken()
	}
	s.state = goJSONAfterValue
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
