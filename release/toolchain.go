package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const goToolchainVersionPrimitive2026 = "go1.26.6"

// GoToolchainIdentity is the closed set of compilers admitted for release
// construction. Exact compiler identity is retained outside stripped binaries.
type GoToolchainIdentity uint8

const (
	// GoToolchainUnknown is the invalid zero compiler identity.
	GoToolchainUnknown GoToolchainIdentity = iota
	// GoToolchainPrimitive2026 identifies the exact reviewed Go 1.26.6 toolchain.
	GoToolchainPrimitive2026
	goToolchainLimit
)

func goToolchainLabels() [goToolchainLimit]string {
	return [...]string{GoToolchainPrimitive2026: "go-1.26.6"}
}

func goToolchainVersions() [goToolchainLimit]string {
	return [...]string{GoToolchainPrimitive2026: goToolchainVersionPrimitive2026}
}

// CurrentGoToolchain returns the compiler identity pinned for Primitive 2026
// release construction.
func CurrentGoToolchain() GoToolchainIdentity { return GoToolchainPrimitive2026 }

// Validate rejects compiler identities outside the reviewed closed domain.
func (i GoToolchainIdentity) Validate() error {
	if !i.IsValid() {
		return contractError(errors.New("go toolchain identity is outside the admitted domain"))
	}
	return nil
}

// IsValid reports membership in the reviewed compiler domain.
func (i GoToolchainIdentity) IsValid() bool {
	return i > GoToolchainUnknown && i < goToolchainLimit && goToolchainLabels()[i] != ""
}

// OffWireEnum declares GoToolchainIdentity as reviewed execution policy.
func (GoToolchainIdentity) OffWireEnum() {}

// String returns the compiler-owned diagnostic label.
func (i GoToolchainIdentity) String() string {
	if !i.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return goToolchainLabels()[i]
}

// Version returns the exact output token required from go version.
func (i GoToolchainIdentity) Version() (string, error) {
	if err := i.Validate(); err != nil {
		return "", err
	}
	return goToolchainVersions()[i], nil
}

func parseGoToolchainVersion(value string) (GoToolchainIdentity, error) {
	for identity := GoToolchainUnknown + 1; identity < goToolchainLimit; identity++ {
		version, err := identity.Version()
		if err == nil && version == value {
			return identity, nil
		}
	}
	return GoToolchainUnknown, manifestError(
		errors.New("go toolchain version is outside the admitted domain"))
}

var _ core.OffWireEnum = GoToolchainUnknown
