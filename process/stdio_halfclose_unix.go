//go:build unix

package process

import (
	"errors"
	"math"
	"os"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// RawControl pins the existing file while shutdown changes the socket's write
// side. It neither duplicates the descriptor nor changes shared blocking flags.
func shutdownOutput(file *os.File) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var shutdownErr error
	controlErr := raw.Control(func(descriptor uintptr) {
		if descriptor > uintptr(math.MaxInt) {
			shutdownErr = core.ErrProcessContract
			return
		}
		shutdownErr = syscall.Shutdown(int(descriptor), syscall.SHUT_WR)
	})
	// A pipe or ordinary file has no socket write side. Its owned Close below
	// is the native EOF operation; other shutdown failures are not discarded.
	if errors.Is(shutdownErr, syscall.ENOTSOCK) {
		shutdownErr = nil
	}
	return errors.Join(controlErr, os.NewSyscallError("shutdown", shutdownErr))
}
