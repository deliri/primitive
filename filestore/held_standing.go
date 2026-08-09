package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// HeldStanding is the closed set of answers to one custody question: does a
// path still name the exact filesystem entry a held handle is open to?
//
// Closed rather than a boolean because the caller's next act differs per
// answer: a lock holder removes the name it still owns, refuses to touch a
// name that now belongs to someone else, and has nothing to do when the name
// is gone. A caller told only "not yours" would have to look again to tell
// the last two apart, and that second look is the race this door exists to
// narrow.
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
		HeldStandingAbsent:   "absent",
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

// ObserveHeldStanding reports whether one absolute path still names the exact
// filesystem entry a held handle is open to.
//
// It exists for the moment custody must be released or trusted: a lock holder
// about to remove its lock file must not remove a name another process has
// already claimed, and a recovery path that reopened a record must know the
// name it opened is the entry it probed. Both questions are one identity
// comparison the standard library answers through os.SameFile, so every
// product that asks reaches past this package with a raw stat pair on a bare
// string path; this door is that comparison made once, against the kind of
// handle this package itself hands out.
//
// Identity is the filesystem's own: device and inode where hosts have them,
// volume and file index on Windows, read from values the standard library
// already returned. A hard link to the held entry is therefore the held
// entry, because the answer is about identity, not spelling. The final
// component is not followed: a symbolic link planted at the name is reported
// as a replacement, never followed to the entry it points at, for the same
// reason Inspect refuses to describe a target the caller cannot use.
//
// Absence is an observation, not a failure. A missing entry, a missing
// parent, and a parent that is not a directory all report HeldStandingAbsent,
// because each means nothing occupies the path. A permission refusal stays an
// error, so an observation is never recorded that the caller was not allowed
// to make.
//
// The answer is one moment. Nothing stops the name changing the instant it is
// returned; the door narrows the window a raw stat pair leaves open, and the
// residue belongs to the advisory lock that guards the record, exactly as it
// does for ObserveSharing's probe.
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
