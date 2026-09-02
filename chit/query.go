package chit

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

const specificSelectionPaginationDiagnostic = "specific chit selection cannot be paginated"

// Selection is the exact all-or-one consumer catalog selection.
type Selection struct {
	Chit ChitID                    `json:"chit_id"`
	Kind core.CatalogSelectionKind `json:"kind"`
}

// All selects every chit in one authenticated scope.
func All() Selection { return Selection{Kind: core.CatalogSelectionAll} }

// Specific selects one exact chit identity.
func Specific(identity ChitID) (Selection, error) {
	candidate := Selection{Kind: core.CatalogSelectionSpecific, Chit: identity}
	return candidate, candidate.Validate()
}

// Validate enforces the exact tagged-union arm.
func (s Selection) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return contractError(err)
	}
	switch s.Kind {
	case core.CatalogSelectionAll:
		if s.Chit != (ChitID{}) {
			return contractError(errors.New("all selection carries a chit identity"))
		}
	case core.CatalogSelectionSpecific:
		if err := s.Chit.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("selection kind escaped its domain"))
	}
	return nil
}

// MarshalJSON emits only the member owned by the selected tagged-union arm.
func (s Selection) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	if s.Kind == core.CatalogSelectionAll {
		return core.MarshalCanonicalJSONDocument(struct {
			Kind core.CatalogSelectionKind `json:"kind"`
		}{Kind: s.Kind})
	}
	return core.MarshalCanonicalJSONDocument(struct {
		Chit ChitID                    `json:"chit_id"`
		Kind core.CatalogSelectionKind `json:"kind"`
	}{Chit: s.Chit, Kind: s.Kind})
}

// Position is the explicit first-page or after-cursor request arm.
type Position struct {
	Cursor Cursor                   `json:"cursor"`
	Kind   core.CatalogPositionKind `json:"kind"`
}

// Start requests the first catalog page.
func Start() Position { return Position{Kind: core.CatalogPositionStart} }

// After requests the page after one authority-issued cursor.
func After(cursor Cursor) (Position, error) {
	candidate := Position{Kind: core.CatalogPositionAfter, Cursor: cursor}
	return candidate, candidate.Validate()
}

// Validate enforces the exact tagged-union arm.
func (p Position) Validate() error {
	if err := p.Kind.Validate(); err != nil {
		return contractError(err)
	}
	switch p.Kind {
	case core.CatalogPositionStart:
		if p.Cursor != (Cursor{}) {
			return contractError(errors.New("start position carries a cursor"))
		}
	case core.CatalogPositionAfter:
		if err := p.Cursor.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("position kind escaped its domain"))
	}
	return nil
}

// MarshalJSON emits only the member owned by the selected tagged-union arm.
func (p Position) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, jsonError(err)
	}
	if p.Kind == core.CatalogPositionStart {
		return core.MarshalCanonicalJSONDocument(struct {
			Kind core.CatalogPositionKind `json:"kind"`
		}{Kind: p.Kind})
	}
	return core.MarshalCanonicalJSONDocument(struct {
		Cursor Cursor                   `json:"cursor"`
		Kind   core.CatalogPositionKind `json:"kind"`
	}{Cursor: p.Cursor, Kind: p.Kind})
}

// Query is the complete typed input behind `chit -all` or `chit <id>`.
type Query struct {
	Scope     receipt.Scope         `json:"scope"`
	Partition Partition             `json:"partition"`
	Selection Selection             `json:"selection"`
	Position  Position              `json:"position"`
	Limit     core.CatalogPageLimit `json:"limit"`
}

// QueryRequest is the constructor boundary for one consumer catalog query.
// PageSize is immediately closed into Core's nominal page limit.
type QueryRequest struct {
	Scope     receipt.Scope
	Partition Partition
	Selection Selection
	Position  Position
	PageSize  uint16
}

// Validate closes every constructor input without constructing the query.
func (r QueryRequest) Validate() error {
	_, limitErr := core.NewCatalogPageLimit(r.PageSize)
	if err := errors.Join(r.Scope.Validate(), r.Partition.Validate(), r.Selection.Validate(), r.Position.Validate(), limitErr); err != nil {
		return contractError(err)
	}
	if r.Selection.Kind == core.CatalogSelectionSpecific && r.Position.Kind != core.CatalogPositionStart {
		return conflictError(errors.New(specificSelectionPaginationDiagnostic))
	}
	return nil
}

// NewQuery constructs one completely typed catalog query.
func NewQuery(request QueryRequest) (Query, error) {
	if err := request.Validate(); err != nil {
		return Query{}, err
	}
	limit, err := core.NewCatalogPageLimit(request.PageSize)
	if err != nil {
		return Query{}, contractError(err)
	}
	candidate := Query{
		Scope: request.Scope, Partition: request.Partition, Selection: request.Selection,
		Position: request.Position, Limit: limit,
	}
	return candidate, candidate.Validate()
}

// Validate closes the scope, selection, position, and shared page limit.
func (q Query) Validate() error {
	if err := errors.Join(q.Scope.Validate(), q.Partition.Validate(), q.Selection.Validate(), q.Position.Validate(), q.Limit.Validate()); err != nil {
		return contractError(err)
	}
	if q.Selection.Kind == core.CatalogSelectionSpecific && q.Position.Kind != core.CatalogPositionStart {
		return conflictError(errors.New(specificSelectionPaginationDiagnostic))
	}
	return nil
}

var (
	_ core.Validatable            = Selection{}
	_ core.Validatable            = Position{}
	_ core.Validatable            = QueryRequest{}
	_ core.Validatable            = Query{}
	_ core.ValidatedJSONMarshaler = Selection{}
	_ core.ValidatedJSONMarshaler = Position{}
)
