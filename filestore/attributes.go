package filestore

import (
	"errors"
	"io/fs"
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
// mask is what separates this from the mode: everything above the permission
// field describes the kind, which PathKind already carries.
func observedPermissions(info fs.FileInfo) Permissions {
	return Permissions{value: info.Mode().Perm(), set: true}
}
