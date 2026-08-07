//go:build linux

package hostfacts

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// openPseudoTerminalSlave mints a fresh pty pair and returns the slave, the
// descriptor the kernel treats as a real terminal.
//
// Linux unlocks the pair with TIOCSPTLCK and names the slave with TIOCGPTN,
// both integer ioctls x/sys wraps directly.
func openPseudoTerminalSlave(t *testing.T) *os.File {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	t.Cleanup(func() {
		if err := master.Close(); err != nil {
			t.Fatalf("close pty master: %v", err)
		}
	})
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlock pty slave: %v", err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("read pty slave number: %v", err)
	}
	slaveName := fmt.Sprintf("/dev/pts/%d", number)
	slave, err := os.OpenFile(slaveName, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open pty slave %s: %v", slaveName, err)
	}
	t.Cleanup(func() {
		if err := slave.Close(); err != nil {
			t.Fatalf("close pty slave: %v", err)
		}
	})
	return slave
}
