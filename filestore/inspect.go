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

// PathKind is the closed vocabulary of native path observations. It records
// mechanical entry kinds without deciding whether a caller can use them.
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

// Validate checks that the observed kind and retained facts agree. Absence
// carries no metadata; an existing entry carries a timestamp and permissions.
// Only a regular file carries a byte extent and allocation observation.
func (i Inspection) Validate() error {
	if err := i.kind.Validate(); err != nil {
		return err
	}
	if i.kind == PathKindAbsent || i.kind == PathKindUnreachable {
		if i != (Inspection{kind: i.kind}) {
			return contractError(errors.New("absent path carries entry metadata"))
		}
		return nil
	}
	return validateInspectionFacts(i)
}

func validateInspectionFacts(i Inspection) error {
	if err := i.modified.Validate(); err != nil {
		return contractError(err)
	}
	if err := i.permissions.Validate(); err != nil {
		return err
	}
	if !i.ownership.set && i.ownership != (Ownership{}) {
		return contractError(errors.New("unreported ownership carries identifiers"))
	}
	if i.kind != PathKindRegularFile {
		if i.size != (core.ByteLength{}) || i.allocation != (Allocation{}) {
			return contractError(errors.New("non-regular entry carries regular-file storage facts"))
		}
		return nil
	}
	if err := i.size.Validate(); err != nil {
		return contractError(err)
	}
	return i.allocation.Validate()
}

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

// ModifiedAt returns the timestamp captured with this existing entry. It
// refuses absent and unreachable paths rather than fabricating an epoch.
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

// Inspect observes one absolute path through Go Lstat. It
// reports the final symbolic link itself, and classifies a missing or
// non-directory parent as unreachable. Permission refusals preserve the native
// cause rather than claiming absence. It creates or modifies no entries.
//
// A missing leaf requires a separate parent Stat to distinguish reachable
// absence from an unreachable path. Callers coordinate namespace changes when
// they require a stable view across those observations.
func Inspect(ctx context.Context, path core.AbsolutePath) (Inspection, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Inspection{}, err
	}
	if err := path.Validate(); err != nil {
		return Inspection{}, contractError(err)
	}
	info, err := os.Lstat(path.String())
	if errors.Is(err, fs.ErrNotExist) || errnoSaysNotADirectory(err) {
		return inspectMissingPath(path)
	}
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	return inspectionForEntry(info)
}

func inspectMissingPath(path core.AbsolutePath) (Inspection, error) {
	parent, err := path.Parent()
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
	return newInspection(PathKindAbsent)
}

// parentHoldsEntries classifies the parent with Go Stat. Intermediate links
// follow Go path semantics. Missing and non-directory parents cannot hold the
// requested entry; other native failures remain errors. Neither observation
// opens the named entry, so a FIFO cannot turn inspection into a blocking read.
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

func inspectionForEntry(info fs.FileInfo) (Inspection, error) {
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
	if err := inspection.Validate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
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
	if err := inspection.Validate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
}

var (
	_ core.Validatable = PathKindUnknown
	_ core.Validatable = Inspection{}
	_ core.Validatable = Allocation{}
)
