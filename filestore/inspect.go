package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
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
		PathKindAbsent:       "absent",
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

// Inspection is one observed path. Its field is unexported and reachable only
// through an accessor that revalidates, so a caller cannot assemble an
// observation it never made.
type Inspection struct {
	kind PathKind
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
	root, err := os.OpenRoot(parent.String())
	if err != nil {
		return inspectionForParentFailure(ctx, parent, err)
	}
	info, statErr := root.Lstat(base.String())
	closeErr := root.Close()
	if closeErr != nil {
		return Inspection{}, sourceError(closeErr)
	}
	return inspectionForEntry(info, statErr)
}

// inspectionForParentFailure separates "there is nothing here" from "I was not
// allowed to look".
//
// A missing parent and a parent that is not a directory are the same fact to a
// caller: nothing is there and nothing can be put there. Only the first is
// reported as a typed error. The standard library detects the second itself
// and returns an untyped error whose sole distinguishing feature is its text,
// so the parent is asked what it is rather than having its error matched on
// prose. Recursion moves one component toward the filesystem root each time
// and terminates there, within the bound AbsolutePath already places on depth.
func inspectionForParentFailure(ctx context.Context, parent core.AbsolutePath, err error) (Inspection, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return newInspection(PathKindUnreachable)
	}
	parentInspection, parentErr := Inspect(ctx, parent)
	if parentErr != nil {
		return Inspection{}, sourceError(err)
	}
	kind, kindErr := parentInspection.Kind()
	if kindErr != nil {
		return Inspection{}, sourceError(err)
	}
	if kind != PathKindDirectory {
		return newInspection(PathKindUnreachable)
	}
	return Inspection{}, sourceError(err)
}

func inspectionForEntry(info fs.FileInfo, err error) (Inspection, error) {
	if errors.Is(err, fs.ErrNotExist) {
		return newInspection(PathKindAbsent)
	}
	if err != nil {
		return Inspection{}, sourceError(err)
	}
	return newInspection(kindForMode(info.Mode()))
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
)
