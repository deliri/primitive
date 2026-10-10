package gomodule

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// DeclaredPath is the identity declared by a main module. Local names are
// valid; this identity does not grant downloadable-module authority.
// Its zero value is invalid. ObserveDeclaration owns its production.
type DeclaredPath struct {
	value string
}

func parseDeclaredPath(value string) (DeclaredPath, error) {
	path := DeclaredPath{value: value}
	if err := path.Validate(); err != nil {
		return DeclaredPath{}, err
	}
	return path, nil
}

func (p DeclaredPath) Validate() error {
	if err := admitPath(p.value, pathAdmissionImport); err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	return nil
}

// String returns the exact admitted declaration, or empty for the zero value.
func (p DeclaredPath) String() string { return p.value }
