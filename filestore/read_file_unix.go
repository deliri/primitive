//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"os"

	"golang.org/x/sys/unix"
)

// openReadFile acquires the source descriptor without ever entering a blocking
// open. A named pipe or device left on the path answers immediately instead of
// parking the caller until a peer appears, so the identity check that follows
// runs against a handle this process already holds rather than against a path
// another process can still swap.
func openReadFile(root *os.Root, path string) (*os.File, error) {
	return openNonblockingFile(rootedOpenRequest{root: root, path: path, flag: os.O_RDONLY})
}

func openDirectory(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK|unix.O_DIRECTORY, 0)
}

func openMutableFile(request rootedOpenRequest) (*os.File, error) {
	return openNonblockingFile(request)
}

func openNonblockingFile(request rootedOpenRequest) (*os.File, error) {
	return request.root.OpenFile(request.path, request.flag|unix.O_NONBLOCK, request.mode)
}

// prepareRegularReadFile restores blocking semantics once the acquired handle
// has been proven regular, so the streaming copy sees the ordinary read
// contract rather than the nonblocking mode the open needed.
func prepareRegularReadFile(file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var blockingErr error
	if err := connection.Control(func(descriptor uintptr) {
		blockingErr = unix.SetNonblock(int(descriptor), false)
	}); err != nil {
		return err
	}
	return blockingErr
}
