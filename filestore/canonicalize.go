package filestore

import (
	"context"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Canonicalize reports the kernel's canonical spelling of one existing path:
// every symbolic link in every component resolved, the terminal answer the
// operating system itself gives for where the name actually leads.
//
// Products need this at exactly one kind of moment: an integrity decision
// over two paths that must name the same real location, such as proving a
// pointer read from disk stays inside an admin root, or pinning a build
// directory that a link farm may respell. Written directly it is
// filepath.EvalSymlinks on a bare string and a re-parse, a real-world touch
// taken outside every rooted boundary this package owns; here the answer
// comes back as an admitted absolute path or not at all.
//
// The resolution follows links deliberately, which is the opposite of
// Inspect's refusal to follow the final component: Inspect answers what
// occupies a name, Canonicalize answers where a name leads. A path that does
// not exist has no canonical spelling and is refused with the native cause
// preserved, absence included, because canonicalizing an absent name would
// answer a question the kernel was never asked.
func Canonicalize(ctx context.Context, path core.AbsolutePath) (core.AbsolutePath, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.AbsolutePath{}, err
	}
	if err := path.Validate(); err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	resolved, err := filepath.EvalSymlinks(path.String())
	if err != nil {
		return core.AbsolutePath{}, sourceError(err)
	}
	canonical, err := core.ParseAbsolutePath(resolved)
	if err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	return canonical, nil
}
