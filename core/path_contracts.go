package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	// filesystemPathMaximumRunes bounds a complete lexical path.
	filesystemPathMaximumRunes = 4096
	// FilesystemPathMaximumComponents bounds non-root lexical components.
	FilesystemPathMaximumComponents = 256
	// filesystemPathComponentMaximumBytes is the portable component byte cap.
	filesystemPathComponentMaximumBytes = 255

	filesystemPathEmptyDiagnostic              = "filesystem path is empty"
	filesystemPathInvalidUTF8Diagnostic        = "filesystem path is not valid UTF-8"
	filesystemPathRuneLimitDiagnostic          = "filesystem path exceeds the rune limit"
	filesystemPathNULDiagnostic                = "filesystem path contains NUL"
	filesystemPathNoncanonicalDiagnostic       = "filesystem path is not lexically clean"
	filesystemPathComponentLimitDiagnostic     = "filesystem path exceeds the component limit"
	filesystemPathOversizedComponentDiagnostic = "filesystem path contains an oversized component"
)

// PathComponent is one canonical, nonempty native filesystem component.
type PathComponent struct {
	value string
}

// ParsePathComponent validates one component without touching the filesystem.
func ParsePathComponent(value string) (PathComponent, error) {
	if value == "" {
		return PathComponent{}, filesystemPathError("path component is empty")
	}
	if !utf8.ValidString(value) {
		return PathComponent{}, filesystemPathError("path component is not valid UTF-8")
	}
	if len(value) > filesystemPathComponentMaximumBytes {
		return PathComponent{}, filesystemPathError("path component exceeds the byte limit")
	}
	if value == "." || value == ".." || containsPathSeparator(value) ||
		filepath.Base(value) != value || strings.IndexByte(value, 0) >= 0 {
		return PathComponent{}, filesystemPathError("path component is not a canonical single component")
	}
	return PathComponent{value: value}, nil
}

func containsPathSeparator(value string) bool {
	for index := range len(value) {
		if os.IsPathSeparator(value[index]) {
			return true
		}
	}
	return false
}

// String returns the validated component text.
func (c PathComponent) String() string {
	return c.value
}

// Validate rejects the unset zero value.
func (c PathComponent) Validate() error {
	if c.value == "" {
		return filesystemPathError("path component is unset")
	}
	return nil
}

// MarshalJSON emits the canonical component as a JSON string.
func (c PathComponent) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(c.String())
}

// UnmarshalJSON accepts one canonical native path component.
func (c *PathComponent) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(ErrJSONContract, filesystemPathError("nil path component receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParsePathComponent(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*c = decoded
	return nil
}

// AbsolutePath is one lexically clean native absolute path. It makes no false
// claim about whether the path names a file, directory, or existing entry.
type AbsolutePath struct {
	value string
}

// RelativePath is one lexically clean native path confined by an os.Root.
// The value "." names the root itself.
type RelativePath struct {
	value string
}

// ParseRelativePath validates native root-relative path form without
// filesystem I/O.
func ParseRelativePath(value string) (RelativePath, error) {
	if err := validateRelativeFilesystemPath(value); err != nil {
		return RelativePath{}, err
	}
	return RelativePath{value: value}, nil
}

// String returns the validated native path.
func (p RelativePath) String() string {
	return p.value
}

// Validate rejects the unset zero value.
func (p RelativePath) Validate() error {
	if p.value == "" {
		return filesystemPathError("relative path is unset")
	}
	return nil
}

// ParseAbsolutePath validates lexical absolute-path form without filesystem I/O.
func ParseAbsolutePath(value string) (AbsolutePath, error) {
	if err := validateAbsoluteFilesystemPath(value); err != nil {
		return AbsolutePath{}, err
	}
	return AbsolutePath{value: value}, nil
}

// String returns the validated native path.
func (p AbsolutePath) String() string {
	return p.value
}

// Validate rejects the unset zero value.
func (p AbsolutePath) Validate() error {
	if p.value == "" {
		return filesystemPathError("absolute path is unset")
	}
	return nil
}

// Parent returns the lexical parent; the filesystem root is its own parent.
func (p AbsolutePath) Parent() (AbsolutePath, error) {
	if err := p.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	return ParseAbsolutePath(filepath.Dir(p.value))
}

// Base returns the final component and rejects the componentless root.
func (p AbsolutePath) Base() (PathComponent, error) {
	if err := p.Validate(); err != nil {
		return PathComponent{}, err
	}
	if filepath.Base(p.value) == string(filepath.Separator) {
		return PathComponent{}, filesystemPathError("filesystem root has no path component")
	}
	return ParsePathComponent(filepath.Base(p.value))
}

// Resolve joins names below an absolute path, admitting each as its own
// component.
//
// This is the shape every product actually needs: a root plus a handful of
// compiler-owned name constants. Written with filepath.Join it produces one
// string that is validated only at the end, so a name that was never a legal
// component can still yield a legal-looking path. Here each name is refused
// where it is introduced, and the caller gets one error instead of one per
// segment.
func (p AbsolutePath) Resolve(names ...string) (AbsolutePath, error) {
	if err := p.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	resolved := p
	for _, name := range names {
		component, err := ParsePathComponent(name)
		if err != nil {
			return AbsolutePath{}, err
		}
		resolved, err = resolved.Join(component)
		if err != nil {
			return AbsolutePath{}, err
		}
	}
	return resolved, nil
}

// Resolve joins names below a relative path, admitting each as its own
// component. It is Resolve's counterpart for paths already confined to a root.
func (p RelativePath) Resolve(names ...string) (RelativePath, error) {
	if err := p.Validate(); err != nil {
		return RelativePath{}, err
	}
	resolved := p
	for _, name := range names {
		component, err := ParsePathComponent(name)
		if err != nil {
			return RelativePath{}, err
		}
		resolved, err = resolved.Join(component)
		if err != nil {
			return RelativePath{}, err
		}
	}
	return resolved, nil
}

// Join appends one validated component to a relative path.
//
// Building a nested path out of filepath.Join and re-parsing the result is the
// shape this replaces. That round trip validates only the finished string, so
// a component that was never a legal component on its own could still produce
// a legal-looking path; here each part is admitted by the type that owns it
// before it is joined.
func (p RelativePath) Join(component PathComponent) (RelativePath, error) {
	if err := p.Validate(); err != nil {
		return RelativePath{}, err
	}
	if err := component.Validate(); err != nil {
		return RelativePath{}, err
	}
	return ParseRelativePath(filepath.Join(p.value, component.value))
}

// JoinRelative resolves a relative path against an absolute one.
//
// An absolute root plus a nested relative path is how every rooted filesystem
// request is expressed, so products otherwise reach for filepath.Join on two
// strings and re-parse. Both sides are already validated by their own types,
// and the result is validated again as an absolute path, so no unchecked text
// exists at any point.
func (p AbsolutePath) JoinRelative(relative RelativePath) (AbsolutePath, error) {
	if err := p.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	if err := relative.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	return ParseAbsolutePath(filepath.Join(p.value, relative.value))
}

// Join appends one validated component without introducing path-kind claims.
func (p AbsolutePath) Join(component PathComponent) (AbsolutePath, error) {
	if err := p.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	if err := component.Validate(); err != nil {
		return AbsolutePath{}, err
	}
	return ParseAbsolutePath(filepath.Join(p.value, component.value))
}

// MarshalJSON emits the canonical native absolute path as a JSON string.
func (p AbsolutePath) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(p.String())
}

// UnmarshalJSON accepts one canonical native absolute path.
func (p *AbsolutePath) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(ErrJSONContract, filesystemPathError("nil absolute path receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParseAbsolutePath(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*p = decoded
	return nil
}

func validateRelativeFilesystemPath(value string) error {
	return validateFilesystemPath(value, filesystemPathRelative)
}

func validateAbsoluteFilesystemPath(value string) error {
	return validateFilesystemPath(value, filesystemPathAbsolute)
}

type filesystemPathKind uint8

const (
	filesystemPathKindUnknown filesystemPathKind = iota
	filesystemPathRelative
	filesystemPathAbsolute
)

func validateFilesystemPath(value string, kind filesystemPathKind) error {
	if err := validateFilesystemPathText(value); err != nil {
		return err
	}
	if err := validateFilesystemPathKind(value, kind); err != nil {
		return err
	}
	if filepath.Clean(value) != value {
		return filesystemPathError(filesystemPathNoncanonicalDiagnostic)
	}
	if filesystemPathComponentCount(value) > FilesystemPathMaximumComponents {
		return filesystemPathError(filesystemPathComponentLimitDiagnostic)
	}
	if filesystemPathHasOversizedComponent(value) {
		return filesystemPathError(filesystemPathOversizedComponentDiagnostic)
	}
	return nil
}

func validateFilesystemPathText(value string) error {
	if value == "" {
		return filesystemPathError(filesystemPathEmptyDiagnostic)
	}
	if !utf8.ValidString(value) {
		return filesystemPathError(filesystemPathInvalidUTF8Diagnostic)
	}
	if utf8.RuneCountInString(value) > filesystemPathMaximumRunes {
		return filesystemPathError(filesystemPathRuneLimitDiagnostic)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return filesystemPathError(filesystemPathNULDiagnostic)
	}
	return nil
}

func validateFilesystemPathKind(value string, kind filesystemPathKind) error {
	switch kind {
	case filesystemPathRelative:
		if !filepath.IsLocal(value) {
			return filesystemPathError("filesystem path is not local")
		}
	case filesystemPathAbsolute:
		if filepath.IsAbs(value) {
			return nil
		}
		return filesystemPathError("filesystem path is not absolute")
	default:
		return filesystemPathError("filesystem path kind is not admitted")
	}
	return nil
}

func filesystemPathComponentCount(value string) int {
	remainder := filesystemPathWithoutVolume(value)
	remainder = strings.Trim(remainder, string(filepath.Separator))
	if remainder == "" {
		return 0
	}
	count := 1
	for index := range len(remainder) {
		if os.IsPathSeparator(remainder[index]) {
			count++
		}
	}
	return count
}

func filesystemPathHasOversizedComponent(value string) bool {
	value = filesystemPathWithoutVolume(value)
	componentBytes := 0
	for index := range len(value) {
		if os.IsPathSeparator(value[index]) {
			componentBytes = 0
			continue
		}
		componentBytes++
		if componentBytes > filesystemPathComponentMaximumBytes {
			return true
		}
	}
	return false
}

func filesystemPathWithoutVolume(value string) string {
	return strings.TrimPrefix(value, filepath.VolumeName(value))
}

func filesystemPathError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
