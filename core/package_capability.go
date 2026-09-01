package core

import "errors"

const packageCapabilityProcessExecutionName = "process_execution"

// PackageCapability is a compiler-visible declaration of a package-owned
// execution boundary recognized by doctrine tooling.
type PackageCapability uint8

const (
	// PackageCapabilityUnknown is outside the declared capability domain.
	PackageCapabilityUnknown PackageCapability = iota
	// PackageCapabilityProcessExecution permits a package to execute reviewed
	// process plans through Primitive's process capability.
	PackageCapabilityProcessExecution
)

// Validate rejects values outside the closed capability domain.
func (c PackageCapability) Validate() error {
	if !c.IsValid() {
		return errors.Join(ErrPrimitiveContract, errors.New("package capability is outside the declared domain"))
	}
	return nil
}

// IsValid reports whether the capability is part of the closed domain.
func (c PackageCapability) IsValid() bool {
	return c == PackageCapabilityProcessExecution
}

// OffWireEnum marks PackageCapability as a compiler-only enum.
func (PackageCapability) OffWireEnum() {}

// String returns the doctrine identifier for a valid capability.
func (c PackageCapability) String() string {
	if !c.IsValid() {
		return UnknownEnumDiagnostic
	}
	return packageCapabilityProcessExecutionName
}

var _ OffWireEnum = PackageCapabilityUnknown
