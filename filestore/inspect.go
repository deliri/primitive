package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// PathKind is the closed set of things one configured path can turn out to be.
//
// Closed rather than a boolean pair because "usable" is the caller's decision
// and it differs per path: a product may create a missing output directory but
// must never create a missing repository. A caller that only learned "not a
// directory" would have to guess which mistake the operator made.
type PathKind uint8

const (
	// PathKindUnknown is the unset kind and describes nothing.
	PathKindUnknown PathKind = iota
	// PathKindAbsent means the parent is reachable and holds no such entry.
	PathKindAbsent
	// PathKindDirectory is a real directory.
	PathKindDirectory
	// PathKindRegularFile is a real file occupying the path.
	PathKindRegularFile
	// PathKindSymbolicLink is a link, reported as itself rather than as its
	// target. A confined root refuses to traverse a link that leaves it, so a
	// caller told only about the target would be told about a path it cannot
	// actually use.
	PathKindSymbolicLink
	// PathKindOther is a device, socket, or named pipe.
	PathKindOther
	// PathKindUnreachable means a parent component is missing or is not a
	// directory, so nothing can exist at this path and nothing can be created
	// there either.
	PathKindUnreachable
	pathKindLimit
)

func pathKindTokens() [pathKindLimit]string {
	return [...]string{
		PathKindUnknown:      "",
		PathKindAbsent:       "path absent",
		PathKindDirectory:    "directory",
		PathKindRegularFile:  "regular file",
		PathKindSymbolicLink: "symbolic link",
		PathKindOther:        "neither a file nor a directory",
		PathKindUnreachable:  "unreachable: a parent is missing or is not a directory",
	}
}

// Validate rejects the unset kind and every kind outside the closed set.
func (k PathKind) Validate() error {
	if k <= PathKindUnknown || k >= pathKindLimit || pathKindTokens()[k] == "" {
		return contractError(errors.New("path kind is not a member of the closed set"))
	}
	return nil
}

// IsValid reports whether k names an observed kind.
func (k PathKind) IsValid() bool { return k.Validate() == nil }

// String returns operator-facing text, or empty text when unset.
func (k PathKind) String() string {
	if k >= pathKindLimit {
		return ""
	}
	return pathKindTokens()[k]
}

// OffWireEnum marks PathKind as an internal observation vocabulary. It names
// what a caller found on this machine and is never serialized.
func (PathKind) OffWireEnum() {}

// Inspection is one observed path. Its fields are unexported and reachable
// only through accessors that revalidate, so a caller cannot assemble an
// observation it never made.
type Inspection struct {
	modified    temporal.Instant
	size        core.ByteLength
	allocation  Allocation
	permissions Permissions
	ownership   Ownership
	kind        PathKind
}

// Validate rejects an observation that names no kind.
func (i Inspection) Validate() error { return i.kind.Validate() }

// Kind returns the observed kind.
func (i Inspection) Kind() (PathKind, error) {
	if err := i.Validate(); err != nil {
		return PathKindUnknown, err
	}
	return i.kind, nil
}

// SizeBytes returns how many bytes the observed regular file holds.
//
// Only a regular file has a size that means anything. A directory's reported
// size is an implementation detail of the filesystem, and a symbolic link's is
// the length of its target text, so answering for either would hand back a
// number that looks like a byte count and is not one.
func (i Inspection) SizeBytes() (core.ByteLength, error) {
	kind, err := i.Kind()
	if err != nil {
		return core.ByteLength{}, err
	}
	if kind != PathKindRegularFile {
		return core.ByteLength{}, contractError(errors.New("only a regular file has a byte count"))
	}
	return i.size, nil
}

// ModifiedAt returns when the observed entry last changed.
//
// The observation already holds this fact, and a caller that had to ask again
// would be asking about a different moment. Staleness decisions — reaping an
// abandoned lock, expiring cached custody — are the reason products otherwise
// keep a raw stat call after adopting Inspect.
//
// Only an entry that exists has a modification time. An absent or unreachable
// path has nothing to report and is refused rather than answered with a zero
// instant that reads as 1970.
func (i Inspection) ModifiedAt() (temporal.Instant, error) {
	kind, err := i.Kind()
	if err != nil {
		return temporal.Instant{}, err
	}
	if kind == PathKindAbsent || kind == PathKindUnreachable {
		return temporal.Instant{}, contractError(errors.New("an absent path has no modification time"))
	}
	if err := i.modified.Validate(); err != nil {
		return temporal.Instant{}, contractError(err)
	}
	return i.modified, nil
}

// Inspect reports what occupies one absolute path without creating, opening,
// or modifying anything.
//
// It exists because products otherwise reach past Filestore to the standard
// library for this one question, which is the only reason a product would hold
// a raw stat call at all. Admission decisions about configured paths are made
// before any effect, and they are made in every product, so the observation
// belongs here beside the effects it gates.
//
// The final component is not followed. A symbolic link is reported as a link,
// because a confined root refuses to traverse one that leaves the root and a
// caller told about the target would be told about a path it cannot use.
//
// A missing or non-directory parent is an observation, not a failure: nothing
// exists there and nothing can be created there, which is exactly what a
// caller needs to know. A permission refusal is a failure, because the path
// may well be usable by someone and the caller must not record an absence it
// was never allowed to observe.
func Inspect(ctx context.Context, path core.AbsolutePath) (Inspection, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Inspection{}, err
	}
	if err := path.Validate(); err != nil {
		return Inspection{}, contractError(err)
	}
	parent, err := path.Parent()
	if err != nil {
		return Inspection{}, contractError(err)
	}
	base, err := path.Base()
	if err != nil {
		return Inspection{}, contractError(err)
	}
	holdsEntries, err := parentHoldsEntries(parent)
	if err != nil {
		return Inspection{}, err
	}
	if !holdsEntries {
		return newInspection(PathKindUnreachable)
	}
	root, err := os.OpenRoot(parent.String())
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	info, statErr := root.Lstat(base.String())
	closeErr := root.Close()
	if closeErr != nil {
		return Inspection{}, sourceError(closeErr)
	}
	return inspectionForEntry(info, statErr)
}

// parentHoldsEntries reports whether the parent is a directory that could hold
// the named entry, established without opening it.
//
// Opening is what makes a hostile parent dangerous. os.OpenRoot performs an
// ordinary open, and opening a named pipe blocks until a writer arrives, so a
// FIFO anywhere in a path used to park the caller forever with the context
// never consulted. Nothing in the rooted-open API accepts a directories-only
// flag, so the kind is settled first by a call that opens nothing.
//
// A missing parent and a parent that is not a directory are the same fact to a
// caller: nothing is there and nothing can be put there. Permission trouble is
// not that fact and stays an error, so an observation is never recorded that
// the caller was not allowed to make.
//
// Stat, not Lstat: intermediate components are conventionally followed, and it
// is only the final component that Inspect leaves unfollowed. Between this
// answer and the open below the parent could in principle be replaced by a
// pipe, which is a far narrower window than the unconditional block it
// replaces, and closing it entirely needs a rooted open the standard library
// does not expose.
func parentHoldsEntries(parent core.AbsolutePath) (bool, error) {
	info, err := os.Stat(parent.String())
	if errors.Is(err, fs.ErrNotExist) || errnoSaysNotADirectory(err) {
		return false, nil
	}
	if err != nil {
		return false, sourceError(err)
	}
	return info.IsDir(), nil
}

func inspectionForEntry(info fs.FileInfo, err error) (Inspection, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return newInspection(PathKindAbsent)
	}
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	// Lstat's contract makes info non-nil exactly when err is nil; this guard
	// makes that contract compiler-visible and fails closed if a filesystem
	// ever breaks it, instead of dereferencing nil three readers later.
	if info == nil {
		return Inspection{}, sourceError(fs.ErrInvalid)
	}
	modified, err := temporal.NewInstant(info.ModTime())
	if err != nil {
		return Inspection{}, contractError(err)
	}
	size, err := observedSize(info)
	if err != nil {
		return Inspection{}, err
	}
	allocation, err := observedAllocation(info)
	if err != nil {
		return Inspection{}, err
	}
	inspection := Inspection{
		kind:        kindForMode(info.Mode()),
		modified:    modified,
		size:        size,
		allocation:  allocation,
		permissions: observedPermissions(info),
		ownership:   observedOwnership(info),
	}
	return inspection, inspection.Validate()
}

// observedSize keeps a nonsensical size out of the observation entirely. Only
// a regular file carries one, and a negative count from the filesystem is a
// refusal rather than a value to widen a typed byte length around.
func observedSize(info fs.FileInfo) (core.ByteLength, error) {
	if !info.Mode().IsRegular() {
		return core.ByteLength{}, nil
	}
	size := info.Size()
	if size < 0 {
		return core.ByteLength{}, sourceError(errors.New("filesystem reported a negative byte count"))
	}
	length, err := core.NewByteLength(uint64(size))
	if err != nil {
		return core.ByteLength{}, contractError(err)
	}
	return length, nil
}

func kindForMode(mode fs.FileMode) PathKind {
	switch {
	case mode&fs.ModeSymlink != 0:
		return PathKindSymbolicLink
	case mode.IsDir():
		return PathKindDirectory
	case mode.IsRegular():
		return PathKindRegularFile
	default:
		return PathKindOther
	}
}

func newInspection(kind PathKind) (Inspection, error) {
	inspection := Inspection{kind: kind}
	return inspection, inspection.Validate()
}

var (
	_ core.Validatable = PathKindUnknown
	_ core.Validatable = Inspection{}
	_ core.Validatable = Allocation{}
)
