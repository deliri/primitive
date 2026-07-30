package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	// FilesystemPathMaximumRunes bounds a complete lexical path.
	FilesystemPathMaximumRunes = 4096
	// FilesystemPathMaximumComponents bounds non-root lexical components.
	FilesystemPathMaximumComponents = 256
	// FilesystemPathComponentMaximumBytes is the portable component byte cap.
	FilesystemPathComponentMaximumBytes = 255

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
	if len(value) > FilesystemPathComponentMaximumBytes {
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
	return marshalJSONString(c.String())
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
	return marshalJSONString(p.String())
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
	if value == "" {
		return filesystemPathError(filesystemPathEmptyDiagnostic)
	}
	if !utf8.ValidString(value) {
		return filesystemPathError(filesystemPathInvalidUTF8Diagnostic)
	}
	if utf8.RuneCountInString(value) > FilesystemPathMaximumRunes {
		return filesystemPathError(filesystemPathRuneLimitDiagnostic)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return filesystemPathError(filesystemPathNULDiagnostic)
	}
	if !filepath.IsLocal(value) {
		return filesystemPathError("filesystem path is not local")
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

func validateAbsoluteFilesystemPath(value string) error {
	if value == "" {
		return filesystemPathError(filesystemPathEmptyDiagnostic)
	}
	if !utf8.ValidString(value) {
		return filesystemPathError(filesystemPathInvalidUTF8Diagnostic)
	}
	if utf8.RuneCountInString(value) > FilesystemPathMaximumRunes {
		return filesystemPathError(filesystemPathRuneLimitDiagnostic)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return filesystemPathError(filesystemPathNULDiagnostic)
	}
	if !filepath.IsAbs(value) {
		return filesystemPathError("filesystem path is not absolute")
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

func filesystemPathComponentCount(value string) int {
	volume := filepath.VolumeName(value)
	remainder := strings.TrimPrefix(value, volume)
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
	componentBytes := 0
	for index := range len(value) {
		if os.IsPathSeparator(value[index]) {
			componentBytes = 0
			continue
		}
		componentBytes++
		if componentBytes > FilesystemPathComponentMaximumBytes {
			return true
		}
	}
	return false
}

func filesystemPathError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
