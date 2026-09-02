package gomodule

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/mod/module"
)

// ImportPathMaximumBytes bounds one Go package import identity.
const ImportPathMaximumBytes = 1024

// ImportPath is one validated Go package import identity. Standard-library
// paths are admitted even though they are not module paths.
type ImportPath struct {
	value string
}

// ParseImportPath admits one canonical Go import identity.
func ParseImportPath(value string) (ImportPath, error) {
	path := ImportPath{value: value}
	if err := path.Validate(); err != nil {
		return ImportPath{}, err
	}
	return path, nil
}

// Validate rejects absent, oversized, or invalid Go import paths.
func (p ImportPath) Validate() error {
	if p.value == "" || len(p.value) > ImportPathMaximumBytes {
		return contractError("import path is absent or exceeds its byte bound")
	}
	if err := module.CheckImportPath(p.value); err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	return nil
}

// String returns the canonical import path, or empty text when invalid.
func (p ImportPath) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// MarshalJSON emits the admitted import identity as one canonical JSON string.
func (p ImportPath) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONString(p.value)
	if err != nil {
		return nil, errors.Join(core.ErrGoModuleContract, err)
	}
	return encoded, nil
}

// UnmarshalJSON admits one import identity without mutating the receiver when
// the external representation is rejected.
func (p *ImportPath) UnmarshalJSON(data []byte) error {
	if p == nil {
		return contractError("import path receiver is nil")
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	candidate, err := ParseImportPath(value)
	if err != nil {
		return err
	}
	*p = candidate
	return nil
}

var _ core.ValidatedJSONMarshaler = ImportPath{}
