package core

import (
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// SourcePathMaximumBytes is the exact encoded byte ceiling for one
	// repository-relative source identity. Providers use this compiler-owned
	// value while incrementally decoding path streams.
	SourcePathMaximumBytes         = 1024
	repositoryIdentityMaximumBytes = 512
)

// SourcePath is one slash-canonical path relative to a repository root. The
// value "." identifies the root itself on every operating system.
type SourcePath struct{ value string }

// RepositoryIdentity is one bounded opaque repository coordinate. Primitive
// validates its transport shape without interpreting provider ownership.
type RepositoryIdentity struct{ value string }

// ParseSourcePath admits one operating-system-independent repository path.
func ParseSourcePath(value string) (SourcePath, error) {
	candidate := SourcePath{value: value}
	if err := candidate.Validate(); err != nil {
		return SourcePath{}, err
	}
	return candidate, nil
}

// NewRepositoryIdentity admits one opaque repository coordinate.
func NewRepositoryIdentity(value string) (RepositoryIdentity, error) {
	candidate := RepositoryIdentity{value: value}
	if err := candidate.Validate(); err != nil {
		return RepositoryIdentity{}, err
	}
	return candidate, nil
}

// Validate refuses noncanonical, absolute, parent-traversing, or native-only
// repository paths.
func (p SourcePath) Validate() error {
	value := p.value
	if !validSourcePathText(value) {
		return sourceIdentityError("source path text is invalid")
	}
	if !slashCanonicalSourcePath(value) {
		return sourceIdentityError("source path is not slash-canonical")
	}
	if sourcePathEscapesRepository(value) {
		return sourceIdentityError("source path escapes its repository")
	}
	return nil
}

func validSourcePathText(value string) bool {
	return len(value) != 0 && len(value) <= SourcePathMaximumBytes && utf8.ValidString(value) && strings.TrimSpace(value) == value
}

func slashCanonicalSourcePath(value string) bool {
	return value == path.Clean(value) && !strings.HasPrefix(value, "/") && !strings.HasSuffix(value, "/") && !strings.ContainsAny(value, "\\\x00\r\n")
}

func sourcePathEscapesRepository(value string) bool {
	return value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, "/../")
}

// Validate refuses empty, padded, whitespace-bearing, or control-bearing
// repository identities.
func (r RepositoryIdentity) Validate() error {
	value := r.value
	if len(value) == 0 || len(value) > repositoryIdentityMaximumBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return sourceIdentityError("repository identity text is invalid")
	}
	for _, current := range value {
		if unicode.IsControl(current) || unicode.IsSpace(current) {
			return sourceIdentityError("repository identity contains whitespace or control text")
		}
	}
	return nil
}

// String returns canonical source-path text, or empty text when invalid.
func (p SourcePath) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// String returns the opaque repository coordinate, or empty text when invalid.
func (r RepositoryIdentity) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.value
}

// MarshalJSON emits one canonical source path.
func (p SourcePath) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(p.value)
}

// UnmarshalJSON admits one canonical source path without mutating the receiver
// on rejection.
func (p *SourcePath) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(ErrJSONContract, sourceIdentityError("source path receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseSourcePath(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*p = parsed
	return nil
}

// MarshalJSON emits one canonical repository identity.
func (r RepositoryIdentity) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(r.value)
}

// UnmarshalJSON admits one repository identity without mutating the receiver
// on rejection.
func (r *RepositoryIdentity) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(ErrJSONContract, sourceIdentityError("repository identity receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := NewRepositoryIdentity(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*r = parsed
	return nil
}

// SourceSubjectKind closes the project/package/file claim and observation
// scope shared by offline source tools.
type SourceSubjectKind uint8

const (
	// SourceSubjectUnknown is the invalid zero source-subject kind.
	SourceSubjectUnknown SourceSubjectKind = iota
	// SourceSubjectProject identifies the complete source project.
	SourceSubjectProject
	// SourceSubjectPackage identifies one Go package in the source project.
	SourceSubjectPackage
	// SourceSubjectFile identifies one file in the source project.
	SourceSubjectFile
	sourceSubjectLimit
)

func sourceSubjectTokens() [sourceSubjectLimit]string {
	return [...]string{"", "project", "package", "file"}
}

// Validate refuses an unknown or future source-subject kind.
func (k SourceSubjectKind) Validate() error {
	if !k.IsValid() {
		return sourceIdentityError("source subject kind is invalid")
	}
	return nil
}

// IsValid reports whether the kind is one published source-subject value.
func (k SourceSubjectKind) IsValid() bool {
	return k > SourceSubjectUnknown && k < sourceSubjectLimit && sourceSubjectTokens()[k] != ""
}

// String returns the canonical kind token or empty text for an invalid kind.
func (k SourceSubjectKind) String() string {
	if k >= sourceSubjectLimit {
		return ""
	}
	return sourceSubjectTokens()[k]
}

// MarshalJSON emits the canonical source-subject token.
func (k SourceSubjectKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(k.String())
}

// UnmarshalJSON accepts only one published source-subject token.
func (k *SourceSubjectKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(ErrJSONContract, sourceIdentityError("source subject kind receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := SourceSubjectProject; candidate < sourceSubjectLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(ErrJSONContract, sourceIdentityError("source subject kind token is unsupported"))
}

// SourceSubject identifies one project, package, or file without assigning it
// human meaning.
type SourceSubject struct {
	// Path is the slash-canonical coordinate within the repository.
	Path SourcePath `json:"path"`
	// Kind states whether Path names the project, a package, or a file.
	Kind SourceSubjectKind `json:"kind"`
}

type sourceSubjectWire SourceSubject

// NewSourceSubject constructs one validated subject.
func NewSourceSubject(kind SourceSubjectKind, path SourcePath) (SourceSubject, error) {
	candidate := SourceSubject{Path: path, Kind: kind}
	if err := candidate.Validate(); err != nil {
		return SourceSubject{}, err
	}
	return candidate, nil
}

// Validate pins project scope to the repository root and prevents a file from
// using the root coordinate. Root packages remain valid and are distinguished
// from the project by Kind.
func (s SourceSubject) Validate() error {
	if err := errors.Join(s.Path.Validate(), s.Kind.Validate()); err != nil {
		return sourceIdentityError(err.Error())
	}
	if s.Kind == SourceSubjectProject && s.Path.String() != "." {
		return sourceIdentityError("project subject does not name the repository root")
	}
	if s.Kind == SourceSubjectFile && s.Path.String() == "." {
		return sourceIdentityError("file subject names the repository root")
	}
	return nil
}

// MarshalJSON emits one validated canonical source-subject document.
func (s SourceSubject) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONDocument(sourceSubjectWire(s))
}

// UnmarshalJSON admits one strict source subject without mutating the receiver
// on rejection.
func (s *SourceSubject) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(ErrJSONContract, sourceIdentityError("source subject receiver is nil"))
	}
	wire, err := DecodeStrictJSONStructure[sourceSubjectWire](data, DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(ErrJSONContract, ErrPrimitiveContract, err)
	}
	candidate := SourceSubject(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*s = candidate
	return nil
}

// CompareSourceSubjects orders subjects canonically without allocating. A
// caller can prove a streamed index is unique by retaining only the previous
// subject.
func CompareSourceSubjects(left, right SourceSubject) int {
	if left.Kind < right.Kind {
		return -1
	}
	if left.Kind > right.Kind {
		return 1
	}
	return strings.Compare(left.Path.String(), right.Path.String())
}

func sourceIdentityError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
