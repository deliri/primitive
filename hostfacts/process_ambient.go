package hostfacts

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// Executable reports the absolute path of the binary running this process.
func Executable() (core.AbsolutePath, error) {
	value, err := os.Executable()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return path, nil
}

// WorkingDirectory reports the calling process's current working directory.
func WorkingDirectory() (core.AbsolutePath, error) {
	value, err := os.Getwd()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return path, nil
}

// ResolveWorkingPath observes the current working directory once and resolves
// path text against that exact typed coordinate. Absolute text remains
// absolute; relative text is anchored to the observed directory.
func ResolveWorkingPath(ctx context.Context, text string) (core.AbsolutePath, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.AbsolutePath{}, err
	}
	working, err := WorkingDirectory()
	if err != nil {
		return core.AbsolutePath{}, err
	}
	path, err := working.ResolveText(text)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return path, nil
}

// AmbientEnvironment observes the calling process's effective environment and
// admits it through Process's one typed child-environment agreement.
func AmbientEnvironment() (process.Environment, error) {
	environment, err := process.ParseEffectiveEnvironment(os.Environ())
	if err != nil {
		return process.Environment{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return environment, nil
}

// LookupAmbientEnvironment observes one exact variable without materializing
// the process's complete environment.
func LookupAmbientEnvironment(name process.EnvironmentName) (process.EnvironmentLookup, error) {
	if err := name.Validate(); err != nil {
		return process.EnvironmentLookup{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	nameText, err := name.Value()
	if err != nil {
		return process.EnvironmentLookup{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	value, present := os.LookupEnv(nameText)
	lookup := process.EnvironmentLookup{Presence: process.EnvironmentPresenceAbsent}
	if present {
		typedValue, valueErr := process.NewEnvironmentValue(value)
		if valueErr != nil {
			return process.EnvironmentLookup{}, errors.Join(core.ErrHostFactsObservation, valueErr)
		}
		lookup = process.EnvironmentLookup{Value: typedValue, Presence: process.EnvironmentPresencePresent}
	}
	if err := lookup.Validate(); err != nil {
		return process.EnvironmentLookup{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return lookup, nil
}
