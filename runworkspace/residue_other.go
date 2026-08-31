//go:build !linux

package runworkspace

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

type LinuxResidueConfiguration struct {
	ProcRoot             core.AbsolutePath
	RunParent            core.AbsolutePath
	ControlGroupRoot     core.AbsolutePath
	NetworkNamespaceRoot core.AbsolutePath
	CredentialRoot       core.AbsolutePath
	SecretRoot           core.AbsolutePath
	UnitPrefix           core.PathComponent
	ProcessUserID        uint32
}

func (c LinuxResidueConfiguration) Validate() error {
	return errors.Join(core.ErrPrimitiveContract, errors.New("Linux residue observation is unavailable on this platform"))
}

type LinuxResidueSource struct{ configuration LinuxResidueConfiguration }

func NewLinuxResidueSource(configuration LinuxResidueConfiguration) (LinuxResidueSource, error) {
	return LinuxResidueSource{}, configuration.Validate()
}

func (s LinuxResidueSource) Validate() error { return s.configuration.Validate() }

func (s LinuxResidueSource) ObserveResidue(context.Context) (Residue, error) {
	return Residue{}, s.Validate()
}

var (
	_ core.Validatable = LinuxResidueConfiguration{}
	_ core.Validatable = LinuxResidueSource{}
	_ ResidueSource    = LinuxResidueSource{}
)
