package garble

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	toolModulePath           = "mvdan.cc/garble"
	toolVersion              = "v0.17.0"
	toolRevision             = "39c484d3007e9a608ac8692dab0b9bb5f71dfc2a"
	toolModuleSum            = "h1:XJ6jJhlT8HTEU9Dd02nLDUciuyPDXGRopwy/Cuoo/0M="
	toolMinimumGoVersion     = "go1.26.0"
	toolUnsupportedGoVersion = "go1.27"
)

// ToolProvenance is the exact public identity recorded for one reviewed
// Garble executable. It contains no seed or custody material.
type ToolProvenance struct {
	ModulePath string
	Version    string
	Revision   string
	ModuleSum  string
}

func toolProvenances() [toolIdentityLimit]ToolProvenance {
	return [...]ToolProvenance{
		ToolIdentityPrimitive2026: {
			ModulePath: toolModulePath, Version: toolVersion,
			Revision: toolRevision, ModuleSum: toolModuleSum,
		},
	}
}

func toolIdentityLabels() [toolIdentityLimit]string {
	return [...]string{
		ToolIdentityPrimitive2026: "primitive-2026",
	}
}

func toolMinimumGoVersions() [toolIdentityLimit]string {
	return [...]string{ToolIdentityPrimitive2026: toolMinimumGoVersion}
}

func toolUnsupportedGoVersions() [toolIdentityLimit]string {
	return [...]string{ToolIdentityPrimitive2026: toolUnsupportedGoVersion}
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
	provenance, err := t.Provenance()
	return provenance.ModulePath, err
}

// Version returns the exact reviewed Go module version.
func (t ToolIdentity) Version() (string, error) {
	provenance, err := t.Provenance()
	return provenance.Version, err
}

// Revision returns the exact reviewed source revision.
func (t ToolIdentity) Revision() (string, error) {
	provenance, err := t.Provenance()
	return provenance.Revision, err
}

// ModuleSum returns the exact reviewed Go module checksum.
func (t ToolIdentity) ModuleSum() (string, error) {
	provenance, err := t.Provenance()
	return provenance.ModuleSum, err
}

// MinimumGoVersion returns the upstream minimum supported Go version.
func (t ToolIdentity) MinimumGoVersion() (string, error) {
	return t.fact(toolMinimumGoVersions())
}

// UnsupportedGoVersion returns the first upstream unsupported Go version.
func (t ToolIdentity) UnsupportedGoVersion() (string, error) {
	return t.fact(toolUnsupportedGoVersions())
}

// Provenance returns every public identity fact for the reviewed tool.
func (t ToolIdentity) Provenance() (ToolProvenance, error) {
	if err := t.Validate(); err != nil {
		return ToolProvenance{}, err
	}
	return toolProvenances()[t], nil
}

// Validate resolves the provenance against the complete reviewed tool domain.
func (p ToolProvenance) Validate() error {
	_, err := ResolveTool(p)
	return err
}

// ResolveTool returns the reviewed identity named by exact public provenance.
func ResolveTool(provenance ToolProvenance) (ToolIdentity, error) {
	for tool := ToolIdentityUnknown + 1; tool < toolIdentityLimit; tool++ {
		if tool.IsValid() && toolProvenances()[tool] == provenance {
			return tool, nil
		}
	}
	return ToolIdentityUnknown, contractError(
		errors.New("garble tool provenance is outside the admitted domain"))
}

func (t ToolIdentity) fact(values [toolIdentityLimit]string) (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return values[t], nil
}
