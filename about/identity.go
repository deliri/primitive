package about

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

type OriginIdentity struct {
	Offering core.Offering `json:"offering"`
}

func (i OriginIdentity) Validate() error {
	if err := i.Offering.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// EvidenceAuthority names one producer or verifier authority. Field position
// supplies the role; the offering supplies the independently checkable owner.
type EvidenceAuthority struct {
	Offering core.Offering `json:"offering"`
}

func (a EvidenceAuthority) Validate() error {
	if err := a.Offering.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

type SubjectIdentity struct {
	Project    core.Offering      `json:"project"`
	Repository RepositoryIdentity `json:"repository"`
}

func (i SubjectIdentity) Validate() error {
	if err := errors.Join(i.Project.Validate(), i.Repository.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type SourceCoordinate struct {
	Repository RepositoryIdentity `json:"repository"`
	Commit     core.BuildCommit   `json:"commit"`
	Tree       core.SHA256Digest  `json:"tree_digest"`
}

func (c SourceCoordinate) Validate() error {
	if err := errors.Join(c.Repository.Validate(), c.Commit.Validate(), c.Tree.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type GitOrigin struct {
	Commit core.BuildCommit `json:"commit"`
	At     temporal.Instant `json:"at"`
}

func (o GitOrigin) Validate() error {
	if err := errors.Join(o.Commit.Validate(), o.At.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type OptionalGitOrigin struct {
	Commit *core.BuildCommit `json:"commit,omitempty"`
	At     *temporal.Instant `json:"at,omitempty"`
}

func (o OptionalGitOrigin) Validate() error {
	if o.Commit == nil && o.At == nil {
		return nil
	}
	if o.Commit == nil || o.At == nil {
		return contractError(errors.New("about optional git origin is incomplete"))
	}
	if err := errors.Join(o.Commit.Validate(), o.At.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type RequestIdentity struct{ value primitiveid.UUIDv7 }
type RunID struct{ value primitiveid.UUIDv7 }
type ExperimentID struct{ value primitiveid.UUIDv7 }
type ObservationID struct{ value primitiveid.UUIDv7 }
type MachineID struct{ value primitiveid.UUIDv7 }
type MachineGenerationID struct{ value primitiveid.UUIDv7 }
type MachineObservationID struct{ value primitiveid.UUIDv7 }

func NewRequestIdentity(value primitiveid.UUIDv7) (RequestIdentity, error) {
	candidate := RequestIdentity{value: value}
	return candidate, candidate.Validate()
}

func NewRunID(value primitiveid.UUIDv7) (RunID, error) {
	candidate := RunID{value: value}
	return candidate, candidate.Validate()
}

func NewExperimentID(value primitiveid.UUIDv7) (ExperimentID, error) {
	candidate := ExperimentID{value: value}
	return candidate, candidate.Validate()
}

func NewObservationID(value primitiveid.UUIDv7) (ObservationID, error) {
	candidate := ObservationID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineID(value primitiveid.UUIDv7) (MachineID, error) {
	candidate := MachineID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineGenerationID(value primitiveid.UUIDv7) (MachineGenerationID, error) {
	candidate := MachineGenerationID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineObservationID(value primitiveid.UUIDv7) (MachineObservationID, error) {
	candidate := MachineObservationID{value: value}
	return candidate, candidate.Validate()
}

func (i RequestIdentity) Validate() error { return validateUUIDIdentity(i.value, "request") }
func (i RunID) Validate() error           { return validateUUIDIdentity(i.value, "run") }
func (i ExperimentID) Validate() error    { return validateUUIDIdentity(i.value, "experiment") }
func (i ObservationID) Validate() error   { return validateUUIDIdentity(i.value, "observation") }
func (i MachineID) Validate() error       { return validateUUIDIdentity(i.value, "machine") }
func (i MachineGenerationID) Validate() error {
	return validateUUIDIdentity(i.value, "machine generation")
}
func (i MachineObservationID) Validate() error {
	return validateUUIDIdentity(i.value, "machine observation")
}

func validateUUIDIdentity(value primitiveid.UUIDv7, kind string) error {
	if err := value.Validate(); err != nil {
		return contractError(errors.New("about "+kind+" identity is invalid"), err)
	}
	return nil
}

func (i RequestIdentity) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i RunID) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.value, i.Validate) }
func (i ExperimentID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i ObservationID) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.value, i.Validate) }
func (i MachineID) MarshalJSON() ([]byte, error)     { return marshalUUIDIdentity(i.value, i.Validate) }
func (i MachineGenerationID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i MachineObservationID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}

func marshalUUIDIdentity(value primitiveid.UUIDv7, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, jsonError(err)
	}
	return value.MarshalJSON()
}

func (i *RequestIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about request identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewRequestIdentity(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *RunID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about run identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewRunID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *ExperimentID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about experiment identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewExperimentID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *ObservationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about observation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewObservationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about machine identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineGenerationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about machine generation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineGenerationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineObservationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil about machine observation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineObservationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func sameSubject(left, right SubjectIdentity) bool {
	return left.Project == right.Project && left.Repository.value == right.Repository.value
}

var (
	_ json.Marshaler   = RequestIdentity{}
	_ json.Unmarshaler = (*RequestIdentity)(nil)
	_ json.Marshaler   = ObservationID{}
	_ json.Unmarshaler = (*ObservationID)(nil)
)
