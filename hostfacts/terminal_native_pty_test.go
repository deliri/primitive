//go:build darwin || linux

package hostfacts

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// The pseudo-terminal slave is the one descriptor a test can mint that the
// kernel treats as a real terminal, so it is the only honest proof of the
// attached observation. The master is deliberately not observed: on Darwin a
// master answers no winsize ioctl until its slave exists, and a renderer
// holds the slave side anyway.
func TestObserveTerminalGeometryReadsARealPseudoTerminal(t *testing.T) {
	t.Parallel()

	t.Run("a pseudo terminal answers with the exact width it was given", func(t *testing.T) {
		t.Parallel()
		slave := openPseudoTerminalSlave(t)
		setTerminalColumns(t, slave, 121)
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(pty slave) error = %v, want nil", err)
		}
		if attachment, err := geometry.Attachment(); err != nil || attachment != TerminalAttachmentTerminal {
			t.Fatalf("pty Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentTerminal)
		}
		if columns, err := geometry.Columns(); err != nil || columns != 121 {
			t.Fatalf("pty Columns() = (%v, %v), want (121, nil)", columns, err)
		}
	})

	t.Run("a one column terminal is still attached", func(t *testing.T) {
		t.Parallel()
		slave := openPseudoTerminalSlave(t)
		setTerminalColumns(t, slave, 1)
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(one column pty) error = %v, want nil", err)
		}
		if columns, err := geometry.Columns(); err != nil || columns != 1 {
			t.Fatalf("one column pty Columns() = (%v, %v), want (1, nil)", columns, err)
		}
	})

	t.Run("a pseudo terminal reporting zero width is detached", func(t *testing.T) {
		t.Parallel()
		slave := openPseudoTerminalSlave(t)
		setTerminalColumns(t, slave, 0)
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(zero-width pty) error = %v, want nil", err)
		}
		requireDetached(t, geometry)
	})
}

// setTerminalColumns fixes the slave's window size so the observation under
// test reads back a width this test chose, not whatever the host had.
func setTerminalColumns(t *testing.T, slave *os.File, columns uint16) {
	t.Helper()
	window := unix.Winsize{Col: columns, Row: 40}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &window); err != nil {
		t.Fatalf("set pty winsize: %v", err)
	}
}
