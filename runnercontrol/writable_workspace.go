package runnercontrol

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// WritableWorkspace is the signed, compiler-visible writable namespace for
// one experiment. The verified checkout is intentionally not part of this
// namespace: it is a separately owned read-only source coordinate.
type WritableWorkspace struct {
	Root      core.AbsolutePath `json:"root"`
	Home      core.AbsolutePath `json:"home"`
	Output    core.AbsolutePath `json:"output"`
	Cache     core.AbsolutePath `json:"cache"`
	Temporary core.AbsolutePath `json:"temporary"`
}

func (w WritableWorkspace) ValidateEnvironment(environment process.Environment) error {
	if err := errors.Join(w.Validate(), environment.Validate()); err != nil {
		return err
	}
	if environment.Mode != process.EnvironmentModeExact {
		return core.ErrPrimitiveContract
	}
	values, err := environment.Strings()
	if err != nil {
		return err
	}
	required := [...]struct {
		name string
		path core.AbsolutePath
	}{
		{core.EnvironmentHomeName, w.Home},
		{core.EnvironmentTemporaryName, w.Temporary},
		{core.EnvironmentCacheName, w.Cache},
	}
	for _, requirement := range required {
		want := requirement.name + "=" + requirement.path.String()
		found := false
		for _, value := range values {
			if value == want {
				found = true
			}
			if strings.HasPrefix(value, requirement.name+"=") && value != want {
				return core.ErrPrimitiveContract
			}
		}
		if !found {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (w WritableWorkspace) Validate() error {
	if err := errors.Join(w.Root.Validate(), w.Home.Validate(), w.Output.Validate(), w.Cache.Validate(), w.Temporary.Validate()); err != nil {
		return err
	}
	coordinates := [...]core.AbsolutePath{w.Home, w.Output, w.Cache, w.Temporary}
	for left := range coordinates {
		if _, err := coordinates[left].RelativeTo(w.Root); err != nil || coordinates[left] == w.Root {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		for right := left + 1; right < len(coordinates); right++ {
			if coordinates[left] == coordinates[right] {
				return core.ErrPrimitiveContract
			}
		}
	}
	return nil
}

var _ core.Validatable = WritableWorkspace{}
