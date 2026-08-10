package core

import (
	"encoding/json"
	"errors"
)

const (
	// CatalogPageMaximumEntries is the shared hard ceiling for one bounded
	// customer-visible catalog page.
	CatalogPageMaximumEntries     = 100
	catalogSelectionTokenAll      = "all"
	catalogSelectionTokenSpecific = "specific"
	catalogPositionTokenStart     = "start"
	catalogPositionTokenAfter     = "after"
	catalogContinuationTokenEnd   = "end"
	catalogContinuationTokenMore  = "more"
)

// CatalogPageLimit is a positive caller-selected page bound shared by every
// Primitive catalog protocol.
type CatalogPageLimit struct{ value uint16 }

// NewCatalogPageLimit closes a caller-selected page bound.
func NewCatalogPageLimit(value uint16) (CatalogPageLimit, error) {
	candidate := CatalogPageLimit{value: value}
	if err := candidate.Validate(); err != nil {
		return CatalogPageLimit{}, err
	}
	return candidate, nil
}

// Validate refuses zero and values wider than one bounded page.
func (l CatalogPageLimit) Validate() error {
	if l.value == 0 || l.value > CatalogPageMaximumEntries {
		return catalogContractError("catalog page limit is outside its bound")
	}
	return nil
}

// Uint16 returns the validated page bound.
func (l CatalogPageLimit) Uint16() uint16 { return l.value }

// MarshalJSON emits the canonical unsigned decimal page bound.
func (l CatalogPageLimit) MarshalJSON() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return json.Marshal(l.value)
}

// UnmarshalJSON accepts only the canonical unsigned decimal page bound.
func (l *CatalogPageLimit) UnmarshalJSON(data []byte) error {
	if l == nil {
		return errors.Join(ErrJSONContract, catalogContractError("nil catalog page limit receiver"))
	}
	var value uint16
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.Join(ErrJSONContract, catalogContractError("catalog page limit JSON is invalid"), err)
	}
	candidate, err := NewCatalogPageLimit(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || string(canonical) != string(data) {
		return errors.Join(ErrJSONContract, catalogContractError("catalog page limit JSON is not canonical"), err)
	}
	*l = candidate
	return nil
}

// CatalogSelectionKind closes the shared all-or-specific query domain.
type CatalogSelectionKind uint8

const (
	// CatalogSelectionUnknown is the invalid zero selection.
	CatalogSelectionUnknown CatalogSelectionKind = iota
	// CatalogSelectionAll selects every item in one scope.
	CatalogSelectionAll
	// CatalogSelectionSpecific selects one exact item.
	CatalogSelectionSpecific
	catalogSelectionLimit
)

func catalogSelectionTokens() [catalogSelectionLimit]string {
	return [...]string{"", catalogSelectionTokenAll, catalogSelectionTokenSpecific}
}

// Validate rejects selection kinds outside the closed domain.
func (k CatalogSelectionKind) Validate() error {
	if k <= CatalogSelectionUnknown || k >= catalogSelectionLimit || catalogSelectionTokens()[k] == "" {
		return catalogContractError("catalog selection kind is invalid")
	}
	return nil
}

// IsValid reports whether k is one published catalog selection kind.
func (k CatalogSelectionKind) IsValid() bool { return k.Validate() == nil }

// String returns the canonical token or empty text for an invalid kind.
func (k CatalogSelectionKind) String() string {
	if k >= catalogSelectionLimit {
		return ""
	}
	return catalogSelectionTokens()[k]
}

// MarshalJSON emits the canonical selection token.
func (k CatalogSelectionKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(k.String())
}

// UnmarshalJSON accepts only an admitted selection token.
func (k *CatalogSelectionKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(ErrJSONContract, catalogContractError("nil catalog selection receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	for candidate := CatalogSelectionUnknown + 1; candidate < catalogSelectionLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(ErrJSONContract, catalogContractError("catalog selection token is unsupported"))
}

// CatalogPositionKind closes the shared first-page-or-after-cursor domain.
type CatalogPositionKind uint8

const (
	// CatalogPositionUnknown is the invalid zero position.
	CatalogPositionUnknown CatalogPositionKind = iota
	// CatalogPositionStart requests the first page.
	CatalogPositionStart
	// CatalogPositionAfter requests the page after a cursor.
	CatalogPositionAfter
	catalogPositionLimit
)

func catalogPositionTokens() [catalogPositionLimit]string {
	return [...]string{"", catalogPositionTokenStart, catalogPositionTokenAfter}
}

// Validate rejects positions outside the closed domain.
func (k CatalogPositionKind) Validate() error {
	if k <= CatalogPositionUnknown || k >= catalogPositionLimit || catalogPositionTokens()[k] == "" {
		return catalogContractError("catalog position kind is invalid")
	}
	return nil
}

// IsValid reports whether k is one published catalog position kind.
func (k CatalogPositionKind) IsValid() bool { return k.Validate() == nil }

// String returns the canonical token or empty text for an invalid kind.
func (k CatalogPositionKind) String() string {
	if k >= catalogPositionLimit {
		return ""
	}
	return catalogPositionTokens()[k]
}

// MarshalJSON emits the canonical position token.
func (k CatalogPositionKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(k.String())
}

// UnmarshalJSON accepts only an admitted position token.
func (k *CatalogPositionKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(ErrJSONContract, catalogContractError("nil catalog position receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	for candidate := CatalogPositionUnknown + 1; candidate < catalogPositionLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(ErrJSONContract, catalogContractError("catalog position token is unsupported"))
}

// CatalogContinuationState closes the shared end-or-more response domain.
type CatalogContinuationState uint8

const (
	// CatalogContinuationUnknown is the invalid zero continuation state.
	CatalogContinuationUnknown CatalogContinuationState = iota
	// CatalogContinuationEnd states that no later page exists.
	CatalogContinuationEnd
	// CatalogContinuationMore states that another page exists.
	CatalogContinuationMore
	catalogContinuationLimit
)

func catalogContinuationTokens() [catalogContinuationLimit]string {
	return [...]string{"", catalogContinuationTokenEnd, catalogContinuationTokenMore}
}

// Validate rejects continuation states outside the closed domain.
func (s CatalogContinuationState) Validate() error {
	if s <= CatalogContinuationUnknown || s >= catalogContinuationLimit || catalogContinuationTokens()[s] == "" {
		return catalogContractError("catalog continuation state is invalid")
	}
	return nil
}

// IsValid reports whether s is one published continuation state.
func (s CatalogContinuationState) IsValid() bool { return s.Validate() == nil }

// String returns the canonical token or empty text for an invalid state.
func (s CatalogContinuationState) String() string {
	if s >= catalogContinuationLimit {
		return ""
	}
	return catalogContinuationTokens()[s]
}

// MarshalJSON emits the canonical continuation token.
func (s CatalogContinuationState) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(s.String())
}

// UnmarshalJSON accepts only an admitted continuation token.
func (s *CatalogContinuationState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(ErrJSONContract, catalogContractError("nil catalog continuation receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	for candidate := CatalogContinuationUnknown + 1; candidate < catalogContinuationLimit; candidate++ {
		if candidate.String() == value {
			*s = candidate
			return nil
		}
	}
	return errors.Join(ErrJSONContract, catalogContractError("catalog continuation token is unsupported"))
}

func catalogContractError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}

var (
	_ Validatable = CatalogPageLimit{}
	_ Validatable = CatalogSelectionUnknown
	_ Validatable = CatalogPositionUnknown
	_ Validatable = CatalogContinuationUnknown
)
