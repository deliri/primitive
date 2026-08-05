package controlplane

import "github.com/deliri/primitive/v2026/core"

// ResponseBindingError is a sealed report that an authentic, well-formed
// response belongs to a different request than the one the caller made.
//
// Sealed because the field it names is a closed domain: a caller may read which
// fact disagreed and switch on it, but cannot construct a binding failure that
// names something outside the header. The interface is satisfied only by this
// package's own unexported type.
type ResponseBindingError interface {
	error
	// Field names the exact bound fact that disagreed.
	Field() ResponseHeaderField
	controlPlaneResponseBinding()
}

type responseBindingError struct {
	field ResponseHeaderField
}

// NewResponseBindingError reports that one exact bound fact disagreed.
//
// A field outside the closed domain is itself a contract violation and returns
// the header identity instead, so a caller can never receive a binding failure
// that names nothing.
func NewResponseBindingError(field ResponseHeaderField) error {
	if err := field.Validate(); err != nil {
		return err
	}
	return responseBindingError{field: field}
}

func (e responseBindingError) Field() ResponseHeaderField { return e.field }

func (responseBindingError) controlPlaneResponseBinding() {}

// Error names the disagreeing fact.
func (e responseBindingError) Error() string {
	return "control-plane response does not bind to its request: " + e.field.String()
}

// Unwrap keeps the package, binding, and Primitive identities reachable through
// errors.Is while the concrete type stays reachable through errors.As.
func (e responseBindingError) Unwrap() []error {
	return []error{core.ErrControlPlaneContract, core.ErrControlPlaneResponseBinding}
}

var (
	_ ResponseBindingError = responseBindingError{}
	_ error                = responseBindingError{}
)
