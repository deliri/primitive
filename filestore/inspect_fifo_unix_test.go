//go:build unix

package filestore_test

import "syscall"

// syscallMkfifo creates a named pipe so the "neither a file nor a directory"
// kind is proved against a real entry rather than asserted. It is a test-only
// use of the substrate: production never creates one. It lives in a unix leaf
// because syscall.Mkfifo does not exist on Windows, and an unconditional
// reference breaks the Windows test binary the canonical gate cross-compiles.
func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
