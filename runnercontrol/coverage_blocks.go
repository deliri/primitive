package runnercontrol

import (
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// CoverageBlock contains only the numeric facts needed by a policy owner.
// Locations are checked and discarded. Zero counts are legitimate observations.
type CoverageBlock struct {
	Statements int64
	Hits       int64
	Mode       CoverageMode
}

func (b CoverageBlock) Validate() error {
	if err := b.Mode.Validate(); err != nil {
		return err
	}
	if b.Statements < 0 || b.Hits < 0 || (b.Mode == CoverageSet && b.Hits > 1) {
		return core.ErrPrimitiveContract
	}
	return nil
}

type CoverageBlockRequest struct{ Source io.Reader }

func (r CoverageBlockRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrLineIOContract
	}
	return nil
}

// CoverageBlocks reads fragments with fixed working memory, independent of
// line, location and file extent. Observations are provisional until clean EOF.
// It does not compute product totals or decide whether coverage is sufficient.
func CoverageBlocks(ctx context.Context, request CoverageBlockRequest) iter.Seq2[CoverageBlock, error] {
	return func(yield func(CoverageBlock, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(CoverageBlock{}, err)
			return
		}
		capacity, err := core.NewByteCount(4096)
		if err != nil {
			yield(CoverageBlock{}, err)
			return
		}
		reader, err := lineio.New(lineio.Request{Source: request.Source, BufferBytes: capacity})
		if err != nil {
			yield(CoverageBlock{}, err)
			return
		}
		input := coverageInput{ctx: ctx, reader: reader}
		mode, err := input.header()
		if err != nil {
			yield(CoverageBlock{}, err)
			return
		}
		for {
			first, err := input.nonspace()
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(CoverageBlock{}, err)
				return
			}
			block, err := input.block(first, mode)
			if err != nil {
				yield(CoverageBlock{}, err)
				return
			}
			if !yield(block, nil) {
				return
			}
		}
	}
}

// The cursor borrows one native fragment. No line or token is collected.
type coverageInput struct {
	ctx      context.Context
	reader   *lineio.Reader
	fragment lineio.Fragment
	offset   int
	terminal error
}

func (r *coverageInput) byte() (byte, error) {
	if r.offset == len(r.fragment.Bytes) {
		if err := contextstate.Validate(r.ctx); err != nil {
			return 0, err
		}
		if r.terminal != nil {
			return 0, r.terminal
		}
		r.fragment, r.terminal = r.reader.ReadFragment()
		r.offset = 0
		if len(r.fragment.Bytes) == 0 {
			return 0, r.terminal
		}
	}
	value := r.fragment.Bytes[r.offset]
	r.offset++
	return value, nil
}
func (r *coverageInput) rune() (rune, error) {
	first, err := r.byte()
	if err != nil {
		return 0, err
	}
	if first == 0 {
		return 0, core.ErrPrimitiveContract
	}
	if first < utf8.RuneSelf {
		return rune(first), nil
	}
	var encoded [utf8.UTFMax]byte
	encoded[0] = first
	size := 1
	for !utf8.FullRune(encoded[:size]) {
		value, err := r.byte()
		if err != nil {
			return 0, errors.Join(core.ErrPrimitiveContract, err)
		}
		encoded[size] = value
		size++
	}
	value, width := utf8.DecodeRune(encoded[:size])
	if value == utf8.RuneError && width == 1 {
		return 0, core.ErrPrimitiveContract
	}
	return value, nil
}
func (r *coverageInput) nonspace() (rune, error) {
	for {
		value, err := r.rune()
		if err != nil || !unicode.IsSpace(value) {
			return value, err
		}
	}
}
func (r *coverageInput) header() (CoverageMode, error) {
	first, err := r.nonspace()
	if err != nil {
		return CoverageModeUnknown, errors.Join(core.ErrPrimitiveContract, err)
	}
	var header [len(coverageModePrefix) + len("atomic")]byte
	used, last := 0, 0
	value := first
	for {
		if value == '\n' {
			break
		}
		if !unicode.IsSpace(value) && (value > unicode.MaxASCII || used == len(header)) {
			return CoverageModeUnknown, core.ErrPrimitiveContract
		}
		if used < len(header) {
			header[used] = byte(value)
			used++
		}
		if !unicode.IsSpace(value) {
			last = used
		}
		value, err = r.rune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return CoverageModeUnknown, err
		}
	}
	return parseCoverageMode(string(header[:last]))
}
func (r *coverageInput) block(first rune, mode CoverageMode) (CoverageBlock, error) {
	separator, err := r.position(first)
	if err != nil {
		return CoverageBlock{}, err
	}
	statements, separator, err := r.number(separator)
	if err != nil {
		return CoverageBlock{}, err
	}
	hits, separator, err := r.number(separator)
	if err != nil {
		return CoverageBlock{}, err
	}
	if err := r.endLine(separator); err != nil {
		return CoverageBlock{}, err
	}
	block := CoverageBlock{Mode: mode, Statements: statements, Hits: hits}
	return block, block.Validate()
}
func (r *coverageInput) number(separator rune) (int64, rune, error) {
	value := separator
	for unicode.IsSpace(value) && value != '\n' {
		var err error
		value, err = r.rune()
		if err != nil {
			return 0, 0, err
		}
	}
	var number int64
	digits := false
	for !unicode.IsSpace(value) && value != 0 {
		if err := appendCoverageSignedDigit(&number, value); err != nil {
			return 0, 0, err
		}
		digits = true
		var err error
		value, err = r.rune()
		if err == io.EOF {
			value = 0
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if !digits {
		return 0, 0, core.ErrPrimitiveContract
	}
	return number, value, nil
}
func appendCoverageSignedDigit(number *int64, value rune) error {
	if value < '0' || value > '9' {
		return core.ErrPrimitiveContract
	}
	digit := int64(value - '0')
	if *number > (math.MaxInt64-digit)/10 {
		return errors.Join(core.ErrPrimitiveContract, core.ErrNumericOverflow)
	}
	*number = *number*10 + digit
	return nil
}
func (r *coverageInput) endLine(value rune) error {
	for value != 0 && value != '\n' {
		if !unicode.IsSpace(value) {
			return core.ErrPrimitiveContract
		}
		var err error
		value, err = r.rune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Each colon replaces the candidate suffix: filenames may themselves contain
// colons. Four decimal coordinates are checked without retaining the filename.
type coverageCoordinateSuffix struct {
	number    int64
	component uint8
	digits    bool
	invalid   bool
}

const coverageCoordinateSeparators = ".,."

func (s *coverageCoordinateSuffix) consume(value rune) {
	if int(s.component) < len(coverageCoordinateSeparators) && value == rune(coverageCoordinateSeparators[s.component]) && s.digits {
		s.component++
		s.number = 0
		s.digits = false
		return
	}
	s.invalid = s.invalid || appendCoverageSignedDigit(&s.number, value) != nil
	s.digits = true
}
func (s coverageCoordinateSuffix) Validate() error {
	if s.invalid || int(s.component) != len(coverageCoordinateSeparators) || !s.digits {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (r *coverageInput) position(value rune) (rune, error) {
	var suffix coverageCoordinateSuffix
	prefix, colon := false, false
	for !unicode.IsSpace(value) {
		if value == ':' {
			colon = prefix
			suffix = coverageCoordinateSuffix{}
		} else if colon {
			suffix.consume(value)
		}
		prefix = true
		var err error
		value, err = r.rune()
		if err != nil {
			return 0, errors.Join(core.ErrPrimitiveContract, err)
		}
	}
	if !colon {
		return 0, core.ErrPrimitiveContract
	}
	return value, suffix.Validate()
}

var _ core.Validatable = CoverageBlock{}
var _ core.Validatable = CoverageBlockRequest{}
