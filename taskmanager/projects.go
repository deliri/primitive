package taskmanager

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// ProjectCursor is one stable updated-at/identity continuation point.
type ProjectCursor struct {
	UpdatedAt temporal.NumericInstant `json:"updated_at"`
	ID        id.UUIDv7               `json:"id"`
}

func (c ProjectCursor) Validate() error {
	if err := errors.Join(c.UpdatedAt.Validate(), c.ID.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// ListProjectsRequest selects one bounded lifecycle page.
type ListProjectsRequest struct {
	After     *ProjectCursor   `json:"after,omitempty"`
	Lifecycle ProjectLifecycle `json:"lifecycle"`
	Order     PageOrder        `json:"order"`
	Limit     PageLimit        `json:"limit"`
}

func (r ListProjectsRequest) Validate() error {
	if err := errors.Join(r.Lifecycle.Validate(), r.Order.Validate(), r.Limit.Validate()); err != nil {
		return contractError(err)
	}
	if r.After != nil {
		return r.After.Validate()
	}
	return nil
}

func (r ListProjectsRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ListProjectsRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

// ProjectSummary is the bounded project list projection.
type ProjectSummary struct {
	Name            Title                   `json:"name"`
	CreatedAt       temporal.NumericInstant `json:"created_at"`
	UpdatedAt       temporal.NumericInstant `json:"updated_at"`
	Revision        Revision                `json:"revision"`
	PhaseCount      uint64                  `json:"phase_count"`
	OpenTaskCount   uint64                  `json:"open_task_count"`
	ClosedTaskCount uint64                  `json:"closed_task_count"`
	ID              id.UUIDv7               `json:"id"`
	Lifecycle       ProjectLifecycle        `json:"lifecycle"`
}

// ProjectDetail is one independently addressed project. Collection history
// remains paged and is represented here only by exact counters.
type ProjectDetail struct {
	Description Description    `json:"description"`
	Summary     ProjectSummary `json:"summary"`
}

func (d ProjectDetail) Validate() error {
	if err := errors.Join(d.Summary.Validate(), d.Description.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (d ProjectDetail) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ProjectDetail
	return core.MarshalCanonicalJSONDocument(wire(d))
}

// GetProjectRequest directly addresses one project without loading either
// lifecycle collection.
type GetProjectRequest struct {
	ProjectID id.UUIDv7 `json:"project_id"`
}

func (r GetProjectRequest) Validate() error { return r.ProjectID.Validate() }

func (r GetProjectRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire GetProjectRequest
	return core.MarshalCanonicalJSONDocument(wire(r))
}

func (s ProjectSummary) Validate() error {
	if err := errors.Join(
		s.ID.Validate(), s.Name.Validate(), s.Lifecycle.Validate(), s.Revision.Validate(),
		s.CreatedAt.Validate(), s.UpdatedAt.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (s ProjectSummary) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ProjectSummary
	return core.MarshalCanonicalJSONDocument(wire(s))
}

// ProjectPage is one bounded lifecycle partition.
type ProjectPage struct {
	Next      *ProjectCursor   `json:"next,omitempty"`
	Items     []ProjectSummary `json:"items"`
	Lifecycle ProjectLifecycle `json:"lifecycle"`
	Order     PageOrder        `json:"order"`
}

func (p ProjectPage) Validate() error {
	if err := errors.Join(p.Lifecycle.Validate(), p.Order.Validate()); err != nil || len(p.Items) > int(PageLimitMaximum) {
		return contractError(err)
	}
	if err := p.validateItems(); err != nil {
		return err
	}
	return validateProjectPageCursor(p)
}

func (p ProjectPage) validateItems() error {
	for _, item := range p.Items {
		if err := item.Validate(); err != nil || item.Lifecycle != p.Lifecycle {
			return contractError(err)
		}
	}
	return nil
}

func validateProjectPageCursor(page ProjectPage) error {
	if page.Next == nil {
		return nil
	}
	if err := page.Next.Validate(); err != nil || len(page.Items) == 0 {
		return contractError(err)
	}
	last := page.Items[len(page.Items)-1]
	if page.Next.UpdatedAt != last.UpdatedAt || page.Next.ID != last.ID {
		return contractError()
	}
	return nil
}

func (p ProjectPage) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	type wire ProjectPage
	return core.MarshalCanonicalJSONDocument(wire(p))
}

var (
	_ core.ValidatedJSONMarshaler = ListProjectsRequest{}
	_ core.ValidatedJSONMarshaler = ProjectSummary{}
	_ core.ValidatedJSONMarshaler = ProjectDetail{}
	_ core.ValidatedJSONMarshaler = GetProjectRequest{}
	_ core.ValidatedJSONMarshaler = ProjectPage{}
)
