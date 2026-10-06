package hostfacts

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// SetAmbientEnvironment installs one validated variable in the current
// process. Callers own process-wide coordination and the binding's lifetime.
// Empty values remain present; no trimming or ambient inheritance is applied.
func SetAmbientEnvironment(ctx context.Context, variable process.EnvironmentVariable) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := variable.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	name, err := variable.Name.Value()
	if err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	value, err := variable.Value.Value()
	if err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if err := os.Setenv(name, value); err != nil {
		return errors.Join(core.ErrHostFacts, err)
	}
	return nil
}

// RemoveAmbientEnvironment removes exactly one validated process binding.
// Absence follows Go's native unset semantics. This does not clear the process
// environment or restore an invented previous world.
func RemoveAmbientEnvironment(ctx context.Context, name process.EnvironmentName) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	text, err := name.Value()
	if err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	if err := os.Unsetenv(text); err != nil {
		return errors.Join(core.ErrHostFacts, err)
	}
	return nil
}
