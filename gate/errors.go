package gate

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	authorizeRequestBoundaryText = "authorize-request"
	newWorkPermitBoundaryText    = "new-work-permit"
	denialBoundaryText           = "denial"
)

// ContractBoundary identifies the Gate-owned boundary that rejected a value.
type ContractBoundary uint8

const (
	ContractBoundaryUnknown ContractBoundary = iota
	ContractBoundaryAuthorizeRequest
	ContractBoundaryNewWorkPermit
	ContractBoundaryDenial
	contractBoundaryLimit
)

func contractBoundaryLabels() [contractBoundaryLimit]string {
	return [...]string{
		"",
		authorizeRequestBoundaryText,
		newWorkPermitBoundaryText,
		denialBoundaryText,
	}
}

// Validate rejects values outside the closed diagnostic boundary domain.
func (b ContractBoundary) Validate() error {
	if b <= ContractBoundaryUnknown || b >= contractBoundaryLimit ||
		contractBoundaryLabels()[b] == "" {
		return core.ErrGateContract
	}
	return nil
}

// IsValid reports membership in the closed diagnostic boundary domain.
func (b ContractBoundary) IsValid() bool { return b.Validate() == nil }

// OffWireEnum declares ContractBoundary as a deliberate off-wire enum.
func (ContractBoundary) OffWireEnum() {}

// String returns the compiler-owned diagnostic label.
func (b ContractBoundary) String() string {
	if b.Validate() != nil {
		return ""
	}
	return contractBoundaryLabels()[b]
}

// ContractError identifies the exact Gate boundary that rejected a contract.
type ContractError struct {
	boundary ContractBoundary
}

// Validate rejects an unset or unknown diagnostic.
func (e ContractError) Validate() error { return e.boundary.Validate() }

// Error returns the stable Core-owned contract diagnostic.
func (ContractError) Error() string { return core.ErrGateContract.Error() }

// Unwrap preserves Gate's stable error identity.
func (ContractError) Unwrap() error { return core.ErrGateContract }

// Boundary returns the exact rejected Gate boundary.
func (e ContractError) Boundary() ContractBoundary { return e.boundary }

func contractError(boundary ContractBoundary, causes ...error) error {
	diagnostic := ContractError{boundary: boundary}
	return errors.Join(append([]error{diagnostic}, causes...)...)
}

var _ core.OffWireEnum = ContractBoundaryUnknown
