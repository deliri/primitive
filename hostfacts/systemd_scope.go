package hostfacts

import (
	"context"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const systemdScopeSuffix = ".scope"
const systemdLoaded = "loaded"
const systemdNotFound = "not-found"

// SystemdScopeRequest names one native transient scope. UserManager selects
// the calling account's manager rather than the system manager.
type SystemdScopeRequest struct {
	Unit        core.PathComponent
	UserManager bool
}

func (r SystemdScopeRequest) Validate() error {
	if err := r.Unit.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if !strings.HasSuffix(r.Unit.String(), systemdScopeSuffix) || r.Unit.String() == systemdScopeSuffix {
		return core.ErrHostFactsContract
	}
	return nil
}

// ObserveSystemdScope reports whether the manager still owns a transient scope.
// Alive is deliberately conservative: even an empty loaded scope remains alive
// until systemd collects it. Process-group absence alone proves nothing about
// descendants that changed groups inside the scope. No process is signalled.
func ObserveSystemdScope(ctx context.Context, r SystemdScopeRequest) (process.Liveness, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return process.LivenessUnknown, fail(OperationSystemdScope, core.ErrHostFactsContract, err)
	}
	platform, err := CurrentPlatform()
	if err != nil {
		return process.LivenessUnknown, fail(OperationSystemdScope, core.ErrHostFactsObservation, err)
	}
	if platform.OperatingSystem != core.OperatingSystemLinux {
		return process.LivenessUnknown, fail(OperationSystemdScope, core.ErrHostFactsUnsupported, nil)
	}
	args := []string{"show", "--property=LoadState", "--value", "--", r.Unit.String()}
	if r.UserManager {
		args = append([]string{"--user"}, args...)
	}
	data, err := resourceCommand(ctx, "systemctl", args...)
	if err != nil {
		return process.LivenessUnknown, fail(OperationSystemdScope, core.ErrHostFactsObservation, err)
	}
	state, err := parseSystemdScopePresence(data)
	if err = errors.Join(err, contextstate.Validate(ctx)); err != nil {
		return process.LivenessUnknown, fail(OperationSystemdScope, core.ErrHostFactsObservation, err)
	}
	return state, nil
}

func parseSystemdScopePresence(data []byte) (process.Liveness, error) {
	switch strings.TrimSpace(string(data)) {
	case systemdLoaded:
		return process.LivenessAlive, nil
	case systemdNotFound:
		return process.LivenessGone, nil
	default:
		return process.LivenessUnknown, core.ErrHostFactsObservation
	}
}
