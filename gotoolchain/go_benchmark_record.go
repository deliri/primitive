package gotoolchain

import (
	"bytes"
	"github.com/deliri/primitive/v2026/core"
	"iter"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
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
)

// Validate refuses an uninitialized or unknown presence.
func (p GoBenchmarkRecordPresence) Validate() error {
	if p != GoBenchmarkRecordAbsent && p != GoBenchmarkRecordPresent {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// OffWireEnum marks presence as an internal observation domain.
func (GoBenchmarkRecordPresence) OffWireEnum() {}

// GoBenchmarkName holds the exact name field emitted by Go, including its CPU
// suffix. Selection, parent attribution and rounding belong to the product.
type GoBenchmarkName struct{ value string }

// String returns the exact admitted name.
func (n GoBenchmarkName) String() string { return n.value }

// Validate admits a UTF-8 benchmark name without whitespace or controls.
func (n GoBenchmarkName) Validate() error {
	if !strings.HasPrefix(n.value, "Benchmark") || !utf8.ValidString(n.value) {
		return core.ErrGoToolchainOutput
	}
	for _, character := range n.value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return core.ErrGoToolchainOutput
		}
	}
	return nil
}

// GoBenchmarkRecord preserves native numeric facts. In particular ns/op remains
// a finite nonnegative float: the receiver decides rounding and saturation.
type GoBenchmarkRecord struct {
	Name        GoBenchmarkName
	Iterations  int64
	Nanoseconds float64
	Bytes       int64
	Allocations int64
	Presence    GoBenchmarkRecordPresence
	Fields      GoBenchmarkMetricFields
}

// Validate checks presence, finite numeric facts and field authority.
func (r GoBenchmarkRecord) Validate() error {
	if err := r.Presence.Validate(); err != nil {
		return err
	}
	if err := r.Fields.Validate(); err != nil {
		return err
	}
	if r.Presence == GoBenchmarkRecordAbsent {
		if r.Name != (GoBenchmarkName{}) || r.Iterations != 0 || r.Nanoseconds != 0 || r.Bytes != 0 || r.Allocations != 0 || r.Fields != GoBenchmarkMetricFieldsNone {
			return core.ErrGoToolchainOutput
		}
		return nil
	}
	if r.Iterations < 0 || r.Nanoseconds < 0 || math.IsNaN(r.Nanoseconds) || math.IsInf(r.Nanoseconds, 0) || r.Bytes < 0 || r.Allocations < 0 {
		return core.ErrGoToolchainOutput
	}
	if r.Fields&GoBenchmarkMetricTime == 0 && r.Nanoseconds != 0 || r.Fields&GoBenchmarkMetricBytes == 0 && r.Bytes != 0 || r.Fields&GoBenchmarkMetricAllocations == 0 && r.Allocations != 0 {
		return core.ErrGoToolchainOutput
	}
	return r.Name.Validate()
}

// GoBenchmarkRecordRequest borrows one raw record. An unrelated or empty record
// is a neutral observation. No line, field-count or byte-extent quota applies.
type GoBenchmarkRecordRequest struct{ Source []byte }

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

// ObserveGoBenchmarkRecord scans Go's field sequence without materializing a
// token slice, metric registry or sample inventory. Apart from the returned
// name, working memory is constant. All refusals use a core error constant.
func ObserveGoBenchmarkRecord(request GoBenchmarkRecordRequest) (GoBenchmarkRecord, error) {
	if !bytes.HasPrefix(request.Source, []byte("Benchmark")) {
		return GoBenchmarkRecord{Presence: GoBenchmarkRecordAbsent}, nil
	}
	if ending := bytes.IndexByte(request.Source, '\n'); ending >= 0 && ending != len(request.Source)-1 {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	next, stop := iter.Pull(bytes.FieldsSeq(request.Source))
	defer stop()
	name, ok := next()
	if !ok {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	if !validGoBenchmarkNameBytes(name) {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	iterations, ok := next()
	if !ok {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	count, err := goBenchmarkInteger(iterations)
	if err != nil || count < 0 {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	record := GoBenchmarkRecord{Iterations: count, Presence: GoBenchmarkRecordPresent}
	metricPairs := false
	for {
		value, ok := next()
		if !ok {
			break
		}
		unit, ok := next()
		if !ok {
			return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
		}
		metricPairs = true
		if err := projectGoBenchmarkMetric(&record, value, unit); err != nil {
			return GoBenchmarkRecord{}, err
		}
	}
	if !metricPairs {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	// A refused row publishes no name, so defer the only source-sized copy
	// until every numeric and framing refusal has been admitted.
	record.Name = GoBenchmarkName{value: string(name)}
	if err := record.Validate(); err != nil {
		return GoBenchmarkRecord{}, err
	}
	return record, nil
}

func validGoBenchmarkNameBytes(value []byte) bool {
	if !bytes.HasPrefix(value, []byte("Benchmark")) || !utf8.Valid(value) {
		return false
	}
	for len(value) != 0 {
		character, size := utf8.DecodeRune(value)
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
		value = value[size:]
	}
	return true
}

var (
	_ core.Validatable = GoBenchmarkName{}
	_ core.Validatable = GoBenchmarkRecord{}
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

func projectGoBenchmarkMetric(record *GoBenchmarkRecord, value, unit []byte) error {
	kind := goBenchmarkUnit(unit)
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
