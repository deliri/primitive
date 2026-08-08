package process

import (
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// Executable reports the absolute path of the binary running this process,
// the fact a daemon needs when it writes a supervisor unit that must start
// this exact binary again. It is an observation, not a claim of durability:
// the platform answers with the path used to start the process, links
// unresolved, and a caller comparing locations canonicalizes through the
// filestore door that owns that question.
func Executable() (core.AbsolutePath, error) {
	value, err := os.Executable()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessObservation, err)
	}
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessContract, err)
	}
	return path, nil
}

// AmbientEnvironment reports the calling process's own environment through
// the same effective-environment admission a child request uses: later
// duplicates win, malformed entries are refused, and the answer is the typed
// Environment every other door already speaks. A product that must filter
// its inheritance before handing it to a child projects this back to
// strings, removes what it owns, and re-admits the remainder; without this
// door that product reads os.Environ directly, which is exactly the ambient
// real-world ask this package exists to own.
func AmbientEnvironment() (Environment, error) {
	return ParseEffectiveEnvironment(os.Environ())
}
