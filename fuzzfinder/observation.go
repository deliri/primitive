package fuzzfinder

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

// Observation is the bounded, canonical result of finding generated artifacts
// in one directory. Names are observations only; they do not prove payload
// identity or custody.
type Observation struct {
	ignoredDirectories uint64
	nonRegular         uint64
	overLimit          uint64
	unsupportedRegular uint64
	limit              RetentionLimit
	retained           uint16
	names              [MaximumRetainedEntries]GeneratedName
	kind               ArtifactKind
	state              ObservationState
}

// Validate rejects contradictory observation states and noncanonical names.
func (o Observation) Validate() error {
	if err := o.state.Validate(); err != nil {
		return err
	}
	if err := o.kind.Validate(); err != nil {
		return err
	}
	if err := o.limit.Validate(); err != nil {
		return err
	}
	if o.retained > o.limit.value || o.retained > MaximumRetainedEntries {
		return contractError(errors.New("retained count exceeds its limit"))
	}
	if o.overLimit != 0 && o.retained != o.limit.value {
		return contractError(errors.New("over-limit observations require a full retained prefix"))
	}
	if err := o.validateNames(); err != nil {
		return err
	}
	return o.validateStateAccounting()
}

func (o Observation) validateStateAccounting() error {
	switch o.state {
	case ObservationComplete:
		if o.unsupportedRegular != 0 {
			return contractError(errors.New("complete observation contains unsupported regular entries"))
		}
	case ObservationUnsupportedFormat:
		if o.unsupportedRegular == 0 {
			return contractError(errors.New("unsupported observation has no unsupported regular entry"))
		}
	case ObservationFailed:
		if o.retained != 0 || o.hasAccounting() {
			return contractError(errors.New("failed observation contains directory facts"))
		}
	}
	return nil
}

func (o Observation) validateNames() error {
	for index := range int(o.retained) {
		if err := o.names[index].Validate(); err != nil {
			return contractError(err)
		}
		if index > 0 && o.names[index-1].compare(o.names[index]) >= 0 {
			return contractError(errors.New("retained generated names are not strictly canonical"))
		}
	}
	for index := int(o.retained); index < len(o.names); index++ {
		if o.names[index] != (GeneratedName{}) {
			return contractError(errors.New("unretained generated-name storage is not empty"))
		}
	}
	return nil
}

func (o Observation) hasAccounting() bool {
	return o.ignoredDirectories != 0 || o.nonRegular != 0 ||
		o.overLimit != 0 || o.unsupportedRegular != 0
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

// Names returns a defensive copy of the canonical retained prefix.
func (o Observation) Names() []GeneratedName {
	return append([]GeneratedName(nil), o.names[:o.retained]...)
}

// Retained returns the number of returned names.
func (o Observation) Retained() EntryCount {
	return EntryCount{value: uint64(o.retained)}
}

// IgnoredDirectories returns the number of child directories not descended.
func (o Observation) IgnoredDirectories() EntryCount {
	return EntryCount{value: o.ignoredDirectories}
}

// NonRegular returns the number of non-directory, non-regular entries.
func (o Observation) NonRegular() EntryCount {
	return EntryCount{value: o.nonRegular}
}

// OverLimitObservations returns the number of otherwise valid observations
// omitted or displaced after the retention limit was full. The unit is an
// observation, so a non-native reader that repeats an omitted name repeats the
// count; real directories cannot contain duplicate names.
func (o Observation) OverLimitObservations() EntryCount {
	return EntryCount{value: o.overLimit}
}

// UnsupportedRegular returns regular files outside the declared Go format.
func (o Observation) UnsupportedRegular() EntryCount {
	return EntryCount{value: o.unsupportedRegular}
}

func failedObservation(kind ArtifactKind, limit RetentionLimit) Observation {
	return Observation{limit: limit, kind: kind, state: ObservationFailed}
}

func incrementSaturating(value *uint64) {
	if *value != ^uint64(0) {
		*value++
	}
}
