package garble

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	toolModulePath           = "mvdan.cc/garble"
	toolVersion              = "v0.16.1-0.20260621195108-ffa2daf72f03"
	toolRevision             = "ffa2daf72f036d7ff72f6a3c8243997f06fa7b4e"
	toolModuleSum            = "h1:3/JEpDf12w/71XWzIrnLazgTQD6UWElzrRQWo4oJ7s0="
	toolMinimumGoVersion     = "go1.26.0"
	toolUnsupportedGoVersion = "go1.27"
)

func toolIdentityLabels() [toolIdentityLimit]string {
	return [...]string{
		ToolIdentityPrimitive2026: "primitive-2026",
	}
}

// ToolIdentity is the closed set of reviewed Garble tool builds.
type ToolIdentity uint8

const (
	// ToolIdentityUnknown is the invalid zero tool.
	ToolIdentityUnknown ToolIdentity = iota
	// ToolIdentityPrimitive2026 is the exact reviewed Primitive 2026 build.
	ToolIdentityPrimitive2026
	toolIdentityLimit
)

// CurrentTool returns the one reviewed Primitive 2026 Garble identity.
func CurrentTool() ToolIdentity {
	return ToolIdentityPrimitive2026
}

// Validate rejects tools outside the reviewed closed domain.
func (t ToolIdentity) Validate() error {
	if !t.IsValid() {
		return contractError(errors.New("garble tool identity is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the reviewed tool-identity domain.
func (t ToolIdentity) IsValid() bool {
	return t > ToolIdentityUnknown && t < toolIdentityLimit && toolIdentityLabels()[t] != ""
}

// OffWireEnum declares ToolIdentity as reviewed execution policy rather than a
// wire encoding.
func (ToolIdentity) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for t.
func (t ToolIdentity) String() string {
	if !t.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return toolIdentityLabels()[t]
}

// ModulePath returns the exact reviewed Go module path.
func (t ToolIdentity) ModulePath() (string, error) {
	return t.fact(toolModulePath)
}

// Version returns the exact reviewed Go module version.
func (t ToolIdentity) Version() (string, error) {
	return t.fact(toolVersion)
}

// Revision returns the exact reviewed source revision.
func (t ToolIdentity) Revision() (string, error) {
	return t.fact(toolRevision)
}

// ModuleSum returns the exact reviewed Go module checksum.
func (t ToolIdentity) ModuleSum() (string, error) {
	return t.fact(toolModuleSum)
}

// MinimumGoVersion returns the upstream minimum supported Go version.
func (t ToolIdentity) MinimumGoVersion() (string, error) {
	return t.fact(toolMinimumGoVersion)
}

// UnsupportedGoVersion returns the first upstream unsupported Go version.
func (t ToolIdentity) UnsupportedGoVersion() (string, error) {
	return t.fact(toolUnsupportedGoVersion)
}

func (t ToolIdentity) fact(value string) (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return value, nil
}
