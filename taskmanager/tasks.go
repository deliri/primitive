package taskmanager

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// TaskSummary is one bounded task-list projection.
type TaskSummary struct {
	Title     Title                   `json:"title"`
	UpdatedAt temporal.NumericInstant `json:"updated_at"`
	Revision  Revision                `json:"revision"`
	ID        id.UUIDv7               `json:"id"`
	ProjectID id.UUIDv7               `json:"project_id"`
	PhaseID   id.UUIDv7               `json:"phase_id"`
	Kind      TaskKind                `json:"kind"`
	State     TaskState               `json:"state"`
}

// TaskDetail is one independently addressed task. Proof history remains in
// its own bounded collections and is represented here only by exact counters.
type TaskDetail struct {
	Description    Description             `json:"description"`
	Summary        TaskSummary             `json:"summary"`
	CreatedAt      temporal.NumericInstant `json:"created_at"`
	EvidenceCount  uint64                  `json:"evidence_count"`
	GitCommitCount uint64                  `json:"git_commit_count"`
}

func (d TaskDetail) Validate() error {
	if err := errors.Join(d.Summary.Validate(), d.Description.Validate(), d.CreatedAt.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (d TaskDetail) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire TaskDetail
	return core.MarshalCanonicalJSONDocument(wire(d))
}

// GetTaskRequest directly addresses one task without loading its project or
// either task collection.
type GetTaskRequest struct {
	ProjectID id.UUIDv7 `json:"project_id"`
	TaskID    id.UUIDv7 `json:"task_id"`
}

func (r GetTaskRequest) Validate() error {
	return validateJoined(r.ProjectID.Validate(), r.TaskID.Validate())
}

func (r GetTaskRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire GetTaskRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (s TaskSummary) Validate() error {
	if err := errors.Join(
		s.ID.Validate(), s.ProjectID.Validate(), s.PhaseID.Validate(), s.Title.Validate(),
		s.Kind.Validate(), s.State.Validate(), s.Revision.Validate(), s.UpdatedAt.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (s TaskSummary) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire TaskSummary
	return core.MarshalCanonicalJSONDocument(wire(s))
}

// TaskCursor is one stable task continuation bound to its project.
type TaskCursor struct {
	ProjectID id.UUIDv7               `json:"project_id"`
	UpdatedAt temporal.NumericInstant `json:"updated_at"`
	ID        id.UUIDv7               `json:"id"`
}

func (c TaskCursor) Validate() error {
	if err := errors.Join(c.ProjectID.Validate(), c.UpdatedAt.Validate(), c.ID.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// TaskPage is one bounded task partition. Every item and cursor names the same
// project so a response cannot splice records from another tenant.
type TaskPage struct {
	Next       *TaskCursor    `json:"next,omitempty"`
	Items      []TaskSummary  `json:"items"`
	ProjectID  id.UUIDv7      `json:"project_id"`
	Collection TaskCollection `json:"collection"`
	Order      PageOrder      `json:"order"`
}

func (p TaskPage) Validate() error {
	if err := errors.Join(p.ProjectID.Validate(), p.Collection.Validate(), p.Order.Validate()); err != nil ||
		len(p.Items) > int(PageLimitMaximum) {
		return contractError(err)
	}
	if err := p.validateItems(); err != nil {
		return err
	}
	if p.Next != nil {
		if err := p.Next.Validate(); err != nil || len(p.Items) == 0 || p.Next.ProjectID != p.ProjectID {
			return contractError(err)
		}
		last := p.Items[len(p.Items)-1]
		if p.Next.UpdatedAt != last.UpdatedAt || p.Next.ID != last.ID {
			return contractError()
		}
	}
	return nil
}

func (p TaskPage) validateItems() error {
	for _, item := range p.Items {
		if err := item.Validate(); err != nil || item.ProjectID != p.ProjectID || !p.Collection.Contains(item.State) {
			return contractError(err)
		}
	}
	return nil
}

// Contains reports whether state belongs to this independently queried
// collection.
func (c TaskCollection) Contains(state TaskState) bool {
	if c.Validate() != nil || state.Validate() != nil {
		return false
	}
	if c == TaskCollectionActive {
		return state == TaskStateBacklog || state == TaskStateInProgress || state == TaskStateBlocked
	}
	return state == TaskStateCompleted || state == TaskStateCancelled
}

func (p TaskPage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire TaskPage
	return core.MarshalCanonicalJSONDocument(wire(p))
}

// TaskChange is an explicit typed patch. A nil field is unchanged; a present
// field must validate. At least one field must be present.
type TaskChange struct {
	Title       *Title       `json:"title,omitempty"`
	Description *Description `json:"description,omitempty"`
	Kind        *TaskKind    `json:"kind,omitempty"`
	State       *TaskState   `json:"state,omitempty"`
}

func (c TaskChange) Validate() error {
	if c.Title == nil && c.Description == nil && c.Kind == nil && c.State == nil {
		return contractError()
	}
	var titleErr, descriptionErr, kindErr, stateErr error
	if c.Title != nil {
		titleErr = c.Title.Validate()
	}
	if c.Description != nil {
		descriptionErr = c.Description.Validate()
	}
	if c.Kind != nil {
		kindErr = c.Kind.Validate()
	}
	if c.State != nil {
		stateErr = c.State.Validate()
	}
	if err := errors.Join(titleErr, descriptionErr, kindErr, stateErr); err != nil {
		return contractError(err)
	}
	return nil
}

// UpdateTaskRequest compares only the addressed task's revision.
type UpdateTaskRequest struct {
	Change           TaskChange `json:"change"`
	ExpectedRevision Revision   `json:"expected_revision"`
	ProjectID        id.UUIDv7  `json:"project_id"`
	TaskID           id.UUIDv7  `json:"task_id"`
	MutationID       id.UUIDv7  `json:"mutation_id"`
}

func (r UpdateTaskRequest) Validate() error {
	if err := errors.Join(
		r.ProjectID.Validate(), r.TaskID.Validate(), r.MutationID.Validate(),
		r.ExpectedRevision.Validate(), r.Change.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (r UpdateTaskRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire UpdateTaskRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

type updateTaskRequestWire UpdateTaskRequest

func (r *UpdateTaskRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonContractError()
	}
	wire, err := core.DecodeStrictJSONStructure[updateTaskRequestWire](
		data,
		core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return jsonContractError(err)
	}
	candidate := UpdateTaskRequest(wire)
	if err := candidate.Validate(); err != nil {
		return jsonContractError(err)
	}
	*r = candidate
	return nil
}

func (r UpdateTaskRequest) IdempotencyKey() (exchange.IdempotencyKey, error) {
	if err := r.Validate(); err != nil {
		return exchange.IdempotencyKey{}, err
	}
	key, err := exchange.ParseIdempotencyKey(r.MutationID.String())
	if err != nil {
		return exchange.IdempotencyKey{}, contractError(err)
	}
	return key, nil
}

var (
	_ core.ValidatedJSONMarshaler = UpdateTaskRequest{}
	_ core.ValidatedJSONMarshaler = TaskSummary{}
	_ core.ValidatedJSONMarshaler = TaskDetail{}
	_ core.ValidatedJSONMarshaler = GetTaskRequest{}
	_ core.ValidatedJSONMarshaler = TaskPage{}
	_ exchange.IdempotencyBound   = UpdateTaskRequest{}
)
