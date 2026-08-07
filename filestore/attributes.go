package filestore

import (
	"errors"
	"io/fs"

	"github.com/deliri/primitive/v2026/core"
)

// Permissions is the POSIX permission bits of one observed entry, separated
// from the file mode that carries them.
//
// fs.FileMode is two facts in one integer: what kind of entry this is, and who
// may do what to it. A product recording permissions writes mode.Perm() to keep
// the second without the first, and a product that forgets writes a number
// whose high bits say "directory" into a field meaning "0644". Answering with a
// distinct type removes the chance to forget, and makes a permissions value
// impossible to pass where a whole mode is wanted.
type Permissions struct {
	value fs.FileMode
	set   bool
}

// Validate rejects the unset zero value and any value carrying mode bits
// outside the permission field.
func (p Permissions) Validate() error {
	if !p.set {
		return contractError(errors.New("filestore permissions are unset"))
	}
	if p.value != p.value.Perm() {
		return contractError(errors.New("filestore permissions carry non-permission mode bits"))
	}
	return nil
}

// IsSet reports whether p came from a real observation.
func (p Permissions) IsSet() bool { return p.set }

// FileMode returns the permission bits as the standard-library type, for a
// caller handing them to an operation that takes one.
func (p Permissions) FileMode() (fs.FileMode, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	return p.value, nil
}

// Bits returns the permission bits as the unsigned integer a durable record
// stores, so a caller persisting them never converts a mode by hand.
func (p Permissions) Bits() (uint32, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	return uint32(p.value), nil
}

// String renders the permission bits in the standard-library notation.
func (p Permissions) String() string {
	if !p.set {
		return "unset"
	}
	return p.value.String()
}

// Ownership is the numeric owner of one observed entry.
//
// A product projecting a file into a durable record has to state who owned it,
// and the standard library answers only through fs.FileInfo.Sys, an `any`
// holding a platform structure. Every product that wants the fact therefore
// performs its own platform type assertion, which is both a portability bug
// waiting on the first non-POSIX host and an untyped value crossing a package
// boundary. This is the fact, typed, with the assertion made once.
//
// Identifiers are numeric and stay numeric. Resolving one to a name is a
// lookup against a user database that may not be present, may disagree between
// the observing and the reading host, and would make an observation depend on
// configuration; the number is what the filesystem actually recorded.
type Ownership struct {
	uid uint32
	gid uint32
	set bool
}

// Validate rejects the unset zero value. Zero identifiers are not rejected:
// uid 0 is root, which is a real and common owner, so the set flag rather than
// the value is what separates "unobserved" from "owned by root".
func (o Ownership) Validate() error {
	if !o.set {
		return contractError(errors.New("filestore ownership is unset"))
	}
	return nil
}

// IsSet reports whether o came from a real observation.
func (o Ownership) IsSet() bool { return o.set }

// UID returns the numeric user identifier that owns the observed entry.
func (o Ownership) UID() (uint32, error) {
	if err := o.Validate(); err != nil {
		return 0, err
	}
	return o.uid, nil
}

// GID returns the numeric group identifier that owns the observed entry.
func (o Ownership) GID() (uint32, error) {
	if err := o.Validate(); err != nil {
		return 0, err
	}
	return o.gid, nil
}

// Allocation is the storage the filesystem reports as backing one observed
// regular file.
//
// A file's byte count and its allocated storage are different facts: a sparse
// file claims a size the device never granted, and a product that just
// preallocated a reserve must be able to ask whether the blocks are really
// there. The standard library answers only through fs.FileInfo.Sys, so every
// product wanting the fact performs its own platform type assertion; this is
// the fact, typed, with the assertion made once.
//
// Not every filesystem measures allocation. An unreported Allocation is a
// real observation, distinct from zero blocks: "this host does not say" lets
// a caller treat its reservation as satisfied vacuously, while zero reported
// bytes says the file is a hole.
type Allocation struct {
	bytes    core.ByteLength
	reported bool
}

// Validate rejects an allocation claiming bytes it never observed.
func (a Allocation) Validate() error {
	if !a.reported && a.bytes.Uint64() != 0 {
		return contractError(errors.New("filestore allocation claims bytes without an observation"))
	}
	return nil
}

// Reported reports whether the filesystem stated an allocation at all.
func (a Allocation) Reported() bool { return a.reported }

// Bytes returns the allocated storage behind the observed file.
//
// Only a reported allocation has a byte count. An unreported one is refused
// rather than answered with zero, which a caller would read as a hole.
func (a Allocation) Bytes() (core.ByteLength, error) {
	if err := a.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	if !a.reported {
		return core.ByteLength{}, contractError(errors.New("filestore allocation was not reported by this filesystem"))
	}
	return a.bytes, nil
}

// Permissions returns who may do what to the observed entry.
//
// Only an entry that exists has permissions. An absent or unreachable path has
// nothing to report and is refused rather than answered with zero bits, which
// a caller would persist as a real mode meaning "nobody may do anything".
func (i Inspection) Permissions() (Permissions, error) {
	kind, err := i.Kind()
	if err != nil {
		return Permissions{}, err
	}
	if kind == PathKindAbsent || kind == PathKindUnreachable {
		return Permissions{}, contractError(errors.New("an absent path has no permissions"))
	}
	if err := i.permissions.Validate(); err != nil {
		return Permissions{}, err
	}
	return i.permissions, nil
}

// Ownership returns the numeric owner of the observed entry.
//
// Only an entry that exists has an owner, and only a host whose filesystem
// records numeric identifiers reports one. A platform that does not is a
// refusal rather than an answer of uid 0, which would name root as the owner
// of every file on a host that has no such concept.
func (i Inspection) Ownership() (Ownership, error) {
	kind, err := i.Kind()
	if err != nil {
		return Ownership{}, err
	}
	if kind == PathKindAbsent || kind == PathKindUnreachable {
		return Ownership{}, contractError(errors.New("an absent path has no owner"))
	}
	if err := i.ownership.Validate(); err != nil {
		return Ownership{}, err
	}
	return i.ownership, nil
}

// observedPermissions keeps the permission bits of one existing entry. The
// mask is what separates this from the kind: everything above the permission
// field describes the kind, which PathKind already carries.
func observedPermissions(info fs.FileInfo) Permissions {
	return Permissions{value: info.Mode().Perm(), set: true}
}

// Allocation returns the storage backing the observed regular file.
//
// Only a regular file has an allocation worth asking about; every other kind
// is refused the way SizeBytes refuses it. Unlike Ownership, an unreported
// answer is returned rather than refused: a host that does not measure
// allocation has still answered the caller's real question, because a
// reservation check against "this filesystem does not say" is vacuously
// satisfied, while fabricating zero would name every file a hole.
func (i Inspection) Allocation() (Allocation, error) {
	kind, err := i.Kind()
	if err != nil {
		return Allocation{}, err
	}
	if kind != PathKindRegularFile {
		return Allocation{}, contractError(errors.New("only a regular file has an allocation"))
	}
	if err := i.allocation.Validate(); err != nil {
		return Allocation{}, err
	}
	return i.allocation, nil
}
