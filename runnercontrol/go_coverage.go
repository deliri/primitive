package runnercontrol

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	GoCoverageLineMaximumBytes uint32 = 1 << 20
	GoCoverageLineMaximum      uint32 = 1 << 20
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
	if o.Statements == 0 || o.Covered > o.Statements || o.BasisPoints > 10_000 || o.Covered > math.MaxUint64/10_000 {
		return core.ErrPrimitiveContract
	}
	if uint64(o.BasisPoints) != o.Covered*10_000/o.Statements {
		return core.ErrPrimitiveContract
	}
	return nil
}

type GoCoverageCompiler struct {
	failure    error
	pending    []byte
	statements uint64
	covered    uint64
	lines      uint32
	mode       CoverageMode
}

func NewGoCoverageCompiler() *GoCoverageCompiler { return &GoCoverageCompiler{} }

func (c *GoCoverageCompiler) Write(data []byte) (int, error) {
	if c == nil {
		return 0, errors.Join(core.ErrPrimitiveContract, errors.New(goCoverageCompilerNilDiagnostic))
	}
	if c.failure != nil {
		return 0, c.failure
	}
	written, extentFailure, err := writeBoundedLines(boundedLineWrite{
		pending: &c.pending,
		data:    data,
		maximum: int(GoCoverageLineMaximumBytes),
		consume: func(line []byte) error { return c.consumeLine(string(line)) },
	})
	if err != nil {
		c.failure = err
		c.pending = nil
		if !extentFailure {
			return len(data), nil
		}
	}
	return written, err
}

func (c *GoCoverageCompiler) consumeLine(line string) error {
	if c.mode == CoverageModeUnknown {
		mode, err := parseCoverageMode(line)
		if err != nil {
			return err
		}
		c.mode = mode
		return nil
	}
	if c.lines >= GoCoverageLineMaximum {
		return coverageFailure("go coverage record count exceeds the ceiling")
	}
	location, statements, count, err := parseCoverageRecord(line)
	if err != nil {
		return err
	}
	if location == "" {
		return coverageFailure("go coverage record location is empty")
	}
	return c.accumulateCoverage(statements, count)
}

func parseCoverageRecord(line string) (string, uint64, uint64, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", 0, 0, coverageFailure("go coverage record must contain location, statements, and count")
	}
	statements, statementErr := strconv.ParseUint(fields[1], 10, 32)
	count, countErr := strconv.ParseUint(fields[2], 10, 64)
	if statementErr != nil || countErr != nil || statements == 0 || !strings.Contains(fields[0], ":") {
		return "", 0, 0, errors.Join(coverageFailure("go coverage record has invalid numeric or location facts"), statementErr, countErr)
	}
	return fields[0], statements, count, nil
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
	c.lines++
	return nil
}

func (c *GoCoverageCompiler) Seal() (GoCoverageObservation, error) {
	if c == nil {
		return GoCoverageObservation{}, errors.Join(core.ErrPrimitiveContract, errors.New(goCoverageCompilerNilDiagnostic))
	}
	if c.failure != nil {
		return GoCoverageObservation{}, c.failure
	}
	if len(c.pending) > 0 {
		if len(c.pending) > int(GoCoverageLineMaximumBytes) {
			return GoCoverageObservation{}, coverageFailure("go coverage line exceeds the byte ceiling")
		}
		if err := c.consumeLine(string(c.pending)); err != nil {
			return GoCoverageObservation{}, err
		}
		c.pending = nil
	}
	if c.mode == CoverageModeUnknown || c.lines == 0 || c.statements > math.MaxUint64/10_000 {
		return GoCoverageObservation{}, coverageFailure("go coverage stream has no bounded statement evidence")
	}
	basisPoints, err := checkedUint16FromUint64(c.covered * 10_000 / c.statements)
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
