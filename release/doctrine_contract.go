package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const packageCapabilityProcessExecutionLabel = "process_execution"

// PackageCapability is the compiler-visible domain used by Witness to prove
// that this package deliberately owns a process execution boundary.
type PackageCapability uint8

const (
	PackageCapabilityUnknown PackageCapability = iota
	PackageCapabilityProcessExecution
)

const doctrinePackageCapability PackageCapability = PackageCapabilityProcessExecution

func (c PackageCapability) Validate() error {
	if !c.IsValid() {
		return contractError(errors.New("package capability is outside the declared domain"))
	}
	return nil
}

func (c PackageCapability) IsValid() bool {
	return c == PackageCapabilityProcessExecution
}

func (PackageCapability) OffWireEnum() {}

func (c PackageCapability) String() string {
	if !c.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return packageCapabilityProcessExecutionLabel
}

func validatePackageCapability() error {
	doctrinePackageCapability.OffWireEnum()
	if err := doctrinePackageCapability.Validate(); err != nil {
		return err
	}
	if doctrinePackageCapability.String() != packageCapabilityProcessExecutionLabel {
		return contractError(errors.New("package capability label differs from its declaration"))
	}
	return nil
}

var (
	_ PackageCapability = doctrinePackageCapability
	_ core.OffWireEnum  = PackageCapabilityUnknown
)
