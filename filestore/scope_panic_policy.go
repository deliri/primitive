package filestore

import "github.com/deliri/primitive/v2026/core"

// ScopePanicPolicy selects callback panic behavior within a native close
// lifetime. Propagation is required when a Go iterator lends its yield into
// the scope: recovering that consumer panic would violate Go's iterator rule.
type ScopePanicPolicy uint8

const (
	ScopePanicSanitize ScopePanicPolicy = iota
	ScopePanicPropagate
	scopePanicPolicyLimit
)

func (p ScopePanicPolicy) IsValid() bool { return p < scopePanicPolicyLimit }

func (p ScopePanicPolicy) Validate() error {
	if !p.IsValid() {
		return core.ErrFilestoreContract
	}
	return nil
}

func (p ScopePanicPolicy) String() string {
	switch p {
	case ScopePanicSanitize:
		return "sanitize"
	case ScopePanicPropagate:
		return "propagate"
	default:
		return core.UnknownEnumDiagnostic
	}
}

func (ScopePanicPolicy) OffWireEnum() {}

var _ core.OffWireEnum = ScopePanicSanitize
