package process

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// Isolation selects how the direct child is placed relative to the caller.
//
// The distinction exists because tools spawn trees. A child sharing the
// caller's process group can only ever be addressed one process at a time,
// so a cancellation reaches the direct child and leaves its descendants
// running. A child leading its own group gives every later signal one
// address for the whole tree.
type Isolation uint8

const (
	// IsolationUnknown is outside the admitted domain.
	IsolationUnknown Isolation = iota
	// IsolationDirect leaves the child in the caller's process group and
	// addresses signals to exactly the direct child.
	IsolationDirect
	// IsolationGroup starts the child as the leader of a new process group
	// and addresses signals to the whole group.
	IsolationGroup
	isolationLimit
)

func isolationDiagnostics() [isolationLimit]string {
	return [...]string{
		IsolationDirect: "direct",
		IsolationGroup:  "group",
	}
}

// isolationOutsideDomainDiagnostic is the one spelling of the isolation
// refusal, shared by the enum's own gate and every platform leaf's
// fail-closed default arm.
const isolationOutsideDomainDiagnostic = "isolation is outside the admitted domain"

// cancelSignalOutsideDomainDiagnostic is the one spelling of the cancel
// signal refusal, shared by the enum's own gate and the platform leaf's
// fail-closed default arm.
const cancelSignalOutsideDomainDiagnostic = "cancel signal is outside the admitted domain"

// Validate rejects values outside the closed isolation domain.
func (i Isolation) Validate() error {
	if !i.IsValid() {
		return contractError(isolationOutsideDomainDiagnostic)
	}
	return nil
}

// IsValid reports whether i is admitted.
func (i Isolation) IsValid() bool {
	diagnostics := isolationDiagnostics()
	return i > IsolationUnknown && i < isolationLimit && diagnostics[i] != ""
}

// OffWireEnum declares that Isolation is not a wire encoding.
func (Isolation) OffWireEnum() {}

// String returns the compiler-owned label for i.
func (i Isolation) String() string {
	diagnostics := isolationDiagnostics()
	if i < isolationLimit && diagnostics[i] != "" {
		return diagnostics[i]
	}
	return core.UnknownEnumDiagnostic
}

// CancelSignal is the closed set of signals a cancellation may deliver.
//
// The choice is a real contract with the child, not a preference: a Go
// process receiving SIGQUIT prints every goroutine stack before dying, which
// is exactly the hang-site evidence a supervisor wants from a wedged tool,
// while SIGKILL ends it silently and SIGINT and SIGTERM invite an orderly
// exit. Whichever signal is chosen, os/exec still escalates to a hard kill
// of the direct child after Request.WaitDelay.
type CancelSignal uint8

const (
	// CancelSignalUnknown is outside the admitted domain.
	CancelSignalUnknown CancelSignal = iota
	// CancelSignalKill ends the child immediately and silently.
	CancelSignalKill
	// CancelSignalQuit asks a Go child to dump its goroutine stacks and die.
	CancelSignalQuit
	// CancelSignalInterrupt delivers the interactive interrupt.
	CancelSignalInterrupt
	// CancelSignalTerminate invites an orderly exit.
	CancelSignalTerminate
	cancelSignalLimit
)

func cancelSignalDiagnostics() [cancelSignalLimit]string {
	return [...]string{
		CancelSignalKill:      "kill",
		CancelSignalQuit:      "quit",
		CancelSignalInterrupt: core.SignalInterruptLabel,
		CancelSignalTerminate: core.SignalTerminateLabel,
	}
}

// Validate rejects values outside the closed cancel-signal domain.
func (s CancelSignal) Validate() error {
	if !s.IsValid() {
		return contractError(cancelSignalOutsideDomainDiagnostic)
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s CancelSignal) IsValid() bool {
	diagnostics := cancelSignalDiagnostics()
	return s > CancelSignalUnknown && s < cancelSignalLimit && diagnostics[s] != ""
}

// OffWireEnum declares that CancelSignal is not a wire encoding.
func (CancelSignal) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s CancelSignal) String() string {
	diagnostics := cancelSignalDiagnostics()
	if s < cancelSignalLimit && diagnostics[s] != "" {
		return diagnostics[s]
	}
	return core.UnknownEnumDiagnostic
}

// Containment states how the direct child is isolated and how a cancellation
// reaches it. The two facts decide who receives which signal, and they are
// stated together or not at all: a fully zero Containment names the one
// documented conservative shape, a direct child killed on cancellation,
// because running one command and reading its output is the common case and
// spelling the same two members in every such request is ceremony. Naming one
// member and leaving the other zero is a real mistake and stays invalid, so a
// half-stated intent surfaces instead of being papered over.
type Containment struct {
	Isolation    Isolation
	CancelSignal CancelSignal
}

// Validate closes both containment members.
func (c Containment) Validate() error {
	if err := c.Isolation.Validate(); err != nil {
		return err
	}
	return c.CancelSignal.Validate()
}

// orDefault supplies the conservative containment a caller that named none
// wants: a direct child, killed on cancellation. Running one command and
// reading its output is the common case, and forcing every such caller to
// spell out the same two members would be ceremony, so the fully zero value
// is filled here. A partially named containment is not defaulted: naming one
// member and not the other is a mistake the caller must complete, and it stays
// invalid so the mistake surfaces rather than being papered over.
func (c Containment) orDefault() Containment {
	if c.Isolation == IsolationUnknown && c.CancelSignal == CancelSignalUnknown {
		return Containment{Isolation: IsolationDirect, CancelSignal: CancelSignalKill}
	}
	return c
}

// signalDelivery is the typed handoff one signal delivery rides from the
// execution that owns the child to the platform leaf that addresses it.
type signalDelivery struct {
	process     *os.Process
	identity    ProcessIdentity
	containment Containment
	signal      CancelSignal
}

// ProcessIdentity is the platform identifier of one started direct child,
// held for signaling and for durable execution records.
//
// An identity is a name, not a capability: the kernel may hand the number to
// another process once the observed one is fully gone, so a holder signals
// through the Execution that owns the child rather than storing the number
// and signaling later.
type ProcessIdentity int32

// Validate rejects the zero and negative identifiers no real child carries.
func (p ProcessIdentity) Validate() error {
	if p <= 0 {
		return contractError("process identity is outside the admitted domain")
	}
	return nil
}

// Int returns the identifier for a durable record or diagnostic.
func (p ProcessIdentity) Int() (int, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	return int(p), nil
}
