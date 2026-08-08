//go:build darwin

package hostfacts

import (
	"fmt"
	"os"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// openPseudoTerminalSlave mints a fresh pty pair and returns the slave, the
// descriptor the kernel treats as a real terminal.
//
// Darwin's grant and unlock ioctls take no argument, and the slave's name is
// determined by the master's device minor: the xnu multiplexer allocates
// /dev/ttysNNN where NNN is the minor number zero-padded to three digits.
// Reading the minor out of the Stat_t the standard library already returned
// avoids the name ioctl, whose byte-buffer calling convention x/sys does not
// wrap.
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
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatalf("grant pty slave: %v", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatalf("unlock pty slave: %v", err)
	}
	info, err := master.Stat()
	if err != nil {
		t.Fatalf("stat pty master: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("pty master Stat_t = %T, want *syscall.Stat_t", info.Sys())
	}
	slaveName := fmt.Sprintf("/dev/ttys%03d", stat.Rdev&0xffffff)
	slave, err := os.OpenFile(slaveName, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open pty slave %s: %v", slaveName, err)
	}
	t.Cleanup(func() {
		if err := slave.Close(); err != nil {
			t.Fatalf("close pty slave: %v", err)
		}
	})
	// The name above is derived from an undocumented xnu convention, so the
	// convention is proven per run rather than assumed: the opened device must
	// carry the same minor the master's Rdev names, or the test would be
	// perturbing some other tty-class descriptor it does not own.
	slaveInfo, err := slave.Stat()
	if err != nil {
		t.Fatalf("stat pty slave: %v", err)
	}
	slaveStat, ok := slaveInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("pty slave Stat_t = %T, want *syscall.Stat_t", slaveInfo.Sys())
	}
	if got, want := slaveStat.Rdev&0xffffff, stat.Rdev&0xffffff; got != want {
		t.Fatalf("opened slave minor = %d, want the master's slave minor %d: the name convention drifted", got, want)
	}
	return slave
}
