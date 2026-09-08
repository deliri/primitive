package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// HeldStanding reports the native identity relation between a held Go handle
// and a path's final entry.
type HeldStanding uint8

const (
	// HeldStandingUnknown is outside the admitted domain.
	HeldStandingUnknown HeldStanding = iota
	// HeldStandingSame reports the path names the exact entry the handle
	// holds.
	HeldStandingSame
	// HeldStandingReplaced reports the path names a different entry now.
	HeldStandingReplaced
	// HeldStandingAbsent reports nothing occupies the path now.
	HeldStandingAbsent
	heldStandingLimit
)

func heldStandingDiagnostics() [heldStandingLimit]string {
	return [...]string{
		HeldStandingSame:     "same entry",
		HeldStandingReplaced: "replaced",
		HeldStandingAbsent:   "path entry absent",
	}
}

// Validate rejects values outside the closed standing domain.
func (s HeldStanding) Validate() error {
	if !s.IsValid() {
		return contractError(errors.New("held standing is outside the admitted domain"))
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s HeldStanding) IsValid() bool {
	diagnostics := heldStandingDiagnostics()
	return s > HeldStandingUnknown && s < heldStandingLimit && diagnostics[s] != ""
}

// OffWireEnum declares that HeldStanding is not a wire encoding.
func (HeldStanding) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s HeldStanding) String() string {
	diagnostics := heldStandingDiagnostics()
	if s < heldStandingLimit && diagnostics[s] != "" {
		return diagnostics[s]
	}
	return core.UnknownEnumDiagnostic
}

// ObserveHeldStanding compares a held handle with os.Lstat(path), using Go's
// os.SameFile identity. Hard links share identity. The final symlink is observed
// as itself; ancestor links follow the native filesystem lookup rules.
//
// Missing entries and non-directory ancestors report Absent. Native observation
// failures retain their cause; an already-closed handle is a contract refusal.
// This is one observation, not a reservation against subsequent name changes.
func ObserveHeldStanding(ctx context.Context, held *os.File, path core.AbsolutePath) (HeldStanding, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return HeldStandingUnknown, err
	}
	if held == nil {
		return HeldStandingUnknown, contractError(errors.New("held handle is nil"))
	}
	if err := path.Validate(); err != nil {
		return HeldStandingUnknown, contractError(err)
	}
	heldInfo, err := heldEntryInfo(held)
	if err != nil {
		return HeldStandingUnknown, err
	}
	return standingAt(heldInfo, path)
}

// heldEntryInfo describes the entry behind the held handle. A handle the
// caller already closed is a contract violation rather than a filesystem
// answer, and the native cause stays reachable either way.
func heldEntryInfo(held *os.File) (fs.FileInfo, error) {
	info, err := held.Stat()
	if errors.Is(err, os.ErrClosed) {
		return nil, contractError(err)
	}
	if err != nil {
		return nil, sourceError(err)
	}
	// Stat's contract makes info non-nil exactly when err is nil; this guard
	// makes that contract compiler-visible and fails closed if a filesystem
	// ever breaks it.
	if info == nil {
		return nil, sourceError(fs.ErrInvalid)
	}
	return info, nil
}

// standingAt compares the held entry's identity against whatever occupies the
// path right now. Lstat, not Stat: the final component is reported as itself,
// so a planted link can never borrow the held entry's identity.
func standingAt(heldInfo fs.FileInfo, path core.AbsolutePath) (HeldStanding, error) {
	pathInfo, err := os.Lstat(path.String())
	if errors.Is(err, fs.ErrNotExist) || errnoSaysNotADirectory(err) {
		return HeldStandingAbsent, nil
	}
	if err != nil {
		return HeldStandingUnknown, sourceError(err)
	}
	if pathInfo == nil {
		return HeldStandingUnknown, sourceError(fs.ErrInvalid)
	}
	if os.SameFile(heldInfo, pathInfo) {
		return HeldStandingSame, nil
	}
	return HeldStandingReplaced, nil
}
