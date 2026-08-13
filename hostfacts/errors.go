package hostfacts

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

// Operation identifies the exact Hostfacts operation that failed.
type Operation uint8

const (
	OperationUnknown Operation = iota
	OperationOpenRoot
	OperationDiskCapacity
	OperationGoMemory
	OperationPhysicalMemory
	OperationCgroupMembership
	OperationCgroupMount
	OperationCgroupLimit
	OperationTreeWalk
	OperationGoOOMBanner
	OperationTerminalGeometry
	OperationDiskRotation
	OperationLogicalCPUCount
	operationLimit
)

func operationLabels() [operationLimit]string {
	return [...]string{
		OperationOpenRoot:         "open root",
		OperationDiskCapacity:     "disk capacity",
		OperationGoMemory:         "go memory",
		OperationPhysicalMemory:   "physical memory",
		OperationCgroupMembership: "cgroup membership",
		OperationCgroupMount:      "cgroup mount",
		OperationCgroupLimit:      "cgroup limit",
		OperationTreeWalk:         "tree walk",
		OperationGoOOMBanner:      "go OOM banner",
		OperationTerminalGeometry: "terminal geometry",
		OperationDiskRotation:     "disk rotation",
		OperationLogicalCPUCount:  "logical CPU count",
	}
}

// String names the operation for diagnostics. Failure renders it, so the name is
// part of the package's external output rather than decoration: without it a
// diagnostic carries an opaque ordinal that a reader has to decode against this
// file to learn which observation failed.
func (o Operation) String() string {
	if !o.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return operationLabels()[o]
}

// OffWireEnum declares Operation as an in-process failure classification
// rather than a wire encoding.
func (Operation) OffWireEnum() {}

// Validate rejects operations outside the closed domain.
func (o Operation) Validate() error {
	if !o.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("host facts operation is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed operation domain.
func (o Operation) IsValid() bool {
	return o > OperationUnknown && o < operationLimit && operationLabels()[o] != ""
}

// Failure carries the typed operation while preserving stable and native
// causes through errors.Is and errors.As.
type Failure struct {
	Cause     error
	Identity  core.ErrorIdentity
	Operation Operation
}

// Validate rejects an invalid operation or a non-Hostfacts identity.
func (e Failure) Validate() error {
	if err := errors.Join(e.Operation.Validate(), e.Identity.Validate()); err != nil {
		return err
	}
	if !errors.Is(e.Identity, core.ErrHostFacts) {
		return core.ErrHostFactsContract
	}
	return nil
}

// Error renders the operation and its causes for diagnostics.
func (e Failure) Error() string {
	detail := e.Cause
	if detail == nil {
		detail = e.Identity
	} else if !errors.Is(detail, e.Identity) {
		detail = errors.Join(e.Identity, detail)
	}
	return fmt.Sprintf("host facts %s: %v", e.Operation.String(), detail)
}

// Unwrap exposes both the stable identity and native cause.
func (e Failure) Unwrap() []error {
	if e.Cause == nil {
		return []error{e.Identity}
	}
	return []error{e.Identity, e.Cause}
}

func fail(operation Operation, identity core.ErrorIdentity, cause error) error {
	failure := Failure{Operation: operation, Identity: identity, Cause: cause}
	if err := failure.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	return failure
}

func failRootOpen(operation Operation, identity core.ErrorIdentity, cause error) error {
	if errors.Is(cause, core.ErrHostFactsContract) {
		identity = core.ErrHostFactsContract
	}
	return fail(operation, identity, cause)
}
