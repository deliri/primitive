//go:build unix

package process

import (
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// A write through Go's fd 1/2 handles exits on EPIPE before defers run.
// A scoped duplicate keeps the same native stream and returns the error.
// ForkLock closes the dup/CLOEXEC window against Go's concurrent child starts.
func pipeErrorDestination(destination io.Writer) (io.Writer, func() error, error) {
	file, ok := destination.(*os.File)
	if !ok {
		return destination, nil, nil
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, nil, err
	}
	duplicate := -1
	var duplicateErr error
	err = raw.Control(func(descriptor uintptr) {
		if descriptor != 1 && descriptor != 2 {
			return
		}
		syscall.ForkLock.RLock()
		duplicate, duplicateErr = syscall.Dup(int(descriptor))
		if duplicateErr == nil {
			syscall.CloseOnExec(duplicate)
		}
		syscall.ForkLock.RUnlock()
	})
	if err := errors.Join(err, duplicateErr); err != nil {
		return nil, nil, err
	}
	if duplicate < 0 {
		return destination, nil, nil
	}
	owned := os.NewFile(uintptr(duplicate), file.Name())
	if owned == nil {
		return nil, nil, errors.Join(core.ErrProcessContract, syscall.Close(duplicate))
	}
	return owned, owned.Close, nil
}
