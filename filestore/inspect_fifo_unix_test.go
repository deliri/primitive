//go:build darwin || linux

package filestore_test

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

// syscallMkfifo creates a named pipe so the "neither a file nor a directory"
// kind is proved against a real entry rather than asserted. It is a test-only
// use of the substrate: production never creates one. It lives in a unix leaf
// because syscall.Mkfifo does not exist on Windows, and an unconditional
// reference breaks the Windows test binary the canonical gate cross-compiles.
func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func nativeInspectionAbsence(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

func nativeInspectionStorage(info fs.FileInfo) (inspectionNativeStorage, error) {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok || status == nil || status.Blocks < 0 {
		return inspectionNativeStorage{}, fs.ErrInvalid
	}
	return inspectionNativeStorage{uid: status.Uid, gid: status.Gid, bytes: uint64(status.Blocks) * core.POSIXAllocationBlockBytes, reported: true}, nil
}
