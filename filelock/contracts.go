package filelock

import (
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// Exclusivity declares how many holders the lock admits at once.
type Exclusivity uint8

const (
	// ExclusivityUnknown is the invalid zero state.
	ExclusivityUnknown Exclusivity = iota
	// Exclusive admits one holder and excludes every other, shared or not.
	Exclusive
	// Shared admits any number of shared holders and excludes an exclusive
	// one. Acquiring it immediately is how a caller asks whether an exclusive
	// holder exists without disturbing one.
	Shared
	exclusivityLimit
)

func exclusivityDiagnostics() [exclusivityLimit]string {
	return [exclusivityLimit]string{
		Exclusive: "exclusive",
		Shared:    "shared",
	}
}

// Validate rejects values outside the closed exclusivity domain.
func (e Exclusivity) Validate() error {
	if !e.IsValid() {
		return contractError(errors.New("filelock exclusivity is invalid"))
	}
	return nil
}

// IsValid reports membership in the closed exclusivity domain.
func (e Exclusivity) IsValid() bool {
	return e > ExclusivityUnknown && e < exclusivityLimit && exclusivityDiagnostics()[e] != ""
}

// OffWireEnum declares Exclusivity as local locking policy rather than a wire
// encoding.
func (Exclusivity) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for e.
func (e Exclusivity) String() string {
	if !e.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return exclusivityDiagnostics()[e]
}

// Patience declares what an acquisition does when another process holds the
// lock.
type Patience uint8

const (
	// PatienceUnknown is the invalid zero state.
	PatienceUnknown Patience = iota
	// Immediate refuses rather than waiting, and reports the refusal as a
	// contended acquisition rather than as a failure.
	Immediate
	// Blocking waits for the current holder to release. Cancellation cannot
	// reach a process parked in the lock call, so a caller that must stay
	// interruptible uses Immediate and decides its own retry cadence.
	Blocking
	patienceLimit
)

func patienceDiagnostics() [patienceLimit]string {
	return [patienceLimit]string{
		Immediate: "immediate",
		Blocking:  "blocking",
	}
}

// Validate rejects values outside the closed patience domain.
func (p Patience) Validate() error {
	if !p.IsValid() {
		return contractError(errors.New("filelock patience is invalid"))
	}
	return nil
}

// IsValid reports membership in the closed patience domain.
func (p Patience) IsValid() bool {
	return p > PatienceUnknown && p < patienceLimit && patienceDiagnostics()[p] != ""
}

// OffWireEnum declares Patience as local locking policy rather than a wire
// encoding.
func (Patience) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for p.
func (p Patience) String() string {
	if !p.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return patienceDiagnostics()[p]
}

// Request names one advisory lock attempt on one open file.
type Request struct {
	File        *os.File
	Exclusivity Exclusivity
	Patience    Patience
}

// Validate rejects a missing file or an unset locking intent.
func (r Request) Validate() error {
	if r.File == nil {
		return contractError(errors.New("filelock file is missing"))
	}
	if err := r.Exclusivity.Validate(); err != nil {
		return err
	}
	return r.Patience.Validate()
}

// Acquisition is the outcome of one attempt. Its field is unexported and
// reachable only through an accessor that revalidates, so a caller cannot
// assemble a hold it never took.
type Acquisition struct {
	held  bool
	valid bool
}

// Validate rejects an outcome that was never produced by an attempt.
func (a Acquisition) Validate() error {
	if !a.valid {
		return contractError(errors.New("filelock acquisition was not produced by an attempt"))
	}
	return nil
}

// Held reports whether this process now holds the lock.
//
// False is only ever returned for an Immediate attempt that found another
// holder. It is a fact about contention, not a failure, which is why it is not
// an error: a caller deciding whether to start work needs to tell "somebody
// else is running" apart from "locking is broken here".
func (a Acquisition) Held() (bool, error) {
	if err := a.Validate(); err != nil {
		return false, err
	}
	return a.held, nil
}

func newAcquisition(held bool) Acquisition {
	return Acquisition{held: held, valid: true}
}

var (
	_ core.Validatable = ExclusivityUnknown
	_ core.Validatable = PatienceUnknown
	_ core.Validatable = Request{}
	_ core.Validatable = Acquisition{}
	_ core.OffWireEnum = ExclusivityUnknown
	_ core.OffWireEnum = PatienceUnknown
)
