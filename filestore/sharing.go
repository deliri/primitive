package filestore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Sharing reports whether a native zero-share read open encounters contention.
type Sharing uint8

const (
	// SharingUnknown is outside the admitted domain.
	SharingUnknown Sharing = iota
	// SharingAvailable reports the path opened without a sharing conflict.
	SharingAvailable
	// SharingHeld reports a native sharing or lock violation during the probe.
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

// ObserveSharing probes a Windows path with a zero-share read open and closes
// the acquired handle before returning. Conflicting handles may belong to any
// process, including this one. Other native failures retain their cause.
// Hosts without Windows share modes refuse with ErrFilestoreContract.
// The result is one observation; it reserves no subsequent access.
func ObserveSharing(ctx context.Context, path core.AbsolutePath) (Sharing, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return SharingUnknown, err
	}
	if err := path.Validate(); err != nil {
		return SharingUnknown, contractError(err)
	}
	return observeSharing(path)
}
