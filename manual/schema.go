package manual

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// NewSchema accepts the published schema token.
func NewSchema(value string) (Schema, error) {
	if value != SchemaV1Token {
		return SchemaUnknown, contractError("manual report schema is unsupported")
	}
	return SchemaV1, nil
}

// Validate rejects the zero value and unknown schema values.
func (s Schema) Validate() error {
	if s != SchemaV1 {
		return contractError("manual report schema is unsupported")
	}
	return nil
}

// IsValid reports whether the schema is published.
func (s Schema) IsValid() bool { return s.Validate() == nil }

// String returns the published token or the shared unknown-enum diagnostic.
func (s Schema) String() string {
	if s != SchemaV1 {
		return core.UnknownEnumDiagnostic
	}
	return SchemaV1Token
}

// MarshalJSON emits the validated canonical schema token.
func (s Schema) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(s.String())
}

// UnmarshalJSON validates before replacing the receiver.
func (s *Schema) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, contractError("manual schema receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	got, err := NewSchema(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*s = got
	return nil
}
