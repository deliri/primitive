package exchange

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Method is the closed HTTP request-method domain Exchange supports.
type Method uint8

const (
	MethodUnknown Method = iota
	MethodGet
	MethodHead
	MethodPost
	MethodPut
	MethodPatch
	MethodDelete
	MethodOptions
	methodLimit
)

func methodFacts() [methodLimit]string {
	return [...]string{
		MethodUnknown: "",
		MethodGet:     "GET",
		MethodHead:    "HEAD",
		MethodPost:    "POST",
		MethodPut:     "PUT",
		MethodPatch:   "PATCH",
		MethodDelete:  "DELETE",
		MethodOptions: "OPTIONS",
	}
}

func parseMethod(value string) (Method, error) {
	for method := MethodGet; method < methodLimit; method++ {
		if method.String() == value {
			return method, nil
		}
	}
	return MethodUnknown, errors.Join(
		core.ErrExchangeContract,
		errors.New("http method is not admitted"),
	)
}

// String returns the standard-library HTTP token, or empty text when invalid.
func (m Method) String() string {
	if m >= methodLimit {
		return ""
	}
	return methodFacts()[m]
}

// Validate rejects methods outside the supported domain.
func (m Method) Validate() error {
	if m <= MethodUnknown || m >= methodLimit || methodFacts()[m] == "" {
		return errors.Join(core.ErrExchangeContract, errors.New("http method is invalid"))
	}
	return nil
}

// IsValid reports whether m belongs to the supported domain.
func (m Method) IsValid() bool { return m.Validate() == nil }

// MarshalJSON emits the exact standard HTTP token.
func (m Method) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}

// UnmarshalJSON accepts one exact method supported by Exchange.
func (m *Method) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrExchangeContract, errors.New("nil http method receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	parsed, err := parseMethod(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = parsed
	return nil
}
