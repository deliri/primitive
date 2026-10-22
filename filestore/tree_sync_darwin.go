package filestore

import (
	"errors"
	"os"
	"syscall"
)

// Darwin's os.File.Sync performs F_FULLFSYNC, a drive-wide cache flush. First
// send every member's data/metadata to the drive with fsync, then let the final
// root Sync in ConfirmTreeDurable flush the drive once. Never substitute just
// a directory sync for the member syncs.
func syncTreeMember(file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var syncErr error
	err = connection.Control(func(fd uintptr) {
		for {
			syncErr = syscall.Fsync(int(fd))
			if !errors.Is(syncErr, syscall.EINTR) {
				return
			}
		}
	})
	return errors.Join(err, syncErr)
}
