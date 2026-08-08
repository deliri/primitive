package process

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ProcessSighting is one process the host reported in a snapshot walk: the
// identity a later liveness probe can ask about, and the executable image
// name the platform recorded for it. It is an observation, never a
// capability: nothing here can signal, wait on, or supervise what it names.
type ProcessSighting struct {
	Image    core.PathComponent
	Identity ProcessIdentity
}

// Validate rejects a sighting whose members fail their own admission.
func (s ProcessSighting) Validate() error {
	if err := s.Identity.Validate(); err != nil {
		return err
	}
	if err := s.Image.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

// ProcessVisit receives one valid sighting; returning an error stops the walk.
type ProcessVisit func(ProcessSighting) error

// ObserveProcesses walks the host's process table where the platform offers
// one as a kernel snapshot, reporting each entry to the visitor in the
// platform's own order with O(1) memory.
//
// Windows owns the only such snapshot in this stack (Toolhelp); a product
// deciding whether a stale lock's writer still exists needs it there, because
// Windows releases never shell out to Unix process listers. POSIX hosts
// refuse: the supported spelling of the same question is composing ps or
// lsof through Run, where the tool's own bounded output is the contract, and
// pretending this door works there would hide that composition behind a
// lookalike.
func ObserveProcesses(ctx context.Context, visit ProcessVisit) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if visit == nil {
		return contractError("process visitor is nil")
	}
	return observeProcesses(ctx, visit)
}
