package release

import (
	"errors"
	"strconv"
	"strings"
)

const (
	goCompilerVersionPrefix = "go"
	// Three uint32 decimal components, two separators and the Go prefix.
	goCompilerVersionMaximumBytes      = 3*10 + 2 + len(goCompilerVersionPrefix)
	goCompilerVersionInvalidDiagnostic = "go compiler version is not canonical stable provenance"
)

// GoCompilerVersion records the exact stable compiler version in published
// evidence. It does not authorize execution. GoToolchainIdentity owns the
// separately reviewed compiler selection for new builds. Its zero is invalid.
type GoCompilerVersion struct {
	major uint32
	minor uint32
	patch uint32
}

func parseGoCompilerVersion(value string) (GoCompilerVersion, error) {
	if len(value) > goCompilerVersionMaximumBytes || !strings.HasPrefix(value, goCompilerVersionPrefix) {
		return GoCompilerVersion{}, manifestError(errors.New(goCompilerVersionInvalidDiagnostic))
	}
	majorText, rest, first := strings.Cut(value[len(goCompilerVersionPrefix):], ".")
	minorText, patchText, second := strings.Cut(rest, ".")
	if !first || !second {
		return GoCompilerVersion{}, manifestError(errors.New(goCompilerVersionInvalidDiagnostic))
	}
	major, majorErr := strconv.ParseUint(majorText, 10, 32)
	minor, minorErr := strconv.ParseUint(minorText, 10, 32)
	patch, patchErr := strconv.ParseUint(patchText, 10, 32)
	if err := errors.Join(majorErr, minorErr, patchErr); err != nil {
		return GoCompilerVersion{}, manifestError(errors.New(goCompilerVersionInvalidDiagnostic), err)
	}
	version := GoCompilerVersion{major: uint32(major), minor: uint32(minor), patch: uint32(patch)}
	canonical, err := version.Version()
	if err != nil || canonical != value {
		return GoCompilerVersion{}, manifestError(errors.New(goCompilerVersionInvalidDiagnostic), err)
	}
	return version, nil
}

func (v GoCompilerVersion) Validate() error {
	if v.major == 0 {
		return manifestError(errors.New(goCompilerVersionInvalidDiagnostic))
	}
	return nil
}

// Version returns the exact canonical stable compiler token.
func (v GoCompilerVersion) Version() (string, error) {
	if err := v.Validate(); err != nil {
		return "", err
	}
	return goCompilerVersionPrefix + strconv.FormatUint(uint64(v.major), 10) + "." +
		strconv.FormatUint(uint64(v.minor), 10) + "." + strconv.FormatUint(uint64(v.patch), 10), nil
}
