//go:build !windows

package process

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// observeProcesses refuses: this host offers no kernel process snapshot, and
// the supported spelling of the same question is composing the platform's
// own process lister through Run.
func observeProcesses(_ context.Context, _ ProcessVisit) error {
	return errors.Join(
		core.ErrProcessUnsupported,
		errors.New("this host offers no kernel process snapshot"),
	)
}
