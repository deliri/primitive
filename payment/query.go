package payment

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

const specificSelectionPaginationDiagnostic = "specific payment selection cannot be paginated"

const (
	// CatalogCursorCommitmentDomain separates payment catalog positions from
	// every other digest use in Primitive.
	CatalogCursorCommitmentDomain = "primitive.payment.catalog-cursor.v1"
	// CatalogCursorFrameSeparator makes the domain and payment identity frame injective.
	CatalogCursorFrameSeparator byte = 0
)

// Cursor is Payment's nominal opaque closure of one catalog position.
type Cursor struct{ value core.SHA256Digest }

// NewCursor applies Payment's catalog domain to one opaque digest.
func NewCursor(value core.SHA256Digest) (Cursor, error) {
	candidate := Cursor{value: value}
	if err := candidate.Validate(); err != nil {
		return Cursor{}, err
	}
	return candidate, nil
}

// CursorFor closes the exact last Payment identity represented by a page into
// the opaque position from which the next page must continue.
func CursorFor(identity PaymentID) (Cursor, error) {
	if err := identity.Validate(); err != nil {
		return Cursor{}, contractError(err)
	}
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(CatalogCursorCommitmentDomain)); err != nil {
		return Cursor{}, contractError(err)
	}
	if _, err := writer.Write([]byte{CatalogCursorFrameSeparator}); err != nil {
		return Cursor{}, contractError(err)
	}
	if _, err := writer.Write([]byte(identity.String())); err != nil {
		return Cursor{}, contractError(err)
	}
	digest, _, err := writer.Seal()
	if err != nil {
		return Cursor{}, contractError(err)
	}
	return NewCursor(digest)
}

// Validate rejects an unset cursor.
func (c Cursor) Validate() error {
	if err := c.value.Validate(); err != nil {
		return contractError(errors.New("payment catalog cursor is invalid"), err)
	}
	return nil
}

// MarshalJSON emits the opaque digest.
func (c Cursor) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return c.value.MarshalJSON()
}

// UnmarshalJSON accepts one digest and preserves the receiver on rejection.
func (c *Cursor) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil payment cursor receiver"))
	}
	var value core.SHA256Digest
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewCursor(value)
	if err != nil {
		return jsonError(err)
	}
	*c = candidate
	return nil
}

// Selection is the exact all-or-one payment receipt selection.
type Selection struct {
	Payment PaymentID                 `json:"payment_id"`
	Kind    core.CatalogSelectionKind `json:"kind"`
}

// All selects every payment in one authenticated scope.
func All() Selection { return Selection{Kind: core.CatalogSelectionAll} }

// Specific selects one exact payment identity.
func Specific(identity PaymentID) (Selection, error) {
	candidate := Selection{Payment: identity, Kind: core.CatalogSelectionSpecific}
	return candidate, candidate.Validate()
}

// Validate enforces the exact tagged-union arm.
func (s Selection) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return contractError(err)
	}
	switch s.Kind {
	case core.CatalogSelectionAll:
		if s.Payment != (PaymentID{}) {
			return contractError(errors.New("all payment selection carries an identity"))
		}
	case core.CatalogSelectionSpecific:
		if err := s.Payment.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("payment selection escaped its domain"))
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
		Payment PaymentID                 `json:"payment_id"`
		Kind    core.CatalogSelectionKind `json:"kind"`
	}{Payment: s.Payment, Kind: s.Kind})
}

// Position is the explicit first-page or after-cursor request arm.
type Position struct {
	Cursor Cursor                   `json:"cursor"`
	Kind   core.CatalogPositionKind `json:"kind"`
}

// Start requests the first payment catalog page.
func Start() Position { return Position{Kind: core.CatalogPositionStart} }

// After requests the page after one authority-issued cursor.
func After(cursor Cursor) (Position, error) {
	candidate := Position{Cursor: cursor, Kind: core.CatalogPositionAfter}
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
			return contractError(errors.New("start payment position carries a cursor"))
		}
	case core.CatalogPositionAfter:
		if err := p.Cursor.Validate(); err != nil {
			return contractError(err)
		}
	default:
		return contractError(errors.New("payment position escaped its domain"))
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

// Query is the complete typed input behind `receipt -all` or `receipt <id>`.
type Query struct {
	Scope     receipt.Scope         `json:"scope"`
	Selection Selection             `json:"selection"`
	Position  Position              `json:"position"`
	Limit     core.CatalogPageLimit `json:"limit"`
}

// QueryRequest is the constructor boundary for one customer receipt query.
// PageSize is immediately closed into Core's nominal page limit.
type QueryRequest struct {
	Scope     receipt.Scope
	Selection Selection
	Position  Position
	PageSize  uint16
}

// Validate closes every constructor input without constructing the query.
func (r QueryRequest) Validate() error {
	_, limitErr := core.NewCatalogPageLimit(r.PageSize)
	if err := errors.Join(r.Scope.Validate(), r.Selection.Validate(), r.Position.Validate(), limitErr); err != nil {
		return contractError(err)
	}
	if r.Selection.Kind == core.CatalogSelectionSpecific && r.Position.Kind != core.CatalogPositionStart {
		return verificationError(errors.New(specificSelectionPaginationDiagnostic))
	}
	return nil
}

// NewQuery constructs one completely typed payment catalog query.
func NewQuery(request QueryRequest) (Query, error) {
	if err := request.Validate(); err != nil {
		return Query{}, err
	}
	limit, err := core.NewCatalogPageLimit(request.PageSize)
	if err != nil {
		return Query{}, contractError(err)
	}
	candidate := Query{
		Scope: request.Scope, Selection: request.Selection,
		Position: request.Position, Limit: limit,
	}
	return candidate, candidate.Validate()
}

// Validate closes scope, selection, position, and the shared page bound.
func (q Query) Validate() error {
	if err := errors.Join(q.Scope.Validate(), q.Selection.Validate(), q.Position.Validate(), q.Limit.Validate()); err != nil {
		return contractError(err)
	}
	if q.Selection.Kind == core.CatalogSelectionSpecific && q.Position.Kind != core.CatalogPositionStart {
		return verificationError(errors.New(specificSelectionPaginationDiagnostic))
	}
	return nil
}

var (
	_ core.Validatable            = Cursor{}
	_ core.Validatable            = Selection{}
	_ core.Validatable            = Position{}
	_ core.Validatable            = QueryRequest{}
	_ core.Validatable            = Query{}
	_ core.ValidatedJSONMarshaler = Cursor{}
	_ core.ValidatedJSONMarshaler = Selection{}
	_ core.ValidatedJSONMarshaler = Position{}
)
