package gomodule

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/mod/module"
)

// PathMaximumBytes is the compiler-owned upper bound for one module identity.
const PathMaximumBytes = 1024

// Path is one validated canonical Go module identity.
type Path struct {
	value string
}

// ParsePath admits one canonical Go module identity.
func ParsePath(value string) (Path, error) {
	path := Path{value: value}
	if err := path.Validate(); err != nil {
		return Path{}, err
	}
	return path, nil
}

// Validate rejects absent, oversized, or noncanonical Go module paths.
func (p Path) Validate() error {
	if p.value == "" {
		return contractError("module path is absent")
	}
	if len(p.value) > PathMaximumBytes {
		return contractError("module path exceeds its byte bound")
	}
	if err := module.CheckPath(p.value); err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	return nil
}

// String returns the canonical path, or an empty string for an invalid value.
func (p Path) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// MarshalJSON emits the admitted module identity as one canonical JSON string.
func (p Path) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	encoded, err := core.MarshalCanonicalJSONString(p.value)
	if err != nil {
		return nil, errors.Join(core.ErrGoModuleContract, err)
	}
	return encoded, nil
}

// UnmarshalJSON admits one module identity without mutating the receiver when
// the external representation is rejected.
func (p *Path) UnmarshalJSON(data []byte) error {
	if p == nil {
		return contractError("module path receiver is nil")
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	candidate, err := ParsePath(value)
	if err != nil {
		return err
	}
	*p = candidate
	return nil
}

func contractError(message string) error {
	return errors.Join(core.ErrGoModuleContract, errors.New(message))
}

var _ core.ValidatedJSONMarshaler = Path{}
