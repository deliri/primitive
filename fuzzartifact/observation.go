package fuzzartifact

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// ObservationState describes how completely one directory was observed.
type ObservationState uint8

const (
	ObservationUnknown ObservationState = iota
	ObservationComplete
	ObservationUnsupportedFormat
	ObservationPartial
	ObservationFailed
	observationStateLimit
)

func observationStateDiagnostics() [observationStateLimit]string {
	return [...]string{
		ObservationComplete:          "complete",
		ObservationUnsupportedFormat: "unsupported-format",
		ObservationPartial:           "partial",
		ObservationFailed:            "observation-failed",
	}
}

// Validate rejects values outside the closed observation domain.
func (s ObservationState) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("observation state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed observation domain.
func (s ObservationState) IsValid() bool {
	return s > ObservationUnknown && s < observationStateLimit &&
		observationStateDiagnostics()[s] != ""
}

// OffWireEnum declares ObservationState as a runtime result, not wire data.
func (ObservationState) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for s.
func (s ObservationState) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return observationStateDiagnostics()[s]
}

// EntryCount is a saturating count of observed directory entries.
type EntryCount struct {
	value uint64
}

// Uint64 returns the exact or saturated count.
func (c EntryCount) Uint64() uint64 {
	return c.value
}

// Observation records constant-size accounting for one directory scan.
// Matched names were handed to Visit; Delivered counts callbacks returning nil.
// Names are not retained. Neither count proves payload identity or custody.
type Observation struct {
	ignoredDirectories uint64
	nonRegular         uint64
	unsupportedRegular uint64
	matched            uint64
	delivered          uint64
	kind               ArtifactKind
	format             CacheFormat
	state              ObservationState
}

// Validate rejects contradictory completion and callback accounting.
func (o Observation) Validate() error {
	if err := errors.Join(o.state.Validate(), o.kind.Validate(), o.format.Validate()); err != nil {
		return contractError(err)
	}
	if o.delivered > o.matched {
		return contractError(errors.New("delivered names exceed matched names"))
	}
	if o.matched-o.delivered > 1 {
		return contractError(errors.New("more than one callback was left unacknowledged"))
	}
	if o.delivered != o.matched && o.state != ObservationPartial {
		return contractError(errors.New("unacknowledged callback requires a partial observation"))
	}
	return o.validateStateAccounting()
}

func (o Observation) validateStateAccounting() error {
	switch o.state {
	case ObservationComplete:
		if o.unsupportedRegular != 0 {
			return contractError(errors.New("complete observation contains refused entries"))
		}
	case ObservationUnsupportedFormat:
		if o.unsupportedRegular == 0 {
			return contractError(errors.New("unsupported observation contradicts entry accounting"))
		}
	case ObservationPartial:
		if !o.hasAccounting() {
			return contractError(errors.New("partial observation contains no directory facts"))
		}
	case ObservationFailed:
		if o.hasAccounting() {
			return contractError(errors.New("failed observation contains directory facts"))
		}
	default:
		return contractError(errors.New("observation state is outside the admitted domain"))
	}
	return nil
}

func (o Observation) hasAccounting() bool {
	return o.matched != 0 || o.ignoredDirectories != 0 || o.nonRegular != 0 || o.unsupportedRegular != 0
}

// State returns the observation completeness state.
func (o Observation) State() ObservationState {
	return o.state
}

// Kind returns the artifact class the request declared for this directory. It
// is present on every observation, including a failed one, because the class is
// a property of the directory the caller named rather than of what was read.
func (o Observation) Kind() ArtifactKind {
	return o.kind
}

// Format returns the exact Go naming contract applied to this observation.
func (o Observation) Format() CacheFormat {
	return o.format
}

// Matched returns the number of generated names handed to the visitor.
func (o Observation) Matched() EntryCount { return EntryCount{value: o.matched} }

// Delivered returns the number of visitor calls that returned nil.
// A failed callback may have performed partial effects owned by the caller.
func (o Observation) Delivered() EntryCount { return EntryCount{value: o.delivered} }

// IgnoredDirectories returns the number of child directories not descended.
func (o Observation) IgnoredDirectories() EntryCount {
	return EntryCount{value: o.ignoredDirectories}
}

// NonRegular returns the number of non-directory, non-regular entries.
func (o Observation) NonRegular() EntryCount {
	return EntryCount{value: o.nonRegular}
}

// UnsupportedRegular returns regular files outside the declared Go format.
func (o Observation) UnsupportedRegular() EntryCount {
	return EntryCount{value: o.unsupportedRegular}
}

func incrementSaturating(value *uint64) {
	if *value != ^uint64(0) {
		*value++
	}
}
