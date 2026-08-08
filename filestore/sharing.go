package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Sharing reports whether another process holds one path against opening.
type Sharing uint8

const (
	// SharingUnknown is outside the admitted domain.
	SharingUnknown Sharing = iota
	// SharingAvailable reports the path opened without a sharing conflict.
	SharingAvailable
	// SharingHeld reports another process holds the path against opening.
	SharingHeld
	sharingLimit
)

func sharingDiagnostics() [sharingLimit]string {
	return [...]string{
		SharingAvailable: "available",
		SharingHeld:      "held",
	}
}

// Validate rejects values outside the closed sharing domain.
func (s Sharing) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("sharing observation is outside the admitted domain"))
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s Sharing) IsValid() bool {
	diagnostics := sharingDiagnostics()
	return s > SharingUnknown && s < sharingLimit && diagnostics[s] != ""
}

// OffWireEnum declares that Sharing is not a wire encoding.
func (Sharing) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s Sharing) String() string {
	diagnostics := sharingDiagnostics()
	if s < sharingLimit && diagnostics[s] != "" {
		return diagnostics[s]
	}
	return core.UnknownEnumDiagnostic
}

// ObserveSharing asks whether another process currently holds one path
// against opening, on the one platform whose open semantics can answer.
//
// Windows opens carry share modes, so a probing open either succeeds or is
// refused with a sharing violation, and a product deciding whether a stale
// lock file may be reclaimed needs exactly that fact there. POSIX hosts
// refuse: opens do not contend, so the question has no kernel answer and the
// supported spelling is composing lsof through Process, which this door will
// not hide behind a lookalike. The probe is an observation of one moment,
// never a reservation: nothing stops the answer changing the instant it is
// returned, and ownership decisions stay with the advisory lock that guards
// the record.
func ObserveSharing(ctx context.Context, path core.AbsolutePath) (Sharing, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return SharingUnknown, err
	}
	if err := path.Validate(); err != nil {
		return SharingUnknown, contractError(err)
	}
	return observeSharing(path)
}
