package process

import (
	"os"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// CommandExitCode is one portable process exit code selected by caller-owned
// command policy. Primitive owns only the bounded termination mechanism; it
// does not interpret why a product selected a particular nonzero code.
type CommandExitCode uint8

const (
	// CommandExitCodeSuccess is the portable successful command result.
	CommandExitCodeSuccess CommandExitCode = 0
	// CommandExitCodeFailure is the portable generic unsuccessful result.
	CommandExitCodeFailure CommandExitCode = 1
	// CommandExitCodeMaximum is the highest portable product-selected code.
	// Values above 125 are reserved by common shells for termination facts.
	CommandExitCodeMaximum CommandExitCode = 125
)

// Validate rejects values outside the portable command-exit domain.
func (c CommandExitCode) Validate() error {
	if c > CommandExitCodeMaximum {
		return contractError("command exit code is outside the portable domain")
	}
	return nil
}

// IsValid reports whether c belongs to the portable command-exit domain.
func (c CommandExitCode) IsValid() bool { return c.Validate() == nil }

// OffWireEnum declares that CommandExitCode is an OS-facing numeric value,
// not a serialized protocol enum.
func (CommandExitCode) OffWireEnum() {}

// String returns the decimal OS-facing representation of a valid code.
func (c CommandExitCode) String() string {
	if !c.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(c), 10)
}

// Code returns the platform exit code owned by this status.
func (c CommandExitCode) Code() (int, error) {
	if err := c.Validate(); err != nil {
		return 1, err
	}
	return int(c), nil
}

// ExitCommand terminates the calling command with status. This ambient effect
// belongs only at a package-main boundary after all cleanup and diagnostics
// have completed; libraries return typed errors instead.
//
// An invalid code fails closed as CommandExitCodeFailure. It cannot be returned
// as an error because a command-exit door does not return after accepting its
// input.
func ExitCommand(exit CommandExitCode) {
	code, err := exit.Code()
	if err != nil {
		code = 1
	}
	// witness:waiver doctrine/logging/log_fatal -- this exact process-owned leaf terminates only after command cleanup and diagnostics.
	os.Exit(code) // witness:waiver doctrine/firewall/fatal_exit -- this exact process-owned leaf is the typed package-main termination boundary.
}
