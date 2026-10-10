package runprotocol

import (
	"bytes"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"math"
)

// GoBenchmarkRecordPresence distinguishes an unrelated record from measured facts.
type GoBenchmarkRecordPresence uint8

const (
	// GoBenchmarkRecordUnknown is the uninitialized observation.
	GoBenchmarkRecordUnknown GoBenchmarkRecordPresence = iota
	// GoBenchmarkRecordAbsent records an unrelated Go output line.
	GoBenchmarkRecordAbsent
	// GoBenchmarkRecordPresent records an admitted benchmark row.
	GoBenchmarkRecordPresent
	// GoBenchmarkRecordRefused records a complete syntactically invalid native row.
	GoBenchmarkRecordRefused
)

// Validate refuses an uninitialized or unknown presence.
func (p GoBenchmarkRecordPresence) Validate() error {
	if p != GoBenchmarkRecordAbsent && p != GoBenchmarkRecordPresent && p != GoBenchmarkRecordRefused {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// OffWireEnum marks presence as an internal observation domain.
func (GoBenchmarkRecordPresence) OffWireEnum() {}

// goBenchmarkMeasurements holds only native scalar facts while one row is read.
type goBenchmarkMeasurements struct {
	Iterations  int64
	Nanoseconds float64
	Bytes       int64
	Allocations int64
	Fields      GoBenchmarkMetricFields
}

func (goBenchmarkMeasurements) runProtocolInternalFlowCarrier() {}

// GoBenchmarkMetricFields records which known units occurred in the row.
type GoBenchmarkMetricFields uint8

const (
	// GoBenchmarkMetricTime records an ns/op field.
	GoBenchmarkMetricTime GoBenchmarkMetricFields = 1 << iota
	// GoBenchmarkMetricBytes records a B/op field.
	GoBenchmarkMetricBytes
	// GoBenchmarkMetricAllocations records an allocs/op field.
	GoBenchmarkMetricAllocations
)

// GoBenchmarkMetricFieldsNone records no recognized unit.
const GoBenchmarkMetricFieldsNone GoBenchmarkMetricFields = 0

// Validate refuses bits outside the native metric domain.
func (f GoBenchmarkMetricFields) Validate() error {
	if f & ^(GoBenchmarkMetricTime|GoBenchmarkMetricBytes|GoBenchmarkMetricAllocations) != 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// OffWireEnum marks metric fields as an internal observation domain.
func (GoBenchmarkMetricFields) OffWireEnum() {}

var (
	_ core.OffWireEnum = GoBenchmarkRecordUnknown
	_ core.OffWireEnum = GoBenchmarkMetricFieldsNone
)

func goBenchmarkUnit(unit []byte) GoBenchmarkMetricFields {
	switch {
	case bytes.Equal(unit, []byte("ns/op")):
		return GoBenchmarkMetricTime
	case bytes.Equal(unit, []byte("B/op")):
		return GoBenchmarkMetricBytes
	case bytes.Equal(unit, []byte("allocs/op")):
		return GoBenchmarkMetricAllocations
	default:
		return GoBenchmarkMetricFieldsNone
	}
}

func projectGoBenchmarkMetric(record *goBenchmarkMeasurements, value io.Reader, kind GoBenchmarkMetricFields) error {
	if kind == GoBenchmarkMetricFieldsNone {
		return nil
	}
	switch kind {
	case GoBenchmarkMetricTime:
		parsed, err := goBenchmarkFloat(value)
		if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return core.ErrGoToolchainOutput
		}
		record.Nanoseconds = parsed
	case GoBenchmarkMetricBytes, GoBenchmarkMetricAllocations:
		parsed, err := goBenchmarkInteger(value)
		if err != nil || parsed < 0 {
			return core.ErrGoToolchainOutput
		}
		if kind == GoBenchmarkMetricBytes {
			record.Bytes = parsed
		} else {
			record.Allocations = parsed
		}
	default:
		return core.ErrGoToolchainOutput
	}
	record.Fields |= kind
	return nil
}
