package process

import (
	"context"
	"errors"
	"os/exec"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Resolve finds the executable one bare command name refers to on the ambient
// PATH and returns it as the absolute path Request.Command requires.
//
// Request.Command is a core.AbsolutePath on purpose: what a bare name runs
// depends on an environment variable the operator controls, so the decision
// must be made once, visibly, before execution rather than by exec on every
// call. That left every product holding a name with nowhere to turn it into a
// path, so all four consumers wrote their own lookup, and each one decided for
// itself what to do about a relative answer, an unrunnable file, and a name
// the host does not have.
//
// A name found through a relative PATH entry is refused, not corrected. Go
// returns exec.ErrDot for that case rather than the path, because running
// whatever happens to sit in the current directory is how a repository being
// inspected takes over the process inspecting it. Consumers wrote a
// filepath.Abs call after LookPath believing they were repairing a relative
// answer; LookPath never hands one back without that error, so the call was
// unreachable and the belief was wrong. Primitive keeps Go's refusal instead
// of re-enabling the behavior Go removed.
//
// The parameter is a core.PathComponent because a name is a name: that type
// already refuses separators, "." and "..", and embedded NUL bytes at its own
// parse boundary, so a caller holding a path cannot arrive here asking a PATH
// question. exec.ErrNotFound stays reachable through errors.Is, so a caller
// probing whether a tool is installed can tell that apart from a name the host
// resolved to something unusable.
func Resolve(ctx context.Context, name core.PathComponent) (core.AbsolutePath, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.AbsolutePath{}, err
	}
	if err := name.Validate(); err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessContract, err)
	}
	found, err := exec.LookPath(name.String())
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessContract, err)
	}
	// The parse is the gate, not a formality: it is the compiler-visible proof
	// that nothing but an absolute path leaves this function, and it is what a
	// future LookPath change would have to break loudly rather than silently.
	path, err := core.ParseAbsolutePath(found)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrProcessContract, err)
	}
	return path, nil
}
