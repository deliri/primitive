package taskmanager

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// PhaseCursor is one stable phase continuation bound to its project.
type PhaseCursor struct {
	ProjectID id.UUIDv7 `json:"project_id"`
	Position  uint64    `json:"position"`
	ID        id.UUIDv7 `json:"id"`
}

func (c PhaseCursor) Validate() error {
	if err := errors.Join(c.ProjectID.Validate(), c.ID.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// ListPhasesRequest selects one bounded page inside one project.
type ListPhasesRequest struct {
	After     *PhaseCursor `json:"after,omitempty"`
	Limit     PageLimit    `json:"limit"`
	ProjectID id.UUIDv7    `json:"project_id"`
	Order     PageOrder    `json:"order"`
}

func (r ListPhasesRequest) Validate() error {
	if err := errors.Join(r.ProjectID.Validate(), r.Order.Validate(), r.Limit.Validate()); err != nil {
		return contractError(err)
	}
	if r.After != nil {
		if err := r.After.Validate(); err != nil || r.After.ProjectID != r.ProjectID {
			return contractError(err)
		}
	}
	return nil
}

func (r ListPhasesRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ListPhasesRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// PhaseSummary is the bounded phase list projection.
type PhaseSummary struct {
	Name            Title                   `json:"name"`
	Description     Description             `json:"description"`
	CreatedAt       temporal.NumericInstant `json:"created_at"`
	UpdatedAt       temporal.NumericInstant `json:"updated_at"`
	Revision        Revision                `json:"revision"`
	Position        uint64                  `json:"position"`
	OpenTaskCount   uint64                  `json:"open_task_count"`
	ClosedTaskCount uint64                  `json:"closed_task_count"`
	ID              id.UUIDv7               `json:"id"`
	ProjectID       id.UUIDv7               `json:"project_id"`
}

func (s PhaseSummary) Validate() error {
	if err := errors.Join(
		s.ID.Validate(), s.ProjectID.Validate(), s.Name.Validate(), s.Description.Validate(),
		s.Revision.Validate(), s.CreatedAt.Validate(), s.UpdatedAt.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (s PhaseSummary) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire PhaseSummary
	return core.MarshalCanonicalJSONDocument(wire(s))
}

// PhasePage is one bounded project-local phase partition.
type PhasePage struct {
	Next      *PhaseCursor   `json:"next,omitempty"`
	Items     []PhaseSummary `json:"items"`
	ProjectID id.UUIDv7      `json:"project_id"`
	Order     PageOrder      `json:"order"`
}

func (p PhasePage) Validate() error {
	if err := errors.Join(p.ProjectID.Validate(), p.Order.Validate()); err != nil || len(p.Items) > int(PageLimitMaximum) {
		return contractError(err)
	}
	project, err := validatePhaseItems(p.Items)
	if err != nil {
		return err
	}
	if len(p.Items) != 0 && project != p.ProjectID {
		return contractError()
	}
	return validatePhasePageCursor(p)
}

func validatePhasePageCursor(page PhasePage) error {
	if page.Next == nil {
		return nil
	}
	if err := page.Next.Validate(); err != nil || len(page.Items) == 0 || page.Next.ProjectID != page.ProjectID {
		return contractError(err)
	}
	last := page.Items[len(page.Items)-1]
	if page.Next.Position != last.Position || page.Next.ID != last.ID {
		return contractError()
	}
	return nil
}

func (p PhasePage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire PhasePage
	return core.MarshalCanonicalJSONDocument(wire(p))
}

func validatePhaseItems(items []PhaseSummary) (id.UUIDv7, error) {
	var project id.UUIDv7
	for index, item := range items {
		if err := item.Validate(); err != nil {
			return id.UUIDv7{}, err
		}
		if index == 0 {
			project = item.ProjectID
			continue
		}
		if item.ProjectID != project {
			return id.UUIDv7{}, contractError()
		}
	}
	return project, nil
}

var (
	_ core.ValidatedJSONMarshaler = ListPhasesRequest{}
	_ core.ValidatedJSONMarshaler = PhaseSummary{}
	_ core.ValidatedJSONMarshaler = PhasePage{}
)
