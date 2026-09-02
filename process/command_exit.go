package process

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// ExitStatus is the closed result of one command boundary. It deliberately
// carries only success or failure: richer operational failures are reported
// before the command reaches this final ambient effect.
type ExitStatus uint8

const (
	// ExitStatusUnknown is outside the admitted command-exit domain.
	ExitStatusUnknown ExitStatus = iota
	// ExitStatusSuccess terminates a command successfully.
	ExitStatusSuccess
	// ExitStatusFailure terminates a command unsuccessfully.
	ExitStatusFailure
)

// Validate rejects values outside the closed command-exit domain.
func (s ExitStatus) Validate() error {
	if s != ExitStatusSuccess && s != ExitStatusFailure {
		return contractError("command exit status is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether s belongs to the closed command-exit domain.
func (s ExitStatus) IsValid() bool { return s.Validate() == nil }

// OffWireEnum marks ExitStatus as a compiler-only enum.
func (ExitStatus) OffWireEnum() {}

// String returns the stable command-boundary identity of a valid status.
func (s ExitStatus) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	if s == ExitStatusSuccess {
		return "success"
	}
	return "failure"
}

var _ core.OffWireEnum = ExitStatusUnknown

// Code returns the platform exit code owned by this status.
func (s ExitStatus) Code() (int, error) {
	if err := s.Validate(); err != nil {
		return 1, err
	}
	if s == ExitStatusSuccess {
		return 0, nil
	}
	return 1, nil
}

// ExitCommand terminates the calling command with status. This ambient effect
// belongs only at a package-main boundary after all cleanup and diagnostics
// have completed; libraries return typed errors instead.
//
// An invalid status fails closed as ExitStatusFailure. It cannot be returned
// as an error because a command-exit door does not return after accepting its
// input.
func ExitCommand(status ExitStatus) {
	code, err := status.Code()
	if err != nil {
		code = 1
	}
	// witness:waiver doctrine/logging/log_fatal -- this exact process-owned leaf terminates only after command cleanup and diagnostics.
	os.Exit(code) // witness:waiver doctrine/firewall/fatal_exit -- this exact process-owned leaf is the typed package-main termination boundary.
}
