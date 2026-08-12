package wiring

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

// ErrorKind identifies which wiring invariant refused a runtime graph.
type ErrorKind uint8

const (
	ErrorKindUnknown ErrorKind = iota
	ErrorKindRequest
	ErrorKindComponent
	ErrorKindDependency
	ErrorKindPrimitiveDoor
	ErrorKindDuplicateComponent
	ErrorKindDuplicateDependency
	ErrorKindDuplicatePrimitiveDoor
	ErrorKindMissingRoot
	ErrorKindMissingDependency
	ErrorKindCycle
	ErrorKindDisconnected
	errorKindLimit
)

func errorKindDiagnostics() [errorKindLimit]string {
	return [...]string{
		ErrorKindRequest:                "runtime wiring request rejected",
		ErrorKindComponent:              "runtime wiring component rejected",
		ErrorKindDependency:             "runtime wiring dependency rejected",
		ErrorKindPrimitiveDoor:          "runtime wiring Primitive door rejected",
		ErrorKindDuplicateComponent:     "runtime wiring component duplicated",
		ErrorKindDuplicateDependency:    "runtime wiring dependency duplicated",
		ErrorKindDuplicatePrimitiveDoor: "runtime wiring Primitive door duplicated",
		ErrorKindMissingRoot:            "runtime wiring root is absent",
		ErrorKindMissingDependency:      "runtime wiring dependency is absent",
		ErrorKindCycle:                  "runtime wiring contains a cycle",
		ErrorKindDisconnected:           "runtime wiring component is disconnected",
	}
}

func errorKindTokens() [errorKindLimit]string {
	return [...]string{
		ErrorKindRequest:                "request",
		ErrorKindComponent:              "component",
		ErrorKindDependency:             "dependency",
		ErrorKindPrimitiveDoor:          "primitive_door",
		ErrorKindDuplicateComponent:     "duplicate_component",
		ErrorKindDuplicateDependency:    "duplicate_dependency",
		ErrorKindDuplicatePrimitiveDoor: "duplicate_primitive_door",
		ErrorKindMissingRoot:            "missing_root",
		ErrorKindMissingDependency:      "missing_dependency",
		ErrorKindCycle:                  "cycle",
		ErrorKindDisconnected:           "disconnected",
	}
}

// Validate rejects the undefined zero and every out-of-domain error kind.
func (k ErrorKind) Validate() error {
	if k <= ErrorKindUnknown || k >= errorKindLimit ||
		errorKindDiagnostics()[k] == "" || errorKindTokens()[k] == "" {
		return core.ErrPrimitiveContract
	}
	return nil
}

// IsValid reports whether k belongs to the closed error-kind domain.
func (k ErrorKind) IsValid() bool { return k.Validate() == nil }

// String returns the canonical token, or an empty string for an invalid kind.
func (k ErrorKind) String() string {
	if !k.IsValid() {
		return ""
	}
	return errorKindTokens()[k]
}

// MarshalJSON emits the canonical error-kind token.
func (k ErrorKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

// UnmarshalJSON accepts only one canonical error-kind token and preserves the
// receiver when external input is rejected.
func (k *ErrorKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := parseErrorKind(token)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*k = parsed
	return nil
}

func parseErrorKind(token string) (ErrorKind, error) {
	for kind := ErrorKindUnknown + 1; kind < errorKindLimit; kind++ {
		if kind.String() == token {
			return kind, nil
		}
	}
	return ErrorKindUnknown, core.ErrPrimitiveContract
}

// ContractError carries the typed owner and peer involved in one rejected
// runtime wiring invariant while preserving Primitive's stable error identity.
type ContractError[I Identity] struct {
	Cause         error
	Owner         I
	Peer          I
	PrimitiveDoor core.PackageIdentity
	Kind          ErrorKind
}

// Error renders the compiler-owned error kind diagnostic.
func (e *ContractError[I]) Error() string {
	if e == nil || e.Kind.Validate() != nil {
		return core.ErrPrimitiveContract.Error()
	}
	return fmt.Sprintf("%s: %v", errorKindDiagnostics()[e.Kind], e.Cause)
}

// Unwrap preserves both Primitive's stable contract identity and the owning
// identity's native validation cause.
func (e *ContractError[I]) Unwrap() []error {
	if e == nil {
		return []error{core.ErrPrimitiveContract}
	}
	return []error{core.ErrPrimitiveContract, e.Cause}
}

type contractErrorRequest[I Identity] struct {
	cause         error
	owner         I
	peer          I
	primitiveDoor core.PackageIdentity
	kind          ErrorKind
}

func wiringContractError[I Identity](request contractErrorRequest[I]) error {
	if request.cause == nil {
		request.cause = core.ErrPrimitiveContract
	}
	return &ContractError[I]{
		Kind: request.kind, Owner: request.owner, Peer: request.peer,
		PrimitiveDoor: request.primitiveDoor, Cause: request.cause,
	}
}

var _ core.Validatable = ErrorKindUnknown
