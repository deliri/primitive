package runnercontrol

import (
	"errors"
	"io"
	"math"
	"math/bits"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

type GoCoverageObservation struct {
	Statements  uint64       `json:"statements"`
	Covered     uint64       `json:"covered"`
	BasisPoints uint16       `json:"basis_points"`
	Mode        CoverageMode `json:"mode"`
}

func (o GoCoverageObservation) Validate() error {
	if err := o.Mode.Validate(); err != nil {
		return err
	}
	if o.Statements == 0 || o.Covered > o.Statements || o.BasisPoints > 10_000 {
		return core.ErrPrimitiveContract
	}
	if uint64(o.BasisPoints) != coverageBasisPoints(o.Covered, o.Statements) {
		return core.ErrPrimitiveContract
	}
	return nil
}

// GoCoverageCompiler folds fragments without retaining source locations or lines.
// The only byte buffers hold the closed mode spelling and one UTF-8 rune.
type GoCoverageCompiler struct {
	failure          error
	statements       uint64
	covered          uint64
	recordStatements uint64
	recordCount      uint64
	header           [12]byte
	runeBytes        [utf8.UTFMax]byte
	mode             CoverageMode
	headerLength     uint8
	runeLength       uint8
	fields           uint8
	inField          bool
	locationColon    bool
	linePresent      bool
}

func NewGoCoverageCompiler() *GoCoverageCompiler { return &GoCoverageCompiler{} }

func (c *GoCoverageCompiler) Write(data []byte) (int, error) {
	if c == nil {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New(goCoverageCompilerNilDiagnostic))
	}
	if c.failure != nil {
		return 0, c.failure
	}
	for _, value := range data {
		// Source locations are observed only for their separator. Skip ordinary
		// ASCII bytes directly; whitespace and UTF-8 still take the lexical path.
		if c.inField && c.fields == 1 && c.runeLength == 0 && value > ' ' && value < utf8.RuneSelf {
			c.locationColon = c.locationColon || value == ':'
			continue
		}
		if c.runeLength == 0 && value < utf8.RuneSelf {
			if err := c.consumeRune(rune(value)); err != nil {
				c.failure = err
				return len(data), nil
			}
			continue
		}
		c.runeBytes[c.runeLength] = value
		c.runeLength++
		if !utf8.FullRune(c.runeBytes[:c.runeLength]) {
			continue
		}
		if err := c.drainRunes(false); err != nil {
			c.failure = err
			// The capture writer can retain this complete input chunk. Seal refuses it.
			return len(data), nil
		}
	}
	return len(data), nil
}

func (c *GoCoverageCompiler) drainRunes(final bool) error {
	for c.runeLength > 0 && (final || utf8.FullRune(c.runeBytes[:c.runeLength])) {
		value, size := utf8.DecodeRune(c.runeBytes[:c.runeLength])
		copy(c.runeBytes[:], c.runeBytes[size:c.runeLength])
		c.runeLength -= uint8(size)
		if err := c.consumeRune(value); err != nil {
			return err
		}
	}
	return nil
}

func (c *GoCoverageCompiler) consumeRune(value rune) error {
	if value == '\n' {
		return c.finishLine()
	}
	c.linePresent = true
	if c.mode == CoverageModeUnknown {
		if value > unicode.MaxASCII || int(c.headerLength) == len(c.header) {
			return coverageFailure("go coverage mode is outside the admitted domain")
		}
		c.header[c.headerLength] = byte(value)
		c.headerLength++
		return nil
	}
	if unicode.IsSpace(value) {
		c.inField = false
		return nil
	}
	if !c.inField {
		c.fields++
		c.inField = true
	}
	switch c.fields {
	case 1:
		c.locationColon = c.locationColon || value == ':'
		return nil
	case 2:
		return appendCoverageDigit(&c.recordStatements, value, math.MaxUint32)
	case 3:
		return appendCoverageDigit(&c.recordCount, value, math.MaxUint64)
	default:
		return coverageFailure("go coverage record must contain exactly three fields")
	}
}

func appendCoverageDigit(number *uint64, value rune, maximum uint64) error {
	if value < '0' || value > '9' {
		return coverageFailure("go coverage record contains a nondecimal count")
	}
	digit := uint64(value - '0')
	if *number > (maximum-digit)/10 {
		return errors.Join(core.ErrNumericOverflow, coverageFailure("go coverage count exceeds its native representation"))
	}
	*number = *number*10 + digit
	return nil
}

func (c *GoCoverageCompiler) finishLine() error {
	if c.mode == CoverageModeUnknown {
		mode, err := parseCoverageMode(string(c.header[:c.headerLength]))
		if err != nil {
			return err
		}
		c.mode = mode
	} else {
		if c.fields != 3 || !c.locationColon || c.recordStatements == 0 {
			return coverageFailure("go coverage record has invalid numeric or location facts")
		}
		if err := c.accumulateCoverage(c.recordStatements, c.recordCount); err != nil {
			return err
		}
	}
	c.linePresent, c.inField, c.locationColon = false, false, false
	c.fields, c.recordStatements, c.recordCount = 0, 0, 0
	return nil
}

func (c *GoCoverageCompiler) accumulateCoverage(statements, count uint64) error {
	if math.MaxUint64-c.statements < statements {
		return errors.Join(core.ErrNumericOverflow, coverageFailure("go coverage statement total overflows"))
	}
	c.statements += statements
	if count > 0 {
		if math.MaxUint64-c.covered < statements {
			return errors.Join(core.ErrNumericOverflow, coverageFailure("go coverage covered total overflows"))
		}
		c.covered += statements
	}
	return nil
}

func (c *GoCoverageCompiler) Seal() (GoCoverageObservation, error) {
	if c == nil {
		return GoCoverageObservation{}, errors.Join(core.ErrPrimitiveContract, errors.New(goCoverageCompilerNilDiagnostic))
	}
	if c.failure != nil {
		return GoCoverageObservation{}, c.failure
	}
	if err := c.drainRunes(true); err != nil {
		c.failure = err
		return GoCoverageObservation{}, err
	}
	if c.linePresent {
		if err := c.finishLine(); err != nil {
			c.failure = err
			return GoCoverageObservation{}, err
		}
	}
	if c.mode == CoverageModeUnknown || c.statements == 0 {
		return GoCoverageObservation{}, coverageFailure("go coverage stream has no statement evidence")
	}
	basisPoints, err := checkedUint16FromUint64(coverageBasisPoints(c.covered, c.statements))
	if err != nil {
		return GoCoverageObservation{}, coverageFailure("go coverage basis points exceed the numeric ceiling")
	}
	result := GoCoverageObservation{Mode: c.mode, Statements: c.statements, Covered: c.covered, BasisPoints: basisPoints}
	return result, result.Validate()
}

func parseCoverageMode(line string) (CoverageMode, error) {
	if !strings.HasPrefix(line, coverageModePrefix) {
		return CoverageModeUnknown, coverageFailure("go coverage stream is missing its canonical mode header")
	}
	value := strings.TrimPrefix(line, coverageModePrefix)
	for mode := CoverageModeUnknown + 1; mode < coverageModeLimit; mode++ {
		if mode.String() == value {
			return mode, nil
		}
	}
	return CoverageModeUnknown, coverageFailure("go coverage mode is outside the admitted domain")
}

func coverageFailure(message string) error {
	return errors.Join(core.ErrPrimitiveContract, errors.New(message))
}

var _ io.Writer = (*GoCoverageCompiler)(nil)

// The quotient is at most 10000 because covered <= statements. A wide
// intermediate avoids imposing a smaller total-coverage ceiling on uint64.
func coverageBasisPoints(covered, statements uint64) uint64 {
	high, low := bits.Mul64(covered, 10000)
	quotient, _ := bits.Div64(high, low, statements)
	return quotient
}
