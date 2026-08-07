//go:build !unix && !windows

package hostfacts

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// observedTerminalGeometry refuses on hosts with no terminal interrogation
// mechanism, so a caller is never told a descriptor is detached on a platform
// where nobody looked.
func observedTerminalGeometry(_ *os.File) (TerminalGeometry, error) {
	return TerminalGeometry{}, fail(OperationTerminalGeometry, core.ErrHostFactsUnsupported, nil)
}
