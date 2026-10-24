//go:build linux && (amd64 || arm64)

package filestore

import (
	"context"
	"errors"
	"os"
	"syscall"

	"github.com/deliri/primitive/v2026/contextstate"
)

func filesystemDurabilityPlatform() error { return nil }

func synchronizeFilesystem(ctx context.Context, file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var syncErr error
	err = connection.Control(func(descriptor uintptr) {
		for {
			if contextErr := contextstate.Validate(ctx); contextErr != nil {
				syncErr = errors.Join(syncErr, contextErr)
				return
			}
			_, _, cause := syscall.Syscall(filesystemSyncTrap, descriptor, 0, 0)
			if cause == 0 {
				syncErr = nil
				return
			}
			syncErr = os.NewSyscallError("syncfs", cause)
			if !errors.Is(cause, syscall.EINTR) {
				return
			}
		}
	})
	return errors.Join(err, syncErr)
}
