package taskmanager

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
)

// CreateProjectRequest creates one independently addressable project.
type CreateProjectRequest struct {
	Name        Title            `json:"name"`
	Description Description      `json:"description"`
	ID          id.UUIDv7        `json:"id"`
	MutationID  id.UUIDv7        `json:"mutation_id"`
	Lifecycle   ProjectLifecycle `json:"lifecycle"`
}

func (r CreateProjectRequest) Validate() error {
	return validateJoined(r.ID.Validate(), r.MutationID.Validate(), r.Name.Validate(), r.Description.Validate(), r.Lifecycle.Validate())
}

func (r CreateProjectRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire CreateProjectRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r CreateProjectRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

// CreatePhaseRequest creates one project-local phase at one explicit position.
type CreatePhaseRequest struct {
	Name        Title       `json:"name"`
	Description Description `json:"description"`
	Position    uint64      `json:"position"`
	ID          id.UUIDv7   `json:"id"`
	ProjectID   id.UUIDv7   `json:"project_id"`
	MutationID  id.UUIDv7   `json:"mutation_id"`
}

func (r CreatePhaseRequest) Validate() error {
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.MutationID.Validate(),
		r.Name.Validate(), r.Description.Validate(),
	)
}

func (r CreatePhaseRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire CreatePhaseRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r CreatePhaseRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

// CreateTaskRequest creates one task without rewriting its project or phase.
type CreateTaskRequest struct {
	Title       Title       `json:"title"`
	Description Description `json:"description"`
	ID          id.UUIDv7   `json:"id"`
	ProjectID   id.UUIDv7   `json:"project_id"`
	PhaseID     id.UUIDv7   `json:"phase_id"`
	MutationID  id.UUIDv7   `json:"mutation_id"`
	Kind        TaskKind    `json:"kind"`
	State       TaskState   `json:"state"`
}

func (r CreateTaskRequest) Validate() error {
	return validateJoined(
		r.ID.Validate(), r.ProjectID.Validate(), r.PhaseID.Validate(), r.MutationID.Validate(),
		r.Title.Validate(), r.Description.Validate(), r.Kind.Validate(), r.State.Validate(),
	)
}

func (r CreateTaskRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire CreateTaskRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r CreateTaskRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

// ListTasksRequest selects one lifecycle partition inside one project and,
// optionally, one phase. Completed history requires an explicit request.
type ListTasksRequest struct {
	PhaseID    *id.UUIDv7     `json:"phase_id,omitempty"`
	After      *TaskCursor    `json:"after,omitempty"`
	Limit      PageLimit      `json:"limit"`
	ProjectID  id.UUIDv7      `json:"project_id"`
	Collection TaskCollection `json:"collection"`
	Order      PageOrder      `json:"order"`
}

func (r ListTasksRequest) Validate() error {
	var phaseErr, cursorErr error
	if r.PhaseID != nil {
		phaseErr = r.PhaseID.Validate()
	}
	if r.After != nil {
		cursorErr = r.After.Validate()
		if cursorErr == nil && r.After.ProjectID != r.ProjectID {
			cursorErr = contractError()
		}
	}
	return validateJoined(
		r.ProjectID.Validate(), phaseErr, cursorErr, r.Collection.Validate(), r.Order.Validate(), r.Limit.Validate(),
	)
}

func (r ListTasksRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ListTasksRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// CompleteTaskRequest atomically closes one task against its entity-local
// revision. Proof records are appended independently before this transition.
type CompleteTaskRequest struct {
	ProjectID        id.UUIDv7 `json:"project_id"`
	TaskID           id.UUIDv7 `json:"task_id"`
	MutationID       id.UUIDv7 `json:"mutation_id"`
	ExpectedRevision Revision  `json:"expected_revision"`
}

func (r CompleteTaskRequest) Validate() error {
	return validateJoined(
		r.ProjectID.Validate(), r.TaskID.Validate(), r.MutationID.Validate(), r.ExpectedRevision.Validate(),
	)
}

func (r CompleteTaskRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire CompleteTaskRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}
func (r CompleteTaskRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	return mutationKey(r, r.MutationID)
}

func validateJoined(values ...error) error {
	if err := errors.Join(values...); err != nil {
		return contractError(err)
	}
	return nil
}

func mutationKey[Request core.Validatable](request Request, mutationID id.UUIDv7) (exchange.IdempotencyKey, error) {
	if err := request.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	key, err := exchange.ParseIdempotencyKey(mutationID.String())
	if err != nil {
		return exchange.IdempotencyKey{}, contractError(err)
	}
	return key, nil
}

var (
	_ core.ValidatedJSONMarshaler = CreateProjectRequest{}
	_ core.ValidatedJSONMarshaler = CreatePhaseRequest{}
	_ core.ValidatedJSONMarshaler = CreateTaskRequest{}
	_ core.ValidatedJSONMarshaler = ListTasksRequest{}
	_ core.ValidatedJSONMarshaler = CompleteTaskRequest{}
	_ exchange.IdempotencyBound   = CreateProjectRequest{}
	_ exchange.IdempotencyBound   = CreatePhaseRequest{}
	_ exchange.IdempotencyBound   = CreateTaskRequest{}
	_ exchange.IdempotencyBound   = CompleteTaskRequest{}
)
